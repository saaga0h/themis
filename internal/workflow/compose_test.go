package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWriteWorkflow_KnownLanguagesRoundTrip(t *testing.T) {
	cases := []struct {
		lang       string
		wantStack  string
		wantVerify []string
	}{
		{"go", "go", []string{"go build ./...", `test -z "$(gofmt -l .)"`, "go vet ./...", "go test ./..."}},
		{"python", "python", []string{"ruff check .", "pytest"}},
		{"node", "node", []string{"npm ci", "npm run build", "npm test"}},
		{"rust", "rust", []string{"cargo build", "cargo test"}},
	}
	for _, c := range cases {
		t.Run(c.lang, func(t *testing.T) {
			dir := t.TempDir()
			cfg := InitConfig{Language: c.lang, Image: "themis-x:latest"}
			if _, err := WriteWorkflow(dir, cfg, false); err != nil {
				t.Fatalf("WriteWorkflow: %v", err)
			}
			d, err := Load(dir)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if d.Stack != c.wantStack {
				t.Errorf("Stack = %q, want %q", d.Stack, c.wantStack)
			}
			if !reflect.DeepEqual(d.Verify, c.wantVerify) {
				t.Errorf("Verify = %q, want %q", d.Verify, c.wantVerify)
			}
			if d.Image != "themis-x:latest" {
				t.Errorf("Image = %q, want themis-x:latest", d.Image)
			}
			raw, _ := os.ReadFile(filepath.Join(dir, workflowFile))
			if strings.Contains(string(raw), "exit 1") {
				t.Errorf("known-language workflow.yaml must not contain the failing placeholder:\n%s", raw)
			}
		})
	}
}

func TestWriteWorkflow_SeedsFootprintExempt(t *testing.T) {
	// Known language → footprint_exempt seeded from the preset and parsed by Load.
	dir := t.TempDir()
	if _, err := WriteWorkflow(dir, InitConfig{Language: "node"}, false); err != nil {
		t.Fatalf("WriteWorkflow: %v", err)
	}
	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"package.json", "package-lock.json"}; !reflect.DeepEqual(d.FootprintExempt, want) {
		t.Errorf("node FootprintExempt = %q, want %q", d.FootprintExempt, want)
	}

	// go → go.mod/go.sum
	dirGo := t.TempDir()
	if _, err := WriteWorkflow(dirGo, InitConfig{Language: "go"}, false); err != nil {
		t.Fatalf("WriteWorkflow: %v", err)
	}
	dGo, _ := Load(dirGo)
	if want := []string{"go.mod", "go.sum"}; !reflect.DeepEqual(dGo.FootprintExempt, want) {
		t.Errorf("go FootprintExempt = %q, want %q", dGo.FootprintExempt, want)
	}

	// other → none (the commented placeholder is not parsed as a value)
	dirOther := t.TempDir()
	if _, err := WriteWorkflow(dirOther, InitConfig{Language: "other"}, false); err != nil {
		t.Fatalf("WriteWorkflow: %v", err)
	}
	dOther, _ := Load(dirOther)
	if len(dOther.FootprintExempt) != 0 {
		t.Errorf("other FootprintExempt = %q, want empty", dOther.FootprintExempt)
	}
}

func TestWriteWorkflow_ProviderSetVsCommented(t *testing.T) {
	// Set → written uncommented and parsed by the loader.
	dir := t.TempDir()
	if _, err := WriteWorkflow(dir, InitConfig{Language: "go", Provider: "github"}, false); err != nil {
		t.Fatalf("WriteWorkflow: %v", err)
	}
	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Provider != "github" {
		t.Errorf("Provider = %q, want github", d.Provider)
	}

	// Unset → left commented, loader sees empty.
	dir2 := t.TempDir()
	if _, err := WriteWorkflow(dir2, InitConfig{Language: "go"}, false); err != nil {
		t.Fatalf("WriteWorkflow: %v", err)
	}
	d2, err := Load(dir2)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d2.Provider != "" {
		t.Errorf("Provider = %q, want empty (commented)", d2.Provider)
	}
	raw, _ := os.ReadFile(filepath.Join(dir2, workflowFile))
	if !strings.Contains(string(raw), "# provider:") {
		t.Errorf("unset provider must leave a commented provider line:\n%s", raw)
	}
}

