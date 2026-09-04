// Package workflow loads the per-project pipeline descriptor
// (.themis/workflow.yaml). The descriptor is how a project tells the factory how
// its stuff flows — the factory binary stays stack-agnostic and reads everything
// stack-specific (verify commands, authoritative docs) from here.
//
// Slice 1 consumes two sections: `verify` (the Green Gate commands) and `docs`
// (the authoritative standards/glossary/architecture the agents read). Later
// slices add conventions, stage toggles, and model tiers; unknown sections are
// rejected (strict decode) so the schema and the loader stay in lockstep.
package workflow

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Docs names the authoritative documents the agents read. Paths are relative to
// the project root. Empty fields are skipped.
type Docs struct {
	Standards    string `yaml:"standards"`
	Glossary     string `yaml:"glossary"`
	Architecture string `yaml:"architecture"`
	// Surfaces are path globs (relative to the project root) that, when a changed
	// file matches one, trigger the Docs step. When empty, Docs always runs; when
	// set, the Docs step is skipped for changes that touch no documented surface.
	Surfaces []string `yaml:"surfaces"`
}

// Descriptor is the per-project pipeline configuration loaded from
// .themis/workflow.yaml.
type Descriptor struct {
	// Stack is an informational label only (e.g. "go", "node", "julia").
	Stack string `yaml:"stack"`
	// Verify is the Green Gate: shell commands run in order; each must exit 0.
	Verify []string `yaml:"verify"`
	// Docs are the authoritative inputs the agents read.
	Docs Docs `yaml:"docs"`
	// FootprintExempt lists paths a footprint gate always allows regardless of the
	// issue's declared packages — generated/lockfiles a legitimate change touches
	// outside its own packages (e.g. go.mod, go.sum). Declared per-repo to keep the
	// factory stack-agnostic. See issue #111.
	FootprintExempt []string `yaml:"footprint_exempt"`
	// Image is the sandbox container image the host-mode launcher runs the factory
	// in — built locally from the project's Containerfile. Required for a launch
	// (the launcher errors when it is empty). See cmd/themis launcher (#140).
	Image string `yaml:"image"`
	// Runtime optionally pins the container runtime for the launcher: "podman" or
	// "docker". Empty means autodetect (podman, then docker).
	Runtime string `yaml:"runtime"`
}

const workflowFile = ".themis/workflow.yaml"

// Load reads .themis/workflow.yaml from dir. A missing file yields an empty
// descriptor (no verify commands, no docs) rather than an error — a project
// without a descriptor simply declares nothing, and callers decide how to treat
// that (the Green Gate, for instance, warns and no-ops).
func Load(dir string) (*Descriptor, error) {
	path := filepath.Join(dir, workflowFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Descriptor{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading workflow descriptor: %w", err)
	}

	var d Descriptor
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("parsing workflow descriptor: %w", err)
	}

	for i, cmd := range d.Verify {
		if cmd == "" {
			return nil, fmt.Errorf("workflow descriptor: verify[%d] is empty", i)
		}
	}
	if d.Runtime != "" && d.Runtime != "podman" && d.Runtime != "docker" {
		return nil, fmt.Errorf("workflow descriptor: runtime %q must be \"podman\", \"docker\", or empty (autodetect)", d.Runtime)
	}
	return &d, nil
}

// StandardsDocs returns the declared authoritative doc paths, in reading order,
// skipping any that are unset.
func (d *Descriptor) StandardsDocs() []string {
	var out []string
	for _, p := range []string{d.Docs.Standards, d.Docs.Glossary, d.Docs.Architecture} {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
