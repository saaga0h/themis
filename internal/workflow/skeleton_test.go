package workflow

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeSentinelDescriptor writes a pre-existing, non-skeleton
// .themis/workflow.yaml into dir so non-destructive-write tests can assert it
// was left alone (or deliberately overwritten).
func writeSentinelDescriptor(t *testing.T, dir string) (path string, content []byte) {
	t.Helper()
	themisDir := filepath.Join(dir, ".themis")
	if err := os.MkdirAll(themisDir, 0o755); err != nil {
		t.Fatalf("mkdir .themis: %v", err)
	}
	content = []byte("sentinel: true\n")
	path = filepath.Join(themisDir, "workflow.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write sentinel descriptor: %v", err)
	}
	return path, content
}

// AC2: the scaffolded file parses via the loader with a non-empty stack
// field, a non-empty verify list, and a populated docs block.
func TestWriteSkeleton_ParsesWithPopulatedFieldsViaLoader(t *testing.T) {
	dir := t.TempDir()
	created, err := WriteSkeleton(dir, false)
	if err != nil {
		t.Fatalf("WriteSkeleton: %v", err)
	}
	if !created {
		t.Error("expected created=true when scaffolding into an empty directory")
	}

	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Stack == "" {
		t.Error("scaffolded descriptor must have a non-empty stack field")
	}
	if len(d.Verify) == 0 {
		t.Error("scaffolded descriptor must have a non-empty verify list")
	}
	// Docs contains a slice field (Surfaces), so it is not comparable with ==;
	// "populated" is asserted field-by-field instead — at least one Docs field
	// must be set by the skeleton.
	if d.Docs.Standards == "" && d.Docs.Glossary == "" && d.Docs.Architecture == "" && len(d.Docs.Surfaces) == 0 {
		t.Error("scaffolded descriptor must populate at least one docs field")
	}
}

// AC2: no hardcoded language assumption — the live stack: value must not be a
// literal known language name, and the skeleton must still demonstrate
// example language values via comments (so the field isn't merely empty of
// guidance, just empty of a hardcoded choice).
func TestWriteSkeleton_NoHardcodedLanguageOutsideComments(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteSkeleton(dir, false); err != nil {
		t.Fatalf("WriteSkeleton: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".themis", "workflow.yaml"))
	if err != nil {
		t.Fatalf("reading scaffolded file: %v", err)
	}

	knownLanguages := []string{
		"go", "node", "python", "rust", "java",
		"ruby", "javascript", "typescript", "c++",
	}

	var stackLine string
	var sawLanguageExampleComment bool
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			lower := strings.ToLower(trimmed)
			for _, lang := range knownLanguages {
				if strings.Contains(lower, lang) {
					sawLanguageExampleComment = true
				}
			}
			continue
		}
		if strings.HasPrefix(trimmed, "stack:") {
			stackLine = trimmed
		}
	}

	if stackLine == "" {
		t.Fatal("skeleton has no non-comment stack: line")
	}
	value := strings.ToLower(strings.Trim(strings.TrimSpace(strings.TrimPrefix(stackLine, "stack:")), `"'`))
	for _, lang := range knownLanguages {
		if value == lang {
			t.Errorf("stack: value %q must not be a hardcoded known language name", value)
		}
	}
	if !sawLanguageExampleComment {
		t.Error("skeleton must demonstrate example language values in a comment, not just leave stack: unexplained")
	}
}

// AC3: the default verify command is deliberately unconfigured — running it
// via bash -c must exit non-zero, so an unconfigured project cannot
// green-gate by accident.
func TestWriteSkeleton_DefaultVerifyCommandFailsOnPurpose(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteSkeleton(dir, false); err != nil {
		t.Fatalf("WriteSkeleton: %v", err)
	}
	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(d.Verify) == 0 {
		t.Fatal("skeleton must declare at least one verify command")
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
		t.Error("expected at least one default verify command to fail on purpose (an unconfigured project must not green-gate)")
	}
}

// AC4: without force, an existing workflow.yaml is left byte-for-byte
// unchanged and WriteSkeleton reports it did not create/overwrite anything.
func TestWriteSkeleton_ExistingFileUnchangedWithoutForce(t *testing.T) {
	dir := t.TempDir()
	path, sentinel := writeSentinelDescriptor(t, dir)

	created, err := WriteSkeleton(dir, false)
	if err != nil {
		t.Fatalf("WriteSkeleton: %v", err)
	}
	if created {
		t.Error("expected created=false when a workflow.yaml already exists and force=false")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file after WriteSkeleton: %v", err)
	}
	if !bytes.Equal(got, sentinel) {
		t.Errorf("existing file must be byte-for-byte unchanged without force; got %q, want %q", got, sentinel)
	}
}

