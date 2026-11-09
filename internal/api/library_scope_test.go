package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/video/highlights"
)

func TestLibrarySubdirectoryAndHighlightFilters(t *testing.T) {
	root := t.TempDir()
	database, err := dbpkg.Open(filepath.Join(root, "scope.db"))
	if err != nil {
		t.Fatal(err)
	}
	deps := newTestDependencies()
	deps.database, deps.config = database, &common.Config{DatabasePath: filepath.Join(root, "scope.db"), JavCoverDir: root}
	deps.bindDatabase(t)
	t.Cleanup(func() { conn, _ := database.DB(); _ = conn.Close() })
	create := func(value any) {
		t.Helper()
		if err := database.Create(value).Error; err != nil {
			t.Fatal(err)
		}
		if video, ok := value.(*models.Video); ok && video.HighlightCount > 0 {
			path := highlights.Path(root, video.ID)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(`{"version":1,"items":[{"time":0,"label":"start"}]}`), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	directory := models.Directory{Path: root}
	create(&directory)
	subdirectory := "旅行/100%_fun's"
	var workIDs, videoIDs []int64
	for i, path := range []string{subdirectory + "/one.mp4", subdirectory + "/deeper/two.mp4", subdirectory + "-other/three.mp4", "旅行/100xxfun's/four.mp4", "elsewhere/five.mp4"} {
		studio := models.JavStudio{Name: fmt.Sprintf("Studio %d", i)}
		series := models.JavSeries{Name: fmt.Sprintf("系列 %d", i)}
		idol := models.JavIdol{Name: fmt.Sprintf("Idol %d", i)}
		create(&studio)
		create(&series)
		create(&idol)
		work := models.Jav{Code: fmt.Sprintf("SCOPE-%03d", i), StudioID: &studio.ID, SeriesID: &series.ID}
		create(&work)
		create(&models.JavIdolMap{JavID: work.ID, JavIdolID: idol.ID})
		video := models.Video{Fingerprint: fmt.Sprintf("scoped-%d", i)}
		if i == 0 {
			video.HighlightCount = 2
		}
		create(&video)
		create(&models.VideoLocation{VideoID: video.ID, DirectoryID: directory.ID, JavID: &work.ID, RelativePath: path, Filename: filepath.Base(path)})
		workIDs = append(workIDs, work.ID)
		videoIDs = append(videoIDs, video.ID)
	}
	// A work can have multiple files. Files outside the selected subtree and
	// marked files must not sneak into an unmarked result's playback options.
	for _, extra := range []struct {
		path   string
		marked bool
	}{{"elsewhere/duplicate.mp4", false}, {subdirectory + "/marked-duplicate.mp4", true}} {
		video := models.Video{Fingerprint: extra.path}
		if extra.marked {
			video.HighlightCount = 1
		}
		create(&video)
		create(&models.VideoLocation{VideoID: video.ID, DirectoryID: directory.ID, JavID: &workIDs[1], RelativePath: extra.path, Filename: filepath.Base(extra.path)})
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/videos", listVideos)
	router.GET("/jav", searchJav)
	router.GET("/jav/idols", listJavIdols)
	router.GET("/jav/studios", listJavStudios)
	router.GET("/jav/series", listJavSeries)
	request := func(endpoint, subdir, highlight, offset string) struct {
		Items []json.RawMessage `json:"items"`
		Total int               `json:"total"`
	} {
		t.Helper()
		params := url.Values{"directory_id": {fmt.Sprint(directory.ID)}, "subdirectory": {subdir}, "limit": {"1"}, "offset": {offset}, "sort": {"modified"}}
		if highlight != "" {
			params.Set("highlight_only", highlight)
		}
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest("GET", endpoint+"?"+params.Encode(), nil))
		if r.Code != 200 {
			t.Fatalf("%s: %d %s", endpoint, r.Code, r.Body.String())
		}
		var result struct {
			Items []json.RawMessage `json:"items"`
			Total int               `json:"total"`
		}
		if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	for _, endpoint := range []string{"/jav", "/jav/idols", "/jav/studios", "/jav/series"} {
		t.Run(endpoint, func(t *testing.T) {
			result := request(endpoint, subdirectory, "", "0")
			if result.Total != 2 || len(result.Items) != 1 {
				t.Fatalf("wrong scope/count: %+v", result)
			}
			second := request(endpoint, subdirectory, "", "1")
			if second.Total != 2 || len(second.Items) != 1 || string(second.Items[0]) == string(result.Items[0]) {
				t.Fatal("pagination mismatch")
			}
			if empty := request(endpoint, "missing", "", "0"); empty.Total != 0 || len(empty.Items) != 0 {
				t.Fatal("empty subtree leaked")
			}
			for _, invalid := range []string{"../escape", "/absolute"} {
				r := httptest.NewRecorder()
				router.ServeHTTP(r, httptest.NewRequest("GET", endpoint+"?directory_id=1&subdirectory="+url.QueryEscape(invalid), nil))
				if r.Code != 400 {
					t.Fatalf("invalid path accepted: %s", invalid)
				}
			}
		})
	}
	for _, endpoint := range []string{"/videos", "/jav"} {
		result := request(endpoint, subdirectory, "0", "0")
		if result.Total != 1 || len(result.Items) != 1 {
			t.Fatalf("%s unmarked total=%d", endpoint, result.Total)
		}
		if endpoint == "/jav" {
			var item models.Jav
			if err := json.Unmarshal(result.Items[0], &item); err != nil {
				t.Fatal(err)
			}
			if item.ID != workIDs[1] || len(item.Videos) != 1 || item.Videos[0].ID != videoIDs[1] {
				t.Fatalf("playback scope leaked: %+v", item.Videos)
			}
		}
		with := request(endpoint, subdirectory, "1", "0")
		if with.Total != 2 {
			t.Fatalf("%s marked total=%d", endpoint, with.Total)
		}
	}
}