func TestWriteWorkflow_RuntimeSetVsCommented(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteWorkflow(dir, InitConfig{Language: "go", Runtime: "docker"}, false); err != nil {
		t.Fatalf("WriteWorkflow: %v", err)
	}
	d, _ := Load(dir)
	if d.Runtime != "docker" {
		t.Errorf("Runtime = %q, want docker", d.Runtime)
	}

	dir2 := t.TempDir()
	if _, err := WriteWorkflow(dir2, InitConfig{Language: "go"}, false); err != nil {
		t.Fatalf("WriteWorkflow: %v", err)
	}
	d2, _ := Load(dir2)
	if d2.Runtime != "" {
		t.Errorf("Runtime = %q, want empty (commented)", d2.Runtime)
	}
}

func TestWriteWorkflow_StackOverride(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteWorkflow(dir, InitConfig{Language: "go", Stack: "custom-label"}, false); err != nil {
		t.Fatalf("WriteWorkflow: %v", err)
	}
	d, _ := Load(dir)
	if d.Stack != "custom-label" {
		t.Errorf("Stack = %q, want custom-label (explicit override)", d.Stack)
	}
}

func TestWriteWorkflow_OtherIsNeutralAndFails(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteWorkflow(dir, InitConfig{Language: "other"}, false); err != nil {
		t.Fatalf("WriteWorkflow: %v", err)
	}
	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Stack != "unconfigured" {
		t.Errorf("Stack = %q, want unconfigured", d.Stack)
	}
	anyFailed := false
	for _, cmd := range d.Verify {
		c := exec.Command("bash", "-c", cmd)
		c.Dir = dir
		if err := c.Run(); err != nil {
			anyFailed = true
		}
	}
	if !anyFailed {
		t.Error("other/neutral verify must fail on purpose")
	}
}

func TestWriteContainerfileFor_KnownSubstitutesToolchain(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteContainerfileFor(dir, InitConfig{Language: "go"}, false); err != nil {
		t.Fatalf("WriteContainerfileFor: %v", err)
	}
	s, _ := os.ReadFile(filepath.Join(dir, "Containerfile"))
	if !strings.Contains(string(s), "go.dev/dl/go") {
		t.Errorf("go Containerfile must contain the toolchain install:\n%s", s)
	}
	if strings.Contains(string(s), "# TODO: install your project's toolchain") {
		t.Error("known-language Containerfile must not keep the toolchain TODO placeholder")
	}
	// Agent layer preserved.
	if !strings.Contains(string(s), "COPY --from=themis-build /themis /usr/local/bin/themis") {
		t.Error("Containerfile must keep the themis agent layer")
	}
}

func TestWriteContainerfileFor_ProviderTools(t *testing.T) {
	// github (and unset, the default) → gh installed in the image; gitea → not.
	for _, prov := range []string{"github", ""} {
		dir := t.TempDir()
		if _, err := WriteContainerfileFor(dir, InitConfig{Language: "go", Provider: prov}, false); err != nil {
			t.Fatalf("WriteContainerfileFor(%q): %v", prov, err)
		}
		s := string(mustRead(t, filepath.Join(dir, "Containerfile")))
		if !strings.Contains(s, "apt-get install -y gh") {
			t.Errorf("provider %q must install gh in the image:\n%s", prov, s)
		}
	}
	dir := t.TempDir()
	if _, err := WriteContainerfileFor(dir, InitConfig{Language: "go", Provider: "gitea"}, false); err != nil {
		t.Fatalf("WriteContainerfileFor(gitea): %v", err)
	}
	s := string(mustRead(t, filepath.Join(dir, "Containerfile")))
	if strings.Contains(s, "apt-get install -y gh") {
		t.Errorf("gitea must not install gh (REST API, no CLI):\n%s", s)
	}
}

func TestWriteContainerfileFor_OtherKeepsTODOAndDocsPointer(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteContainerfileFor(dir, InitConfig{Language: "other"}, false); err != nil {
		t.Fatalf("WriteContainerfileFor: %v", err)
	}
	s := string(mustRead(t, filepath.Join(dir, "Containerfile")))
	if !strings.Contains(s, "TODO") {
		t.Error("other Containerfile must keep a toolchain TODO")
	}
	if !strings.Contains(s, "docs/configuration-reference.md") {
		t.Error("other Containerfile TODO must point at the reference docs")
	}
	if strings.Contains(s, "go.dev/dl") {
		t.Error("other Containerfile must not contain any per-language example")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