// AC4: with force=true, an existing workflow.yaml is overwritten with the
// skeleton content.
func TestWriteSkeleton_ForceOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path, sentinel := writeSentinelDescriptor(t, dir)

	created, err := WriteSkeleton(dir, true)
	if err != nil {
		t.Fatalf("WriteSkeleton: %v", err)
	}
	if !created {
		t.Error("expected created=true when force=true overwrites an existing file")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file after WriteSkeleton: %v", err)
	}
	if bytes.Equal(got, sentinel) {
		t.Fatal("force=true must overwrite the existing sentinel content")
	}

	// Compare against a fresh scaffold in an empty directory to confirm the
	// forced write equals the skeleton content, not merely "different from
	// the sentinel".
	freshDir := t.TempDir()
	if _, err := WriteSkeleton(freshDir, false); err != nil {
		t.Fatalf("WriteSkeleton (fresh): %v", err)
	}
	want, err := os.ReadFile(filepath.Join(freshDir, ".themis", "workflow.yaml"))
	if err != nil {
		t.Fatalf("reading fresh scaffold: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("forced overwrite content must equal fresh skeleton content;\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// #142: the skeleton scaffolds the image: field so a fresh project has the
// launcher config point (loadable — the existing Parses test would fail on a
// bad image: line).
func TestWriteSkeleton_IncludesImageField(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteSkeleton(dir, false); err != nil {
		t.Fatalf("WriteSkeleton: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".themis", "workflow.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), "image:") {
		t.Error("skeleton must scaffold an image: field")
	}
}

// #142: the Containerfile scaffold pre-fills the fixed agent layer and leaves the
// toolchain as a TODO the user fills.
func TestWriteContainerfile_ScaffoldsAgentLayerAndToolchainTODO(t *testing.T) {
	dir := t.TempDir()
	created, err := WriteContainerfile(dir, false)
	if err != nil {
		t.Fatalf("WriteContainerfile: %v", err)
	}
	if !created {
		t.Error("expected created=true scaffolding into an empty dir")
	}
	s, err := os.ReadFile(filepath.Join(dir, "Containerfile"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// themis is built from source in a builder stage (arch-matching), not COPY'd
	// from the build context.
	for _, want := range []string{
		"ARG THEMIS_REPO=https://github.com/saaga0h/themis.git",
		"go build -o /themis ./cmd/themis",
		"COPY --from=themis-build /themis /usr/local/bin/themis",
		"npm install -g @anthropic-ai/claude-code",
		"TODO",
	} {
		if !strings.Contains(string(s), want) {
			t.Errorf("Containerfile missing %q", want)
		}
	}
	if strings.Contains(string(s), "COPY themis /usr/local/bin/themis") {
		t.Error("Containerfile must not COPY a themis binary from the build context (dead cross-platform model)")
	}
}

// #142: .env.example names the tokens but carries no values.
func TestWriteEnvExample_NamesTokensWithoutValues(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteEnvExample(dir, false); err != nil {
		t.Fatalf("WriteEnvExample: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".env.example"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(raw)
	for _, want := range []string{"CLAUDE_CODE_OAUTH_TOKEN=", "GH_TOKEN="} {
		if !strings.Contains(s, want) {
			t.Errorf(".env.example missing %q", want)
		}
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "CLAUDE_CODE_OAUTH_TOKEN=") && strings.TrimPrefix(line, "CLAUDE_CODE_OAUTH_TOKEN=") != "" {
			t.Errorf("token line must carry no value: %q", line)
		}
	}
}

// #142: writeScaffold's non-destructive contract holds for the Containerfile too.
func TestWriteContainerfile_ExistingUnchangedWithoutForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Containerfile")
	if err := os.WriteFile(path, []byte("SENTINEL\n"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	created, err := WriteContainerfile(dir, false)
	if err != nil {
		t.Fatalf("WriteContainerfile: %v", err)
	}
	if created {
		t.Error("expected created=false when a Containerfile already exists")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "SENTINEL\n" {
		t.Errorf("existing Containerfile must be unchanged; got %q", got)
	}
}
