package ltx

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenameFamilyPreservesTimeAndNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "ABC-123.mp4")
	next := filepath.Join(root, "ABC-123_LTX.mp4")
	names := []string{"ABC-123.mp4", "ABC-123.zh-CN.srt", "ABC-123.nfo", "ABC-123-poster.jpg", "ABC-1234.srt", "ABC-123.other.mp4"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	stamp := time.Unix(1700000000, 0)
	if err := os.Chtimes(old, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	undo, err := RenameFamily(old, next)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ABC-123_LTX.mp4", "ABC-123_LTX.zh-CN.srt", "ABC-123_LTX.nfo", "ABC-123_LTX-poster.jpg", "ABC-1234.srt", "ABC-123.other.mp4"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal(name, err)
		}
	}
	info, err := os.Stat(next)
	if err != nil || !info.ModTime().Equal(stamp) {
		t.Fatal("modified time changed", err)
	}
	if err := undo(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ABC-123_LTX.zh-CN.srt"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RenameFamily(old, next); !errors.Is(err, os.ErrExist) {
		t.Fatal("sidecar collision accepted", err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("video moved despite collision", err)
	}
	// An independently created destination is also protected by the OS move.
	if err := os.WriteFile(next, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := moveExclusive(old, next); err == nil {
		t.Fatal("exclusive move overwrote target")
	}
	data, _ := os.ReadFile(next)
	if string(data) != "keep" {
		t.Fatal("target overwritten")
	}
}

func TestRenameFamilyRollback(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "ABC-123.mp4")
	next := filepath.Join(root, "ABC-123_LTX.mp4")
	for _, path := range []string{old, filepath.Join(root, "ABC-123.srt")} {
		if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	_, err := renameFamily(old, next, func(a, b string) error {
		calls++
		if calls == 2 {
			return errors.New("injected failure")
		}
		return moveExclusive(a, b)
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("rollback lost original", err)
	}
	if _, err := os.Stat(next); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("new path remains", err)
	}
}

func TestRenameFamilyRejectsSharedSidecars(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"ABC-123.mp4", "ABC-123.mkv", "ABC-123.srt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("shared"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := RenameFamily(filepath.Join(root, "ABC-123.mp4"), filepath.Join(root, "ABC-123_LTX.mp4"))
	if !errors.Is(err, ErrSharedSidecars) {
		t.Fatal("shared subtitle moved", err)
	}
	for _, name := range []string{"ABC-123.mp4", "ABC-123.mkv", "ABC-123.srt"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
}
