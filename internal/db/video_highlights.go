package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"
	"javboss/internal/models"
	"javboss/internal/video/highlights"
)

type highlightFileStamp struct {
	Modified time.Time
	Size     int64
	Count    int
}
type highlightIndex struct {
	sync.Mutex
	root    string
	checked time.Time
	files   map[int64]highlightFileStamp
}

func (store *Store) VideoHighlightCounts(ctx context.Context) (map[int64]int, error) {
	var rows []struct {
		ID             int64
		HighlightCount int
	}
	if err := store.conn.WithContext(ctx).Model(&models.Video{}).Select("id, highlight_count").Where("highlight_count > 0").Find(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.ID] = row.HighlightCount
	}
	return counts, nil
}

// SyncVideoHighlights only inspects local application sidecars, never media/NAS
// paths. Counts are reconciled before SQL pagination; malformed/in-progress files
// retain their last known count and are retried on the next request.
func (store *Store) SyncVideoHighlights(ctx context.Context, dataDir string, force bool) error {
	index := &store.highlightIndex
	index.Lock()
	defer index.Unlock()
	if dataDir == "" {
		return nil
	}
	if !force && index.root == dataDir && time.Since(index.checked) < time.Second {
		return nil
	}
	if index.root != dataDir {
		index.files = nil
	}
	if index.files == nil {
		index.files = map[int64]highlightFileStamp{}
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "video"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	counts := map[int64]int{}
	current, err := store.VideoHighlightCounts(ctx)
	if err != nil {
		return err
	}
	pending := map[int64]highlightFileStamp{}
	seen := map[int64]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		path := highlights.Path(dataDir, id)
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		seen[id] = true
		stamp := highlightFileStamp{Modified: info.ModTime(), Size: info.Size(), Count: index.files[id].Count}
		if !force && index.files[id] == stamp && current[id] == stamp.Count {
			continue
		}
		doc, err := highlights.Read(path)
		if err != nil {
			continue
		}
		counts[id] = len(doc.Items)
		stamp.Count = len(doc.Items)
		pending[id] = stamp
		// Cache only after successful database writes below.
	}
	for id := range current {
		if !seen[id] {
			counts[id] = 0
			delete(index.files, id)
		}
	}
	if err := store.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for id, count := range counts {
			if err := tx.Model(&models.Video{}).Where("id = ? AND highlight_count <> ?", id, count).UpdateColumn("highlight_count", count).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("index highlights: %w", err)
	}
	for id, stamp := range pending {
		index.files[id] = stamp
	}
	index.root, index.checked = dataDir, time.Now()
	return nil
}
