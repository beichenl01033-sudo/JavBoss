package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/ltxcatalog"
	"javboss/internal/models"
)

func TestLtxFilterReachesAllListEndpoints(t *testing.T) {
	deps := newTestDependencies()
	deps.config = &common.Config{DatabasePath: filepath.Join(t.TempDir(), "data.db")}
	testAuthService(deps, t)
	for _, row := range []any{
		&models.Directory{ID: 1, Path: t.TempDir()},
		&models.JavStudio{ID: 1, Name: "filter studio"},
		&models.JavSeries{ID: 1, Name: "filter series"},
		&models.JavIdol{ID: 1, Name: "filter idol"},
		&models.Jav{ID: 1, Code: "ABC-001", StudioID: int64TestPtr(1), SeriesID: int64TestPtr(1)},
		&models.Video{ID: 1, Fingerprint: "filter"},
		&models.VideoLocation{ID: 1, VideoID: 1, DirectoryID: 1, RelativePath: "ABC-001_LTX.mp4", Filename: "ABC-001_LTX.mp4", JavID: int64TestPtr(1)},
		&models.JavIdolMap{JavID: 1, JavIdolID: 1},
	} {
		if err := deps.database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.GET("/videos", listVideos)
	router.GET("/jav", searchJav)
	router.GET("/options", listJavFilterOptions)
	router.GET("/idols", listJavIdols)
	router.GET("/studios", listJavStudios)
	router.GET("/series", listJavSeries)
	request := func(endpoint, query string, status int, total int) {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", endpoint+"?directory_id=1&"+query, nil))
		if w.Code != status {
			t.Fatalf("%s?%s: %d %s", endpoint, query, w.Code, w.Body)
		}
		if status == 200 {
			var body struct {
				Total int `json:"total"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Total != total {
				t.Fatalf("%s?%s: total=%d want %d", endpoint, query, body.Total, total)
			}
		}
	}
	endpoints := []string{"/videos", "/jav", "/options", "/idols", "/studios", "/series"}
	for _, endpoint := range endpoints {
		request(endpoint, "ltx_local=with", 200, 1)
		request(endpoint, "ltx_local=without", 200, 0)
		request(endpoint, "ltx_catalog=not_found", 409, 0)
		request(endpoint, "ltx_local=invalid", 400, 0)
	}
	snapshot := ltxcatalog.Snapshot{Source: ltxcatalog.Source, Complete: true, Total: 1, Confirmed: 1, Items: []ltxcatalog.Item{{Code: "ABC-001"}}}
	if _, err := ltxcatalog.Save(highlightDataDir(), snapshot); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range endpoints {
		request(endpoint, "ltx_local=with&ltx_catalog=confirmed", 200, 1)
		request(endpoint, "ltx_catalog=pending", 200, 0)
		request(endpoint, "ltx_catalog=not_found", 200, 0)
	}
}

func int64TestPtr(value int64) *int64 { return &value }

func TestLtxFilterRequiresCompleteCacheAndValidStatus(t *testing.T) {
	previous := common.SetConfig(&common.Config{DatabasePath: filepath.Join(t.TempDir(), "test.db")})
	t.Cleanup(func() { common.SetConfig(previous) })
	router := gin.New()
	router.GET("/filter", func(c *gin.Context) {
		f, ok := parseLtxFilter(c)
		if ok {
			c.JSON(200, f)
		}
	})
	request := func(query string, want int) dbpkg.LtxFilter {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/filter?"+query, nil))
		if w.Code != want {
			t.Fatalf("%s: %d %s", query, w.Code, w.Body)
		}
		var f dbpkg.LtxFilter
		if want == 200 {
			if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil {
				t.Fatal(err)
			}
		}
		return f
	}
	request("", 200)
	request("ltx_local=with", 200)
	request("ltx_local=without", 200)
	request("ltx_local=bad", 400)
	request("ltx_catalog=bad", 400)
	for _, status := range []string{"confirmed", "pending", "not_found"} {
		request("ltx_catalog="+status, 409)
	}
	snapshot := ltxcatalog.Snapshot{Source: ltxcatalog.Source, Complete: true, Total: 2, Confirmed: 1, Pending: 1, Items: []ltxcatalog.Item{{Code: "abc-001"}, {Code: "DEF_002", Pending: true}}}
	if _, err := ltxcatalog.Save(highlightDataDir(), snapshot); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ status, codes string }{{"confirmed", `["ABC001"]`}, {"pending", `["DEF002"]`}, {"not_found", `["ABC001","DEF002"]`}} {
		if f := request("ltx_catalog="+tc.status+"&ltx_local=without", 200); f.CodesJSON != tc.codes || f.Local != "without" {
			t.Fatalf("%+v", f)
		}
	}
}
