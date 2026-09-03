package runner

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// inlineCode matches backtick-delimited inline code spans. Asset names inside
// inline code are prose cross-references (e.g. "flagged by `ac-drafter`"), not
// delegations, so they are stripped before matching.
var inlineCode = regexp.MustCompile("`[^`]*`")

// repoRoot is the repository root relative to this test's package directory
// (internal/runner). The factory asset manifest and the skills/agents/commands
// trees live at the repo root.
const repoRoot = "../.."

// manifestAsset is one entry from factory/manifest.txt.
type manifestAsset struct {
	kind string // "skill" | "agent" | "command"
	name string
}

// parseManifest reads factory/manifest.txt, skipping blank and comment lines.
func parseManifest(t *testing.T) []manifestAsset {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, "factory", "manifest.txt"))
	if err != nil {
		t.Fatalf("reading factory/manifest.txt: %v", err)
	}
	var assets []manifestAsset
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("malformed manifest line (want '<kind> <name>'): %q", line)
		}
		assets = append(assets, manifestAsset{kind: fields[0], name: fields[1]})
	}
	return assets
}

// assetPath returns the on-disk path of an asset relative to the repo root.
func assetPath(a manifestAsset) string {
	switch a.kind {
	case "skill":
		return filepath.Join(repoRoot, "skills", a.name, "SKILL.md")
	case "agent":
		return filepath.Join(repoRoot, "agents", a.name+".md")
	case "command":
		return filepath.Join(repoRoot, "commands", a.name+".md")
	default:
		return ""
	}
}

// TestManifestEntriesExist guards against typos and stale entries: every asset
// named in factory/manifest.txt must exist in the repo's tracked source tree.
func TestManifestEntriesExist(t *testing.T) {
	for _, a := range parseManifest(t) {
		path := assetPath(a)
		if path == "" {
			t.Errorf("manifest entry has unknown kind %q (name %q)", a.kind, a.name)
			continue
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("manifest lists %s %q but %s is missing: %v", a.kind, a.name, path, err)
		}
	}
}

// templateFiles returns the templates the Go runner renders. This is the
// surface where the pipeline invokes commands and skills directly.
func templateFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot, "templates", "*.md"))
	if err != nil {
		t.Fatalf("globbing templates: %v", err)
	}
	return files
}

// agentSurfaceFiles returns the full delegation surface for subagents: the
// templates plus the body of every manifest-listed asset, so transitive
// delegations are covered — a skill or command can fan out to agents
// (test-red -> test-architect/test-writer, review -> the reviewers), and an
// agent can delegate to another (test-writer -> test-runner).
func agentSurfaceFiles(t *testing.T, manifest []manifestAsset) []string {
	t.Helper()
	files := templateFiles(t)
	for _, a := range manifest {
		if p := assetPath(a); p != "" {
			files = append(files, p)
		}
	}
	return files
}

// readAll concatenates the contents of the given files, stripping inline-code
// spans so backticked prose mentions are not mistaken for delegations.
func readAll(t *testing.T, files []string) string {
	t.Helper()
	var sb strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading surface file %s: %v", f, err)
		}
		sb.Write(inlineCode.ReplaceAll(data, []byte(" ")))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// basenamesWithoutExt lists the names of *.md files (or directories, for skills)
// in dir, stripping the .md extension.
func assetNames(t *testing.T, dir string, isDir bool) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot, dir))
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		switch {
		case isDir && e.IsDir():
			names = append(names, e.Name())
		case !isDir && !e.IsDir() && strings.HasSuffix(e.Name(), ".md"):
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return names
}

// TestManifestCoversPipelineDelegations is the anti-starvation guard: if the
// pipeline's delegation surface references a skill, agent, or command that the
// repo provides, that asset MUST be in factory/manifest.txt — otherwise the
// curated container .claude/ would not expose it and the step would fail to
// delegate. Assets that the surface never references are correctly omitted.
func TestManifestCoversPipelineDelegations(t *testing.T) {
	manifest := parseManifest(t)
	inManifest := func(kind, name string) bool {
		for _, a := range manifest {
			if a.kind == kind && a.name == name {
				return true
			}
		}
		return false
	}

	// Subagents can be reached transitively (a skill or command fans out to
	// them), so they are matched against the full delegation surface. Names are
	// distinctive and hyphenated, so a substring match is reliable.
	agentSurface := readAll(t, agentSurfaceFiles(t, manifest))
	for _, name := range assetNames(t, "agents", false) {
		if strings.Contains(agentSurface, name) && !inManifest("agent", name) {
			t.Errorf("pipeline surface delegates to agent %q but it is not in factory/manifest.txt", name)
		}
	}

	// Skills and commands are invoked directly by the rendered templates, never
	// transitively, so they are matched against the templates only — a prose
	// cross-reference to /implement inside an agent body is not an invocation.
	templateSurface := readAll(t, templateFiles(t))
	for _, name := range assetNames(t, "skills", true) {
		if strings.Contains(templateSurface, name) && !inManifest("skill", name) {
			t.Errorf("a template invokes skill %q but it is not in factory/manifest.txt", name)
		}
	}
	for _, name := range assetNames(t, "commands", false) {
		if strings.Contains(templateSurface, "/"+name) && !inManifest("command", name) {
			t.Errorf("a template invokes command /%s but it is not in factory/manifest.txt", name)
		}
	}
}
