// Package ltxcatalog stores complete, browser-observed public catalogue snapshots.
// A failed or partial import never replaces the last successful snapshot.
package ltxcatalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const MaxItems = 20000
const MaxBytes = 4 << 20
const Source = "https://ltxlib.dpdns.org"

type Item struct {
	Code    string `json:"code"`
	Pending bool   `json:"pending"`
}
type Snapshot struct {
	Source    string    `json:"source"`
	Complete  bool      `json:"complete"`
	Total     int       `json:"total"`
	Confirmed int       `json:"confirmed"`
	Pending   int       `json:"pending"`
	UpdatedAt time.Time `json:"updated_at"`
	Items     []Item    `json:"items"`
}

var mu sync.Mutex
var codePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,127}$`)

func Key(code string) string {
	return strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToUpper(strings.TrimSpace(code)))
}

func Validate(s Snapshot) error {
	if s.Source != Source || !s.Complete || s.Total <= 0 || s.Total > MaxItems || len(s.Items) != s.Total || s.Pending < 0 || s.Confirmed < 0 || s.Pending+s.Confirmed != s.Total {
		return errors.New("incomplete catalogue snapshot")
	}
	seen := make(map[string]bool, s.Total)
	pending := 0
	for _, item := range s.Items {
		key := Key(item.Code)
		if !codePattern.MatchString(item.Code) || key == "" || seen[key] {
			return errors.New("invalid or duplicate catalogue code")
		}
		seen[key] = true
		if item.Pending {
			pending++
		}
	}
	if pending != s.Pending {
		return errors.New("catalogue status counts do not match")
	}
	return nil
}

func Load(root string) (Snapshot, error) {
	mu.Lock()
	defer mu.Unlock()
	var s Snapshot
	if root == "" {
		return s, errors.New("catalogue storage unavailable")
	}
	f, err := os.Open(filepath.Join(root, "ltx", "catalog.json"))
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{Source: Source, Items: []Item{}}, nil
	}
	if err != nil {
		return s, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return s, err
	}
	if len(raw) > MaxBytes {
		return s, errors.New("catalogue cache too large")
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return s, err
	}
	if err := Validate(s); err != nil {
		return s, err
	}
	return s, nil
}

func Save(root string, s Snapshot) (Snapshot, error) {
	mu.Lock()
	defer mu.Unlock()
	if root == "" {
		return s, errors.New("catalogue storage unavailable")
	}
	if err := Validate(s); err != nil {
		return s, err
	}
	s.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > MaxBytes {
		return s, errors.New("catalogue serialization failed")
	}
	dir := filepath.Join(root, "ltx")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return s, err
	}
	f, err := os.CreateTemp(dir, ".catalog-*.json")
	if err != nil {
		return s, err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(raw)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return s, writeErr
	}
	if closeErr != nil {
		return s, closeErr
	}
	if err := os.Rename(f.Name(), filepath.Join(dir, "catalog.json")); err != nil {
		return s, fmt.Errorf("replace catalogue cache: %w", err)
	}
	return s, nil
}
