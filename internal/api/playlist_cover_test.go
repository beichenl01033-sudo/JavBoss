package api

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
)

func TestPlaylistCoverUsesCachedImagesWithoutReadingNetworkVideo(t *testing.T) {
	root := t.TempDir()
	database, err := dbpkg.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn, _ := database.DB(); _ = conn.Close() })
	deps := newTestDependencies()
	deps.database = database
	deps.config = &common.Config{DatabasePath: filepath.Join(root, "test.db"), JavCoverDir: filepath.Join(root, "covers")}
	deps.bindDatabase(t)
	directory := models.Directory{Path: "/not-mounted-network-video"}
	video := models.Video{Fingerprint: "cover", DurationSec: 300}
	for _, model := range []any{&directory, &video} {
		if err := database.Create(model).Error; err != nil {
			t.Fatal(err)
		}
	}
	location, err := dbpkg.Default().UpsertVideoLocation(t.Context(), video.ID, directory.ID, "not-mounted.mp4", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	thumbnail := filepath.Join(root, "video", strconv.FormatInt(video.ID, 10), "screenshot", "128.jpg")
	if err := os.MkdirAll(filepath.Dir(thumbnail), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(thumbnail, []byte("cached thumbnail"), 0600); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/videos/:id/playlist-cover", getPlaylistCover)
	request := func(query string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/videos/"+strconv.FormatInt(video.ID, 10)+"/playlist-cover"+query, nil))
		return response
	}
	response := request("")
	if response.Code != 200 || response.Body.String() != "cached thumbnail" {
		t.Fatalf("thumbnail fallback: %d %s", response.Code, response.Body.String())
	}
	work := models.Jav{Code: "COVER-001", CoverCropPosition: 0.25}
	if err := database.Create(&work).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(location).Update("jav_id", work.ID).Error; err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 240, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 240; x++ {
			img.SetRGBA(x, y, color.RGBA{R: byte(x*7 + y*13), G: byte(x * y), B: byte(x ^ y), A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(deps.config.JavCoverDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deps.config.JavCoverDir, "cover-001.png"), buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	response = request("?location_id=" + strconv.FormatInt(location.ID, 10))
	if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), buffer.Bytes()) || response.Header().Get("X-JavBoss-Cover-Position") != "0.25" {
		t.Fatalf("poster/crop: %d %s", response.Code, response.Header())
	}
	custom := "mpv_00-01-02.png"
	if err := os.WriteFile(filepath.Join(filepath.Dir(thumbnail), custom), []byte("custom cover"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&video).Update("cover_screenshot_name", custom).Error; err != nil {
		t.Fatal(err)
	}
	response = request("")
	if response.Code != 200 || response.Body.String() != "custom cover" || response.Header().Get("X-JavBoss-Cover-Aspect") != "0" {
		t.Fatal("custom cover did not take precedence")
	}
	if response = request("?location_id=9999"); response.Code != 404 {
		t.Fatalf("mismatched location accepted: %d", response.Code)
	}
	if response = request("?location_id=-1"); response.Code != 400 {
		t.Fatalf("negative location accepted: %d", response.Code)
	}
	if err := database.Model(&video).Update("cover_screenshot_name", "").Error; err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(thumbnail); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(deps.config.JavCoverDir, "cover-001.png")); err != nil {
		t.Fatal(err)
	}
	if response = request(""); response.Code != 404 {
		t.Fatalf("missing artwork should not trigger extraction: %d", response.Code)
	}
}
