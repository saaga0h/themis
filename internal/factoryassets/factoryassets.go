// Package factoryassets materializes the factory-internal curated assets — the
// skills and agents named in factory/manifest.txt, plus the factory's Claude Code
// settings — from an embedded filesystem into a target .claude directory, so an
// in-sandbox run has them without any repo mount or the themis repo being present.
//
// It also installs the interactive authoring/review set (skills, commands, and
// the agents those spawn) into a user's .claude via InstallInteractive — the
// cross-platform replacement for the retired install.sh.
//
// The embedded filesystem is supplied by the caller (the module-root themis
// package holds the go:embed; see the note there on why the embed cannot live
// here). Keeping the embed out of this package leaves the copy logic pure and
// testable against any fs.FS.
package factoryassets

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// settingsJSON is the Claude Code settings a factory run uses: no co-authored-by
// trailer, and the bundled skills disabled so only the curated factory set is
// active. Matches what `make factory-cc` wrote before the embed replaced it.
const settingsJSON = `{
  "includeCoAuthoredBy": false,
  "disableBundledSkills": true
}
`

// manifestEntry is one "kind name" line from factory/manifest.txt.
type manifestEntry struct{ kind, name string }

// Materialize writes the manifest'd factory-internal skills and agents from fsys
// into claudeDir, plus a settings.json. Skills are directory trees
// (skills/<name>/…); agents are single files (agents/<name>.md). Only "skill" and
// "agent" entries are materialized — the pipeline delegates to those; commands are
// interactive-only and not part of a run.
func Materialize(fsys fs.FS, claudeDir string) error {
	entries, err := parseManifest(fsys)
	if err != nil {
		return err
	}
	for _, e := range entries {
		switch e.kind {
		case "skill":
			if err := copyTree(fsys, "skills/"+e.name, filepath.Join(claudeDir, "skills", e.name)); err != nil {
				return err
			}
		case "agent":
			if err := copyFile(fsys, "agents/"+e.name+".md", filepath.Join(claudeDir, "agents", e.name+".md")); err != nil {
				return err
			}
		}
	}
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settingsJSON), 0o644)
}

// interactiveSkills are the human's authoring/review skills. skills/ also holds
// factory-internal (test-red, pr-composition) and design add-ons (deepen,
// deepening, design-it-twice), so the interactive set is named explicitly rather
// than "all of skills/".
var interactiveSkills = []string{
	"grill-me", "split-walker", "issue-writer", "contract-drafter",
	"review-walker", "pr-review",
	"diagnose-themis-run", "themis-bug-report",
}

// pipelineOnlyAgents are spawned only by the in-sandbox pipeline, never by an
// interactive command, so they are excluded from the interactive install. All
// other agents (the /review panel, /document's doc-*, pr-review's pr-*) are
// installed so the interactive commands can spawn them.
var pipelineOnlyAgents = map[string]bool{
	"test-architect": true, "test-writer": true, "test-runner": true,
}

// InstallInteractive writes the interactive authoring/review assets from fsys
// into claudeDir: the named interactive skills, every command, and every agent
// except the pipeline-only ones. Used by `themis skills install` to populate a
// user's .claude on any OS.
func InstallInteractive(fsys fs.FS, claudeDir string) error {
	for _, name := range interactiveSkills {
		if err := copyTree(fsys, "skills/"+name, filepath.Join(claudeDir, "skills", name)); err != nil {
			return err
		}
	}
	if err := copyDirFiles(fsys, "commands", filepath.Join(claudeDir, "commands"), nil); err != nil {
		return err
	}
	return copyDirFiles(fsys, "agents", filepath.Join(claudeDir, "agents"), pipelineOnlyAgents)
}

// copyDirFiles copies each *.md file directly under srcDir into dstDir, skipping
// any whose base name (without .md) is in exclude.
func copyDirFiles(fsys fs.FS, srcDir, dstDir string, exclude map[string]bool) error {
	entries, err := fs.ReadDir(fsys, srcDir)
	if err != nil {
		return fmt.Errorf("reading embedded %s: %w", srcDir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if exclude[strings.TrimSuffix(e.Name(), ".md")] {
			continue
		}
		if err := copyFile(fsys, srcDir+"/"+e.Name(), filepath.Join(dstDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func parseManifest(fsys fs.FS) ([]manifestEntry, error) {
	data, err := fs.ReadFile(fsys, "factory/manifest.txt")
	if err != nil {
		return nil, fmt.Errorf("reading factory manifest: %w", err)
	}
	var out []manifestEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 {
			return nil, fmt.Errorf("malformed manifest line: %q", line)
		}
		out = append(out, manifestEntry{kind: f[0], name: f[1]})
	}
	return out, nil
}

func copyFile(fsys fs.FS, src, dst string) error {
	data, err := fs.ReadFile(fsys, src)
	if err != nil {
		return fmt.Errorf("reading embedded %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func copyTree(fsys fs.FS, src, dst string) error {
	return fs.WalkDir(fsys, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, src), "/")
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(fsys, p, target)
	})
}
