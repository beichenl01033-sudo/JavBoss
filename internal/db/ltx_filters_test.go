package db

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"javboss/internal/models"
)

func TestLtxFiltersFilesWorksAndEntities(t *testing.T) {
	deps := newTestDependencies()
	database := openTestDB(deps, t)
	ctx := context.Background()
	dirs := []models.Directory{{Path: "/media"}, {Path: "/other"}, {Path: "/disabled"}}
	if err := database.Create(&dirs).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&dirs[2]).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	studio := models.JavStudio{Name: "ltx studio"}
	series := models.JavSeries{Name: "ltx series"}
	idol := models.JavIdol{Name: "ltx idol"}
	for _, row := range []any{&studio, &series, &idol} {
		if err := database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	works := []models.Jav{{Code: "ABC-001"}, {Code: "ABC-002"}, {Code: "ABC-003"}, {Code: "ABC-004"}}
	for i := range works {
		works[i].StudioID, works[i].SeriesID = &studio.ID, &series.ID
	}
	if err := database.Create(&works).Error; err != nil {
		t.Fatal(err)
	}
	for _, work := range works {
		if err := database.Create(&models.JavIdolMap{JavID: work.ID, JavIdolID: idol.ID}).Error; err != nil {
			t.Fatal(err)
		}
	}
	add := func(work int, dir int, name string, marks int) {
		t.Helper()
		video := models.Video{Fingerprint: fmt.Sprintf("%d/%s", dir, name), HighlightCount: marks}
		if err := database.Create(&video).Error; err != nil {
			t.Fatal(err)
		}
		loc, err := deps.store.UpsertVideoLocation(ctx, video.ID, dirs[dir].ID, name, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if work >= 0 {
			if err := database.Model(loc).Update("jav_id", works[work].ID).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	add(0, 0, "a/ABC-001.mp4", 1)
	add(0, 0, "b/ABC-001_LTX.mp4", 2)
	add(1, 0, "a/ABC-002.mp4", 0)
	add(1, 2, "ABC-002_LTX.mp4", 0)      // disabled copies do not affect absence
	add(2, 0, "LTX/ABC-003_LTX2.mp4", 1) // neither folder nor partial token is LTX
	add(3, 1, "ABC-004_ltx.mkv", 1)
	add(-1, 0, "unknown_LTX.mp4", 1)
	// Legacy nullable filenames fall back to the basename of relative_path.
	if err := database.Model(&models.VideoLocation{}).Where("relative_path = ?", "b/ABC-001_LTX.mp4").Update("filename", nil).Error; err != nil {
		t.Fatal(err)
	}
	tag := models.Tag{Name: "tagged"}
	if err := database.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("INSERT INTO video_tag (video_id, tag_id) SELECT id, ? FROM video", tag.ID).Error; err != nil {
		t.Fatal(err)
	}
	scope := []int64{dirs[0].ID}
	cases := []struct {
		name         string
		filter       LtxFilter
		files, works int64
	}{
		{"all", LtxFilter{}, 5, 3},
		{"with", LtxFilter{Local: "with"}, 2, 1},
		{"without", LtxFilter{Local: "without"}, 3, 2},
		{"confirmed", LtxFilter{Catalog: "confirmed", CodesJSON: `["ABC001"]`}, 2, 1},
		{"pending", LtxFilter{Catalog: "pending", CodesJSON: `["ABC002"]`}, 1, 1},
		{"not listed", LtxFilter{Catalog: "not_found", CodesJSON: `["ABC001","ABC002"]`}, 1, 1},
		{"empty status", LtxFilter{Catalog: "pending", CodesJSON: `[]`}, 0, 0},
		{"combined", LtxFilter{Local: "without", Catalog: "confirmed", CodesJSON: `["ABC001","ABC002"]`}, 2, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter := VideoFilter{Ltx: tc.filter}
			count, err := deps.store.CountVideos(ctx, nil, "", scope, filter)
			if err != nil || count != tc.files {
				t.Fatalf("files %d want %d: %v", count, tc.files, err)
			}
			taggedCount, err := deps.store.CountVideos(ctx, []string{tag.Name}, "", scope, filter)
			if err != nil || taggedCount != tc.files {
				t.Fatalf("tagged files %d want %d: %v", taggedCount, tc.files, err)
			}
			items, err := deps.store.ListVideos(ctx, 1, 0, nil, "", "filename", nil, scope, filter)
			if err != nil || (tc.files > 0 && len(items) != 1) || (tc.files == 0 && len(items) != 0) {
				t.Fatalf("page %v: %v", items, err)
			}
			jfilter := JavSearchFilters{StudioID: -1, Ltx: tc.filter}
			jitems, total, err := deps.store.SearchJavWithPrefixFilters(ctx, nil, nil, "", "", "code", 1, 0, nil, scope, jfilter)
			if err != nil || total != tc.works {
				t.Fatalf("works %d want %d: %v", total, tc.works, err)
			}
			if tc.filter.Local == "with" && len(jitems) > 0 {
				if len(jitems[0].Videos) != 1 || !ltxMarkedSQL(jitems[0].Videos[0].Filename, "") {
					t.Fatal("unfiltered playback files")
				}
			}
			options, err := deps.store.ListJavFilterOptions(ctx, nil, nil, "", "", scope, jfilter, JavFilterOptionSearches{}, 120)
			if err != nil || options.Total != tc.works {
				t.Fatalf("options %#v: %v", options, err)
			}
			studios, _, err := deps.store.ListJavStudiosInSubdirectory(ctx, "", 20, 0, scope, 0, "", tc.filter)
			if err != nil || (tc.works > 0 && (len(studios) != 1 || studios[0].WorkCount != tc.works)) || (tc.works == 0 && len(studios) != 0) {
				t.Fatalf("studios %+v: %v", studios, err)
			}
			seriesItems, _, err := deps.store.ListJavSeriesInSubdirectory(ctx, "", 20, 0, scope, 0, "", tc.filter)
			if err != nil || (tc.works > 0 && (len(seriesItems) != 1 || seriesItems[0].WorkCount != tc.works)) || (tc.works == 0 && len(seriesItems) != 0) {
				t.Fatalf("series %+v: %v", seriesItems, err)
			}
			idols, _, err := deps.store.ListJavIdols(ctx, "", "", 20, 0, scope, 0, JavIdolFilters{Ltx: tc.filter})
			if err != nil || (tc.works > 0 && (len(idols) != 1 || idols[0].WorkCount != tc.works)) || (tc.works == 0 && len(idols) != 0) {
				t.Fatalf("idols %+v: %v", idols, err)
			}
		})
	}
	// Scoped absence ignores the LTX copy outside a/, and highlight intersection
	// must use the very same location rather than any other copy of the work.
	for _, tc := range []struct {
		sub   string
		local string
		marks bool
		want  int64
	}{
		{"a", "without", false, 2}, {"a", "with", false, 0}, {"b", "with", true, 1}, {"a", "without", true, 1},
	} {
		_, total, err := deps.store.SearchJavWithPrefixFilters(ctx, nil, nil, "", "", "code", 20, 0, nil, scope, JavSearchFilters{StudioID: -1, Subdirectory: tc.sub, Ltx: LtxFilter{Local: tc.local}, HighlightOnly: tc.marks})
		if err != nil || total != tc.want {
			t.Fatalf("scoped %+v: %d %v", tc, total, err)
		}
	}
	// A single JSON bind supports the maximum catalogue without SQLite variable limits.
	codes := make([]string, 20000)
	for i := range codes {
		codes[i] = fmt.Sprintf("TEST%d", i)
	}
	codes[0] = "ABC001"
	raw, _ := json.Marshal(codes)
	count, err := deps.store.CountVideos(ctx, nil, "", scope, VideoFilter{Ltx: LtxFilter{Catalog: "confirmed", CodesJSON: string(raw)}})
	if err != nil || count != 2 {
		t.Fatalf("large catalogue: %d %v", count, err)
	}
}
