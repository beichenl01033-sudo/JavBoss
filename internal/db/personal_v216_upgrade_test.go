package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"javboss/internal/models"
	"javboss/internal/video/highlights"
)

func TestPersonalV216UpgradePreservesHighlightsAndPlaybackMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "upgrade.db")
	legacy, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { legacy.Close() })
	if err := goose.UpToContext(t.Context(), legacy, migrationDir, 202611040001); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO directory(id, path) VALUES (1, '/test-library')`,
		`INSERT INTO video(id, fingerprint, watched_ms, play_count, highlight_count) VALUES (1, 'personal-upgrade', 120000, 7, 2)`,
		`INSERT INTO video_location(id, video_id, directory_id, relative_path, filename, modified_at)
		 VALUES (1, 1, 1, 'nested/movie.mp4', 'movie.mp4', '2001-02-03 04:05:06+00:00')`,
	} {
		if _, err := legacy.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	marks := []byte(`{"version":1,"items":[{"time":0,"label":"first"},{"time":20,"label":"second"}]}`)
	sidecar := highlights.Path(root, 1)
	if err := os.MkdirAll(filepath.Dir(sidecar), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, marks, 0600); err != nil {
		t.Fatal(err)
	}
	upgraded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeDBForSchemaCompare(t, upgraded)
	fresh, err := Open(filepath.Join(root, "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer closeDBForSchemaCompare(t, fresh)
	assertSchemaSnapshotEqual(t, loadSchemaSnapshot(t, upgraded), loadSchemaSnapshot(t, fresh))
	var video models.Video
	if err := upgraded.First(&video, 1).Error; err != nil {
		t.Fatal(err)
	}
	if video.WatchedMS != 120000 || video.PlayCount != 7 || video.HighlightCount != 2 ||
		video.SubtitleSettings != (models.SubtitleSettings{}) || video.MediaInfo.Version != 0 {
		t.Fatalf("upgrade changed personal playback state: %+v", video)
	}
	var location models.VideoLocation
	if err := upgraded.First(&location, 1).Error; err != nil || location.RelativePath != "nested/movie.mp4" || location.ModifiedAt.Year() != 2001 {
		t.Fatalf("upgrade changed directory or file time: %+v, %v", location, err)
	}
	store := NewStore(upgraded)
	offset, track := int64(1200), "off"
	if found, err := store.UpdateVideoSubtitleSettings(t.Context(), 1, &offset, &track); err != nil || !found {
		t.Fatalf("subtitle update: %v, %v", found, err)
	}
	info := models.MediaInfo{Version: models.MediaInfoVersion, VideoCodec: "hevc", Container: "mp4", Width: 1920, Height: 1080, DurationSeconds: 60}
	if saved, err := store.FillVideoMediaInfo(t.Context(), 1, info); err != nil || !saved {
		t.Fatalf("media info update: %v, %v", saved, err)
	}
	if err := upgraded.Model(&models.Video{}).Where("id = ?", 1).UpdateColumn("highlight_count", 3).Error; err != nil {
		t.Fatal(err)
	}
	// A stale scanner record cannot erase independently saved personal or official state.
	if err := store.SaveVideo(t.Context(), &video); err != nil {
		t.Fatal(err)
	}
	var saved models.Video
	if err := upgraded.First(&saved, 1).Error; err != nil || saved.HighlightCount != 3 || saved.WatchedMS != 120000 ||
		saved.SubtitleSettings.OffsetMS != offset || saved.SubtitleSettings.Track != track || saved.MediaInfo != info {
		t.Fatalf("scan overwrote playback state: %+v, %v", saved, err)
	}
	if after, err := os.ReadFile(sidecar); err != nil || string(after) != string(marks) {
		t.Fatal("upgrade or scan changed highlight sidecar", err)
	}
}
