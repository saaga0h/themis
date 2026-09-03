package templates

import (
	"io/fs"
	"testing"
)

// The binary must carry every agent-step template, or a run against a target
// repo that lacks a templates/ dir fails at "reading template ...".
func TestEmbeddedTemplates_IncludeEveryAgentStep(t *testing.T) {
	want := []string{"implement.md", "review.md", "ship.md", "test-red.md", "update-docs.md"}
	for _, name := range want {
		data, err := fs.ReadFile(FS, name)
		if err != nil {
			t.Errorf("embedded template %s missing: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("embedded template %s is empty", name)
		}
	}
}
