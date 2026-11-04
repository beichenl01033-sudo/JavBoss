// Package highlights reads the existing MPV sidecar format. Sidecars remain the
// source of truth; the database stores only a rebuildable count for catalog queries.
package highlights

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Item struct {
	Time  float64 `json:"time"`
	Label string  `json:"label"`
}

type Document struct {
	Version  int    `json:"version"`
	Items    []Item `json:"items"`
	Revision string `json:"revision,omitempty"`
}

var editMu sync.Mutex
var ErrConflict = errors.New("highlights changed; refresh and retry")

func Path(dataDir string, videoID int64) string {
	return filepath.Join(dataDir, "video", strconv.FormatInt(videoID, 10), "highlights.json")
}

func Read(path string) (Document, error) {
	doc := Document{Version: 1, Items: []Item{}}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return withRevision(doc), nil
	}
	if err != nil {
		return doc, err
	}
	defer f.Close()
	const maxSize = 2 << 20
	raw, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil {
		return doc, err
	}
	if len(raw) > maxSize {
		return doc, errors.New("highlight file too large")
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return doc, fmt.Errorf("read highlights: %w", err)
	}
	items := make([]Item, 0, len(doc.Items))
	for _, item := range doc.Items {
		if math.IsNaN(item.Time) || math.IsInf(item.Time, 0) || item.Time < 0 {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Time < items[j].Time })
	doc.Items, doc.Version = items, 1
	return withRevision(doc), nil
}

func withRevision(doc Document) Document {
	raw, _ := json.Marshal(doc.Items)
	sum := sha256.Sum256(raw)
	doc.Revision = hex.EncodeToString(sum[:])
	return doc
}

// Edit uses the displayed revision to avoid overwriting another edit or an MPV save.
func Edit(path, revision string, at float64, label *string) (Document, error) {
	editMu.Lock()
	defer editMu.Unlock()
	doc, err := Read(path)
	if err != nil {
		return doc, err
	}
	if revision == "" || revision != doc.Revision {
		return doc, ErrConflict
	}
	index := -1
	for i, item := range doc.Items {
		if item.Time == at {
			index = i
			break
		}
	}
	if index < 0 {
		return doc, ErrConflict
	}
	if label == nil {
		doc.Items = append(doc.Items[:index], doc.Items[index+1:]...)
	} else {
		value := strings.TrimSpace(*label)
		if value == "" || len([]rune(value)) > 200 {
			return doc, errors.New("invalid highlight label")
		}
		doc.Items[index].Label = value
	}
	doc.Revision = ""
	raw, err := json.Marshal(doc)
	if err != nil {
		return doc, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".highlights-*.json")
	if err != nil {
		return doc, err
	}
	tempPath := tmp.Name()
	defer os.Remove(tempPath)
	_, err = tmp.Write(append(raw, '\n'))
	closeErr := tmp.Close()
	if err != nil {
		return doc, err
	}
	if closeErr != nil {
		return doc, closeErr
	}
	latest, err := Read(path)
	if err != nil {
		return doc, err
	}
	if latest.Revision != revision {
		return latest, ErrConflict
	}
	if err := os.Rename(tempPath, path); err != nil {
		return doc, err
	}
	return withRevision(doc), nil
}
