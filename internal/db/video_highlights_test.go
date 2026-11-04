package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/models"
	"javboss/internal/video/highlights"
)

func TestHighlightIndexLegacyImportFilteringAndRemoval(t *testing.T) {
	deps := newTestDependencies()
	database := openTestDB(deps, t)
	ctx := context.Background()
	root := t.TempDir()
	directory := models.Directory{Path: "/not-mounted-nas"}
	if err := database.Create(&directory).Error; err != nil {
		t.Fatal(err)
	}
	videos := []models.Video{
		{Fingerprint: "no-marks", DirectoryID: directory.ID, Path: "no.mp4"},
		{Fingerprint: "old-marks", DirectoryID: directory.ID, Path: "yes.mp4"},
	}
	if err := database.Create(&videos).Error; err != nil {
		t.Fatal(err)
	}
	createVideoLocationsForVideos(t, database, videos...)
	path := highlights.Path(root, videos[1].ID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	write := func(raw string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"version":1,"items":[{"time":0,"label":"start"},{"time":32,"label":"later"}]}`)
	sync := func() {
		t.Helper()
		if err := deps.store.SyncVideoHighlights(ctx, root, true); err != nil {
			t.Fatal(err)
		}
	}
	sync()
	filter := VideoFilter{HighlightOnly: true}
	items, err := deps.store.ListVideos(ctx, 1, 0, nil, "", "filename", nil, []int64{directory.ID}, filter)
	if err != nil || len(items) != 1 || items[0].ID != videos[1].ID || items[0].HighlightCount != 2 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	count, err := deps.store.CountVideos(ctx, nil, "", []int64{directory.ID}, filter)
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	// New scans of an old struct cannot erase the independently maintained count.
	if err := deps.store.SaveVideo(ctx, &videos[1]); err != nil {
		t.Fatal(err)
	}
	counts, err := deps.store.VideoHighlightCounts(ctx)
	if err != nil || counts[videos[1].ID] != 2 {
		t.Fatalf("scan erased count: %v %v", counts, err)
	}
	write(`{"items":[`)
	sync()
	counts, _ = deps.store.VideoHighlightCounts(ctx)
	if counts[videos[1].ID] != 2 {
		t.Fatal("in-progress MPV save erased count")
	}
	write(`{"version":1,"items":[]}`)
	sync()
	counts, _ = deps.store.VideoHighlightCounts(ctx)
	if len(counts) != 0 {
		t.Fatalf("last mark deletion not indexed: %v", counts)
	}
	write(`{"version":1,"items":[{"time":12,"label":"restored"}]}`)
	sync()
	// Simulate application restart with a fresh Store and the same sidecars.
	restarted := NewStore(database)
	if err := restarted.SyncVideoHighlights(ctx, root, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := restarted.SyncVideoHighlights(ctx, root, true); err != nil {
		t.Fatal(err)
	}
	counts, _ = restarted.VideoHighlightCounts(ctx)
	if len(counts) != 0 {
		t.Fatalf("removed file not reconciled: %v", counts)
	}
}

func TestJavHighlightFilterUsesVisibleLocationsAndCountsBeforePagination(t *testing.T) {
	deps := newTestDependencies()
	database := openTestDB(deps, t)
	ctx := context.Background()
	dirs := []models.Directory{{Path: "/one"}, {Path: "/two"}}
	if err := database.Create(&dirs).Error; err != nil {
		t.Fatal(err)
	}
	works := []models.Jav{{Code: "HL-001"}, {Code: "HL-002"}}
	if err := database.Create(&works).Error; err != nil {
		t.Fatal(err)
	}
	videos := []models.Video{{Fingerprint: "marked", HighlightCount: 2}, {Fingerprint: "unmarked"}}
	if err := database.Create(&videos).Error; err != nil {
		t.Fatal(err)
	}
	for i := range videos {
		loc, err := deps.store.UpsertVideoLocation(ctx, videos[i].ID, dirs[i].ID, "file.mp4", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := database.Model(loc).Update("jav_id", works[i].ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	filter := JavSearchFilters{StudioID: -1, HighlightOnly: true}
	items, total, err := deps.store.SearchJavWithPrefixFilters(ctx, nil, nil, "", "", "code", 1, 0, nil, nil, filter)
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != works[0].ID {
		t.Fatalf("items=%v total=%d err=%v", items, total, err)
	}
	_, total, err = deps.store.SearchJavWithPrefixFilters(ctx, nil, nil, "", "", "code", 1, 0, nil, []int64{dirs[1].ID}, filter)
	if err != nil || total != 0 {
		t.Fatalf("directory scope leaked: %d %v", total, err)
	}
}
