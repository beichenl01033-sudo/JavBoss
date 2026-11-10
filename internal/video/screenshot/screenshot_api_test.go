package screenshot_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/models"
	"javboss/internal/video/screenshot"
)

func TestGetThumbnailReturnsCachedImageWithoutGenerationMetadata(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "video", "1", "screenshot", "128.jpg")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("cached image"), 0600); err != nil {
		t.Fatal(err)
	}
	screenshot.Start(t.Context(), dataDir)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	// No database is installed: a cached thumbnail must not fetch or regenerate the video.
	got, err := screenshot.GetThumbnail(ctx, &models.Video{ID: 1, DurationSec: 300})
	if err != nil || got != path {
		t.Fatalf("GetThumbnail = %q, %v; want %q", got, err, path)
	}
}

func TestGetThumbnailReportsUnavailableTimestamp(t *testing.T) {
	screenshot.Start(t.Context(), t.TempDir())
	for _, video := range []*models.Video{nil, {ID: 1, DurationSec: 0}, {ID: 1, DurationSec: -1}} {
		if _, err := screenshot.GetThumbnail(t.Context(), video); !errors.Is(err, screenshot.ErrNoThumbnail) {
			t.Fatalf("GetThumbnail(%+v) = %v, want ErrNoThumbnail", video, err)
		}
	}
}

func TestCachedThumbnailPathDoesNotRequireQueueOrMountedVideo(t *testing.T) {
	root := t.TempDir()
	video := &models.Video{ID: 99, DurationSec: 300, Path: "/unmounted/network.mp4"}
	if _, ok := screenshot.CachedThumbnailPath(root, video); ok {
		t.Fatal("missing cache reported present")
	}
	path := filepath.Join(root, "video", "99", "screenshot", "128.jpg")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("cached"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, ok := screenshot.CachedThumbnailPath(root, video); !ok || got != path {
		t.Fatalf("cache=%s %t", got, ok)
	}
}
