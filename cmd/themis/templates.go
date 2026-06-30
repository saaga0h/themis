package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/saaga0h/themis/templates"
)

// resolveTemplateDir returns a directory containing the pipeline templates plus a
// cleanup func the caller must defer.
//
// When override is non-empty it is used directly — this is the --templates escape
// hatch for editing templates on disk without rebuilding. Otherwise the embedded
// templates are materialized into a temp directory, which keeps the runner
// path-based (it reads templates by filename) while the binary stays
// self-contained and runnable against any target repo.
func resolveTemplateDir(override string) (dir string, cleanup func(), err error) {
	noop := func() {}
	if override != "" {
		return override, noop, nil
	}

	tmp, err := os.MkdirTemp("", "themis-templates-")
	if err != nil {
		return "", noop, fmt.Errorf("creating template temp dir: %w", err)
	}
	cleanup = func() { os.RemoveAll(tmp) }

	entries, err := fs.ReadDir(templates.FS, ".")
	if err != nil {
		cleanup()
		return "", noop, fmt.Errorf("reading embedded templates: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, rerr := templates.FS.ReadFile(e.Name())
		if rerr != nil {
			cleanup()
			return "", noop, fmt.Errorf("reading embedded template %s: %w", e.Name(), rerr)
		}
		if werr := os.WriteFile(filepath.Join(tmp, e.Name()), data, 0o644); werr != nil {
			cleanup()
			return "", noop, fmt.Errorf("materializing template %s: %w", e.Name(), werr)
		}
	}
	return tmp, cleanup, nil
}
