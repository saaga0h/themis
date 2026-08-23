package workflow

import (
	"os"
	"path/filepath"
)

// skeleton is the scaffolded .themis/workflow.yaml written by `themis init`.
// The stack value and verify command are deliberately unconfigured — a fresh
// project must not green-gate until someone edits this file with real
// commands. Language names appear only in the comment, never as the live
// stack: value, so the factory stays stack-agnostic.
const skeleton = `# .themis/workflow.yaml — this project's pipeline descriptor.
# The factory binary is stack-agnostic: everything stack-specific (the Green
# Gate commands, the authoritative docs) is declared here, not hardcoded.

# stack is an informational label only — it selects no behavior. Examples:
# go, node, python, rust, java, ruby, javascript, typescript, c++.
stack: "unconfigured"

# verify is the Green Gate: shell commands run in order via ` + "`bash -c`" + `;
# each must exit 0 for a run to ship. The command below fails on purpose so
# an unconfigured project cannot green-gate by accident — replace it with
# your project's real build/lint/test commands, e.g.:
#   - go build ./... && go vet ./... && go test ./...
#   - npm ci && npm run build && npm test
#   - pip install -e . && pytest
verify:
  - "echo 'themis: no verify commands configured in .themis/workflow.yaml' && exit 1"

# docs names the authoritative documents agents read during review. Paths are
# relative to the project root; unset fields are skipped.
docs:
  standards: CODING_STANDARDS.md
  glossary: UBIQUITOUS_LANGUAGE.md
  architecture: ""
`

// WriteSkeleton scaffolds .themis/workflow.yaml in dir. Without force, an
// existing file is left byte-for-byte unchanged and created=false is
// returned; with force, the skeleton always overwrites whatever is there.
func WriteSkeleton(dir string, force bool) (created bool, err error) {
	path := filepath.Join(dir, workflowFile)

	if !force {
		if _, statErr := os.Stat(path); statErr == nil {
			return false, nil
		} else if !os.IsNotExist(statErr) {
			return false, statErr
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(skeleton), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
