package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDescriptor(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".themis"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".themis", "workflow.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// A missing descriptor is not an error — it yields an empty descriptor so a
// project that declares nothing is handled by the caller (gate no-ops, etc.).
func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	d, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(d.Verify) != 0 || len(d.StandardsDocs()) != 0 {
		t.Errorf("expected empty descriptor, got %+v", d)
	}
}

func TestLoad_ParsesVerifyAndDocs(t *testing.T) {
	dir := t.TempDir()
	writeDescriptor(t, dir, `stack: go
verify:
  - go build ./...
  - go test ./...
docs:
  standards: CODING_STANDARDS.md
  glossary: UBIQUITOUS_LANGUAGE.md
`)
	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Stack != "go" {
		t.Errorf("stack: got %q", d.Stack)
	}
	if len(d.Verify) != 2 || d.Verify[0] != "go build ./..." {
		t.Errorf("verify: got %v", d.Verify)
	}
	got := d.StandardsDocs()
	want := []string{"CODING_STANDARDS.md", "UBIQUITOUS_LANGUAGE.md"}
	if len(got) != len(want) {
		t.Fatalf("StandardsDocs: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("StandardsDocs[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// footprint_exempt parses into the always-allowed path list a footprint gate uses.
func TestLoad_ParsesFootprintExempt(t *testing.T) {
	dir := t.TempDir()
	writeDescriptor(t, dir, `stack: go
verify:
  - go build ./...
footprint_exempt:
  - go.mod
  - go.sum
`)
	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(d.FootprintExempt) != 2 || d.FootprintExempt[0] != "go.mod" || d.FootprintExempt[1] != "go.sum" {
		t.Errorf("FootprintExempt: got %v, want [go.mod go.sum]", d.FootprintExempt)
	}
}

// Doc surfaces parse into a glob list used to trigger the Docs step.
func TestLoad_ParsesDocSurfaces(t *testing.T) {
	dir := t.TempDir()
	writeDescriptor(t, dir, `docs:
  standards: CODING_STANDARDS.md
  surfaces:
    - cmd/themis/
    - README.md
    - "docs/*.md"
`)
	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(d.Docs.Surfaces) != 3 || d.Docs.Surfaces[0] != "cmd/themis/" {
		t.Errorf("surfaces: got %v", d.Docs.Surfaces)
	}
}

// StandardsDocs skips unset doc fields and preserves reading order.
func TestStandardsDocs_SkipsUnset(t *testing.T) {
	d := &Descriptor{Docs: Docs{Standards: "S.md", Architecture: "A.md"}}
	got := d.StandardsDocs()
	want := []string{"S.md", "A.md"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
}

// An empty verify entry is a configuration error — it would run as a no-op shell
// command and silently weaken the gate.
func TestLoad_RejectsEmptyVerifyEntry(t *testing.T) {
	dir := t.TempDir()
	writeDescriptor(t, dir, "verify:\n  - \"\"\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for empty verify entry")
	}
}

// Unknown top-level fields are rejected so the schema and loader stay in lockstep.
func TestLoad_RejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	writeDescriptor(t, dir, "bogus: true\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestLoad_ParsesImageAndRuntime(t *testing.T) {
	dir := t.TempDir()
	writeDescriptor(t, dir, "image: themis-myproj:latest\nruntime: docker\n")
	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Image != "themis-myproj:latest" {
		t.Errorf("image: got %q", d.Image)
	}
	if d.Runtime != "docker" {
		t.Errorf("runtime: got %q", d.Runtime)
	}
}

func TestLoad_EmptyRuntimeIsAutodetect(t *testing.T) {
	dir := t.TempDir()
	writeDescriptor(t, dir, "image: x:latest\n")
	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Runtime != "" {
		t.Errorf("runtime: got %q, want empty (autodetect)", d.Runtime)
	}
}

func TestLoad_RejectsInvalidRuntime(t *testing.T) {
	dir := t.TempDir()
	writeDescriptor(t, dir, "runtime: containerd\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for invalid runtime")
	}
}

func TestLoad_ParsesProvider(t *testing.T) {
	dir := t.TempDir()
	writeDescriptor(t, dir, "provider: gitea\n")
	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Provider != "gitea" {
		t.Errorf("provider: got %q, want gitea", d.Provider)
	}
}

func TestLoad_RejectsInvalidProvider(t *testing.T) {
	dir := t.TempDir()
	writeDescriptor(t, dir, "provider: gitlab\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for invalid provider")
	}
}
