package ltxcatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotValidationAndLastCompleteCache(t *testing.T) {
	root := t.TempDir()
	empty, err := Load(root)
	if err != nil || empty.Complete || len(empty.Items) != 0 {
		t.Fatalf("empty: %+v, %v", empty, err)
	}
	good := Snapshot{Source: Source, Complete: true, Total: 2, Confirmed: 1, Pending: 1, Items: []Item{{Code: "ABC-123"}, {Code: "DEF-456", Pending: true}}}
	saved, err := Save(root, good)
	if err != nil || saved.UpdatedAt.IsZero() {
		t.Fatal(err)
	}
	for _, change := range []func(*Snapshot){
		func(s *Snapshot) { s.Complete = false }, func(s *Snapshot) { s.Total = 3 }, func(s *Snapshot) { s.Pending = 0 },
		func(s *Snapshot) { s.Source = "https://evil.test" }, func(s *Snapshot) { s.Items = []Item{{Code: "abc_123"}, {Code: "ABC-123", Pending: true}} },
		func(s *Snapshot) { s.Items = []Item{{Code: "../private"}, {Code: "DEF-456", Pending: true}} },
	} {
		bad := good
		change(&bad)
		if _, err := Save(root, bad); err == nil {
			t.Fatal("invalid snapshot accepted")
		}
		current, err := Load(root)
		if err != nil || !current.UpdatedAt.Equal(saved.UpdatedAt) || current.Total != 2 {
			t.Fatalf("old cache changed: %+v, %v", current, err)
		}
	}
	// Replacement persists on disk and does not depend on the server process.
	good.Items[0].Code = "GHI-789"
	if _, err := Save(root, good); err != nil {
		t.Fatal(err)
	}
	current, err := Load(root)
	if err != nil || current.Items[0].Code != "GHI-789" {
		t.Fatal(current, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "ltx", "catalog.json"))
	if err != nil || !json.Valid(raw) {
		t.Fatal(err)
	}
}
