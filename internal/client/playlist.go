package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"javboss/internal/common/logging"
	"javboss/internal/mpv"
)

// handlePlaylist resolves remote files to local grants just like handlePlay.
// Never forward this endpoint: playback belongs to the client machine.
func (c *Client) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondJSONError(w, http.StatusMethodNotAllowed, "请求方法无效", "Method not allowed")
		return
	}
	var request struct {
		Player     string        `json:"player"`
		Items      []playRequest `json:"items"`
		StartIndex int           `json:"start_index"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxClientRequestBody))
	if err := decoder.Decode(&request); err != nil || len(request.Items) == 0 {
		respondJSONError(w, http.StatusBadRequest, "播放列表请求无效", "Invalid playlist request")
		return
	}
	if request.StartIndex < 0 || request.StartIndex >= len(request.Items) {
		respondJSONError(w, http.StatusBadRequest, "播放列表起始位置无效", "Invalid playlist start index")
		return
	}
	if request.Player != "" && request.Player != "mpv" {
		respondJSONError(w, http.StatusBadRequest, "Client 模式仅支持 MPV 外部播放列表", "Client mode only supports MPV external playlists")
		return
	}

	cookie := r.Header.Get("Cookie")
	if cookie == "" {
		respondJSONError(w, http.StatusUnauthorized, "远端登录状态不存在，请重新登录", "Remote authentication is missing; please sign in again")
		return
	}
	for _, item := range request.Items {
		if item.VideoID <= 0 || item.LocationID < 0 || item.StartTimeSec < 0 {
			respondJSONError(w, http.StatusBadRequest, "播放列表包含无效视频", "Playlist contains an invalid video")
			return
		}
	}
	dataDir := c.clientDataDir()
	remote := c.remoteState()
	items := make([]mpv.PlaylistItem, 0, len(request.Items))
	for _, item := range request.Items {
		query := make(url.Values)
		if item.LocationID > 0 {
			query.Set("location_id", strconv.FormatInt(item.LocationID, 10))
		}
		remotePath := "/videos/" + strconv.FormatInt(item.VideoID, 10) + "/stream"
		if err := c.probeRemoteMedia(r, remotePath, query.Encode(), cookie); err != nil {
			status := http.StatusBadGateway
			var statusErr *remoteStatusError
			if errors.As(err, &statusErr) && statusErr.status >= 400 && statusErr.status <= 599 {
				status = statusErr.status
			}
			respondJSONError(w, status, "远端视频不可用或登录已失效", "The remote video is unavailable or authentication has expired")
			return
		}
		token, err := c.mediaToken(remotePath, query.Encode(), cookie)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "创建本机播放会话失败", "Could not create the local playback session")
			return
		}
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = "Video #" + strconv.FormatInt(item.VideoID, 10)
		}
		videoID := item.VideoID
		items = append(items, mpv.PlaylistItem{
			Path:  c.localBaseURL + "/__client/media/" + token,
			Title: title,
			Cover: c.playlistCoverResolver(remote, item.VideoID, item.LocationID, cookie),
			OnStarted: func() {
				if c.remoteState() == remote {
					c.incrementRemotePlayCount(videoID, cookie)
				}
			},
			Options: mpv.PlayOptions{
				NewWatchReporter: c.playbackReporter(item.VideoID, item.LocationID, cookie),
				DataDir:          dataDir, VideoID: item.VideoID,
				StartTimeSec: item.StartTimeSec, EnableNetworkThumbnail: true,
			},
		})
	}
	if err := c.playPlaylist(items, request.StartIndex); err != nil {
		logging.Error("client MPV playlist failed: %v", err)
		respondJSONError(w, http.StatusInternalServerError, "使用本机 MPV 播放列表失败", "Could not play the playlist with local MPV")
		return
	}
	for _, item := range items {
		if err := c.startScreenshotSync(item.Options.VideoID, cookie, dataDir); err != nil {
			logging.Error("start client screenshot sync failed: %v", err)
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "count": len(items)})
}

func (c *Client) playlistCoverResolver(remote *remoteState, videoID, locationID int64, cookie string) mpv.PlaylistCoverResolver {
	return func(ctx context.Context) (mpv.PlaylistCoverSource, error) {
		if remote == nil || c.remoteState() != remote {
			return mpv.PlaylistCoverSource{}, errors.New("remote server changed")
		}
		target := *remote.base
		target.Path = "/videos/" + strconv.FormatInt(videoID, 10) + "/playlist-cover"
		query := url.Values{}
		if locationID > 0 {
			query.Set("location_id", strconv.FormatInt(locationID, 10))
		}
		target.RawQuery = query.Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return mpv.PlaylistCoverSource{}, err
		}
		request.Header.Set("Cookie", cookie)
		// Bind credentials to the original remote; never follow a redirect or a
		// concurrent server switch. Lua/MPV only receive the decoded local bitmap.
		response, err := c.transport.RoundTrip(request)
		if err != nil {
			return mpv.PlaylistCoverSource{}, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return mpv.PlaylistCoverSource{}, errors.New("cached remote cover unavailable")
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, (20<<20)+1))
		if err != nil || len(data) > 20<<20 || c.remoteState() != remote {
			return mpv.PlaylistCoverSource{}, errors.New("invalid remote cover response")
		}
		aspect, _ := strconv.ParseFloat(response.Header.Get("X-JavBoss-Cover-Aspect"), 64)
		position, _ := strconv.ParseFloat(response.Header.Get("X-JavBoss-Cover-Position"), 64)
		return mpv.PlaylistCoverSource{Data: data, CropAspect: aspect, CropPosition: position}, nil
	}
}
