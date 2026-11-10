package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/scrape/cover"
	"javboss/internal/mpv"
	"javboss/internal/video/screenshot"
)

func playlistCoverResolver(store *dbpkg.Store, dataDir, coverDir string, videoID, locationID int64) mpv.PlaylistCoverResolver {
	return func(parent context.Context) (mpv.PlaylistCoverSource, error) {
		ctx, cancel := context.WithTimeout(parent, 3*time.Second)
		defer cancel()
		resolvedLocation := locationID
		if resolvedLocation == 0 {
			location, err := store.GetPrimaryVideoLocation(ctx, videoID)
			if err != nil {
				return mpv.PlaylistCoverSource{}, err
			}
			if location == nil {
				return mpv.PlaylistCoverSource{}, os.ErrNotExist
			}
			resolvedLocation = location.ID
		}
		video, err := store.GetVideoForLocation(ctx, videoID, resolvedLocation)
		if err != nil {
			return mpv.PlaylistCoverSource{}, err
		}
		if video == nil {
			return mpv.PlaylistCoverSource{}, os.ErrNotExist
		}
		if path, ok := customVideoCoverPath(dataDir, video); ok {
			return mpv.PlaylistCoverSource{Path: path}, nil
		}
		if video.Jav != nil && coverDir != "" && !strings.ContainsAny(video.Jav.Code, "/\\") {
			if path, ok := cover.FindPath(coverDir, video.Jav.Code); ok {
				return mpv.PlaylistCoverSource{Path: path, CropAspect: 376.0 / 538.0, CropPosition: video.Jav.CoverCropPosition}, nil
			}
		}
		if path, ok := screenshot.CachedThumbnailPath(dataDir, video); ok {
			return mpv.PlaylistCoverSource{Path: path}, nil
		}
		return mpv.PlaylistCoverSource{}, os.ErrNotExist
	}
}

// GET /videos/:id/playlist-cover?location_id= returns cached artwork only. It
// never extracts video frames or downloads covers. Crop headers describe the
// saved JAV poster crop for authenticated remote clients, without exposing paths.
func getPlaylistCover(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "视频 ID 无效", "Invalid video ID")
		return
	}
	locationID := int64(0)
	if raw := c.Query("location_id"); raw != "" {
		locationID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || locationID <= 0 {
			respondLocalizedError(c, http.StatusBadRequest, "视频位置 ID 无效", "Invalid location ID")
			return
		}
	}
	if locationID == 0 {
		location, err := dbpkg.Default().GetPrimaryVideoLocation(c.Request.Context(), id)
		if err != nil || location == nil {
			respondLocalizedError(c, http.StatusNotFound, "视频不存在", "Video does not exist")
			return
		}
		locationID = location.ID
	}
	cfg := common.CurrentConfig()
	if cfg == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	source, err := playlistCoverResolver(dbpkg.Default(), filepath.Dir(cfg.DatabasePath), cfg.JavCoverDir, id, locationID)(c.Request.Context())
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		c.Status(status)
		return
	}
	c.Header("Cache-Control", "private, no-cache")
	c.Header("X-JavBoss-Cover-Aspect", strconv.FormatFloat(source.CropAspect, 'g', -1, 64))
	c.Header("X-JavBoss-Cover-Position", strconv.FormatFloat(source.CropPosition, 'g', -1, 64))
	c.File(source.Path)
}
