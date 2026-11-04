package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/mpv"
	"javboss/internal/video/highlights"
)

func TestHighlightAPIAndQueuePreserveTimeZeroAndSidecars(t *testing.T) {
	t.Setenv("JAVBOSS_CONTAINER", "false")
	root := t.TempDir()
	database, err := dbpkg.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	deps := newTestDependencies()
	deps.database, deps.config = database, &common.Config{DatabasePath: filepath.Join(root, "test.db")}
	deps.bindDatabase(t)
	previousPlay := playMPVPlaylistFrom
	t.Cleanup(func() { playMPVPlaylistFrom = previousPlay; sqlDB, _ := database.DB(); _ = sqlDB.Close() })
	dir := models.Directory{Path: root}
	video := models.Video{Fingerprint: "highlight-api", DurationSec: 100}
	for _, value := range []any{&dir, &video} {
		if err := database.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	location, err := dbpkg.Default().UpsertVideoLocation(context.Background(), video.ID, dir.ID, "movie.mp4", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(root, "movie.mp4")
	if err := os.WriteFile(media, []byte("original video"), 0600); err != nil {
		t.Fatal(err)
	}
	path := highlights.Path(root, video.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"items":[{"time":0,"label":"zero"},{"time":20,"label":"twenty"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/videos", listVideos)
	router.GET("/videos/:id/highlights", getVideoHighlights)
	router.PATCH("/videos/:id/highlights", editVideoHighlight)
	router.POST("/videos/playlist", playVideoPlaylist)
	request := func(method, url string, body any, status int) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, url, strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatalf("%s %s => %d %s", method, url, w.Code, w.Body.String())
		}
		return w
	}
	url := "/videos/" + strconv.FormatInt(video.ID, 10) + "/highlights"
	get := request("GET", url, nil, 200)
	var doc highlights.Document
	if err := json.Unmarshal(get.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	list := request("GET", "/videos?highlight_only=1&limit=1", nil, 200)
	var payload struct {
		Total int
		Items []models.Video
	}
	if err := json.Unmarshal(list.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != 1 || len(payload.Items) != 1 || payload.Items[0].HighlightCount != 2 {
		t.Fatalf("filtered list: %+v", payload)
	}
	var played []mpv.PlaylistItem
	playMPVPlaylistFrom = func(items []mpv.PlaylistItem, index int) error { played = items; return nil }
	zero := 0.0
	queue := videoPlaylistRequest{Player: "mpv", HighlightMode: true, LeadSeconds: 5, PreviewSeconds: 30, Items: []videoPlaylistItemRequest{{VideoID: video.ID, LocationID: location.ID, HighlightTime: &zero}}}
	request("POST", "/videos/playlist", queue, 200)
	if len(played) != 1 || !played[0].Options.HighlightMode || played[0].Options.StartTimeSec != 0 || played[0].Options.HighlightPreviewSeconds != 30 {
		t.Fatalf("zero start lost: %+v", played)
	}
	badTime := 99.0
	queue.Items[0].HighlightTime = &badTime
	request("POST", "/videos/playlist", queue, http.StatusConflict)
	queue.Items[0].HighlightTime = nil
	queue.Player = "system"
	request("POST", "/videos/playlist", queue, http.StatusBadRequest)
	request("PATCH", url, map[string]any{"time": 0, "label": "new name", "revision": doc.Revision}, 200)
	request("PATCH", url, map[string]any{"time": 0, "label": nil, "revision": doc.Revision}, http.StatusConflict)
	fresh, err := highlights.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	request("PATCH", url, map[string]any{"time": 0, "label": nil, "revision": fresh.Revision}, 200)
	if raw, err := os.ReadFile(media); err != nil || string(raw) != "original video" {
		t.Fatal("editing a highlight modified source media")
	}
}
