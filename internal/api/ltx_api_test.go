package api

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/ltxcatalog"
	"javboss/internal/models"
	"javboss/internal/video/highlights"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLtxCatalogueTokenBoundaryAndAtomicImport(t *testing.T) {
	deps := newTestDependencies()
	deps.config = &common.Config{DatabasePath: filepath.Join(t.TempDir(), "test.db")}
	router := NewRouter("", testAuthService(deps, t))
	token, err := newExtensionCredential()
	if err != nil {
		t.Fatal(err)
	}
	if err := deps.database.Create(&models.ExtensionToken{Name: "ltx", Token: token, ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	request := func(body, credential, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/extension/ltx/catalog", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if credential != "" {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	valid := `{"source":"https://ltxlib.dpdns.org","complete":true,"total":2,"confirmed":1,"pending":1,"items":[{"code":"ABC-123","pending":false},{"code":"DEF-456","pending":true}]}`
	for _, tc := range []struct {
		body, credential, origin string
		status                   int
	}{
		{valid, "", "", 401}, {valid, token, "https://ltxlib.dpdns.org", 403}, {valid, token, javBossExtensionOrigin, 200},
		{strings.Replace(valid, `"total":2`, `"total":3`, 1), token, javBossExtensionOrigin, 400},
		{strings.Replace(valid, `"DEF-456"`, `"abc_123"`, 1), token, javBossExtensionOrigin, 400},
		{strings.Replace(valid, `"complete":true`, `"complete":false`, 1), token, javBossExtensionOrigin, 400},
	} {
		if got := request(tc.body, tc.credential, tc.origin); got.Code != tc.status {
			t.Fatalf("status %d, want %d: %s", got.Code, tc.status, got.Body)
		}
	}
	saved, err := ltxcatalog.Load(filepath.Dir(deps.config.DatabasePath))
	if err != nil || saved.Total != 2 || saved.Items[0].Code != "ABC-123" {
		t.Fatal(saved, err)
	}
	req := httptest.NewRequest(http.MethodOptions, "/extension/ltx/catalog", nil)
	req.Header.Set("Origin", javBossExtensionOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	// The catalogue is protected by the normal session, never a website origin.
	if got := performRequest(router, "GET", "/ltx/catalog", nil, "", nil); got.Code != 401 {
		t.Fatal(got.Code)
	}
}

func TestLtxRenameKeepsIdentityHighlightsAndOwnership(t *testing.T) {
	deps := newTestDependencies()
	root := t.TempDir()
	deps.config = &common.Config{DatabasePath: filepath.Join(t.TempDir(), "test.db")}
	testAuthService(deps, t)
	for _, row := range []any{&models.Directory{ID: 1, Path: root}, &models.Video{ID: 1, Fingerprint: "identity", PlayCount: 7, HighlightCount: 1}, &models.Jav{ID: 1, Code: "ABC-123"}} {
		if err := deps.database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	javID := int64(1)
	stamp := time.Unix(1700000000, 0).UTC()
	if err := deps.database.Create(&models.VideoLocation{ID: 1, VideoID: 1, DirectoryID: 1, JavID: &javID, RelativePath: "original_CD1.mp4", Filename: "original_CD1.mp4", ModifiedAt: stamp}).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"original_CD1.mp4", "original_CD1.zh-CN.srt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(filepath.Join(root, "original_CD1.mp4"), stamp, stamp); err != nil {
		t.Fatal(err)
	}
	highlightPath := highlights.Path(filepath.Dir(deps.config.DatabasePath), 1)
	if err := os.MkdirAll(filepath.Dir(highlightPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(highlightPath, []byte(`{"version":1,"items":[{"time":12,"label":"test"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.PATCH("/videos/:id/locations/:location_id/ltx", renameVideoLocation)
	rename := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PATCH", "/videos/1/locations/1/ltx", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	w := rename(`{"filename":"ABC-123_LTX_CD1.mp4","ltx":true}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var video models.Video
	if err := json.Unmarshal(w.Body.Bytes(), &video); err != nil {
		t.Fatal(err)
	}
	if video.ID != 1 || video.PlayCount != 7 || video.HighlightCount != 1 || !video.ModifiedAt.Equal(stamp) {
		t.Fatalf("identity or time changed: %+v", video)
	}
	if _, err := os.Stat(filepath.Join(root, "ABC-123_LTX_CD1.zh-CN.srt")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(highlightPath); err != nil || !strings.Contains(string(data), `"time":12`) {
		t.Fatal("highlights lost", err)
	}
	owned, err := dbOwnershipForTest(t, "ABC-123")
	if err != nil || !owned {
		t.Fatal("LTX ownership missing", err)
	}
	if w := rename(`{"filename":"ABC-123_CD1.mp4","ltx":true}`); w.Code != 400 {
		t.Fatal("mismatched mark accepted", w.Code)
	}
	if w := rename(`{"filename":"ABC-123_LTX_CD1.mkv","ltx":true}`); w.Code != 400 {
		t.Fatal("extension change accepted", w.Code)
	}
	if w := rename(`{"filename":"ABC-123_CD1.mp4","ltx":false}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	owned, err = dbOwnershipForTest(t, "ABC-123")
	if err != nil || owned {
		t.Fatal("cancel did not clear LTX", err)
	}
}

func dbOwnershipForTest(t *testing.T, code string) (bool, error) {
	t.Helper()
	rows, err := dbpkg.Default().LookupJavOwnership(t.Context(), []string{code})
	if err != nil {
		return false, err
	}
	return rows[0].LTX, nil
}
