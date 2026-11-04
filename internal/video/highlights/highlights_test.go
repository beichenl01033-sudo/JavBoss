package highlights

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacySidecarEditsAndConcurrentRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "highlights.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"items":[{"time":20.25,"label":"later"},{"time":0,"label":"start"},{"time":-1,"label":"invalid"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Read(path)
	if err != nil || len(doc.Items) != 2 || doc.Items[0].Time != 0 {
		t.Fatalf("legacy document: %+v, %v", doc, err)
	}
	name := "新名称"
	renamed, err := Edit(path, doc.Revision, 0, &name)
	if err != nil || renamed.Items[0].Label != name {
		t.Fatalf("rename: %+v, %v", renamed, err)
	}
	if _, err := Edit(path, doc.Revision, 20.25, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale edit accepted: %v", err)
	}
	deleted, err := Edit(path, renamed.Revision, 0, nil)
	if err != nil || len(deleted.Items) != 1 || deleted.Items[0].Time != 20.25 {
		t.Fatalf("delete: %+v, %v", deleted, err)
	}
	if _, err := Edit(path, deleted.Revision, 20.25, nil); err != nil {
		t.Fatal(err)
	}
	empty, err := Read(path)
	if err != nil || len(empty.Items) != 0 {
		t.Fatalf("clear: %+v %v", empty, err)
	}
}

func TestMissingAndMalformedSidecars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "highlights.json")
	missing, err := Read(path)
	if err != nil || missing.Items == nil || missing.Revision == "" {
		t.Fatalf("missing: %+v %v", missing, err)
	}
	if err := os.WriteFile(path, []byte(`{"items":[`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("partially written JSON should not be treated as empty")
	}
}
