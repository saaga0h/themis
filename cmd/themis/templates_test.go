package main

import (
	"os"
	"path/filepath"
	"testing"
)

// With no override, the embedded templates are materialized to a real directory
// the runner can read, and cleanup removes it.
func TestResolveTemplateDir_MaterializesEmbedded(t *testing.T) {
	dir, cleanup, err := resolveTemplateDir("")
	if err != nil {
		t.Fatalf("resolveTemplateDir: %v", err)
	}
	defer cleanup()

	for _, name := range []string{"implement.md", "review.md", "ship.md", "test-red.md", "update-docs.md"} {
		data, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			t.Errorf("materialized template %s missing: %v", name, rerr)
			continue
		}
		if len(data) == 0 {
			t.Errorf("materialized template %s is empty", name)
		}
	}

	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("cleanup did not remove the temp dir %s (err=%v)", dir, err)
	}
}

// An explicit --templates override is used verbatim, with no temp dir created.
func TestResolveTemplateDir_OverrideUsedDirectly(t *testing.T) {
	want := t.TempDir()
	dir, cleanup, err := resolveTemplateDir(want)
	if err != nil {
		t.Fatalf("resolveTemplateDir: %v", err)
	}
	defer cleanup()
	if dir != want {
		t.Errorf("override dir = %q, want %q", dir, want)
	}
}
