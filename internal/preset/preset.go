// Package preset holds the per-language scaffolding data used by `themis init`:
// for each default language, the Green Gate `verify:` commands and the
// Containerfile toolchain stanza that installs the tools those commands call.
//
// It is the single source of truth the headless init path and the interactive
// TUI both read, replacing per-language snippets that would otherwise drift
// across the docs and the scaffold templates. It is pure data — no file
// writing, no template composition, no CLI, no TUI (those consume it).
//
// Adding a future default language is a single new entry in the table below:
// Languages is derived from the table keys, so there is no parallel list to
// keep in sync.
package preset

import "sort"

// Preset is the language-specific fill for a scaffolded project.
type Preset struct {
	Stack     string   // value written to workflow.yaml `stack:` (informational)
	Display   string   // label shown in the init picker
	Verify    []string // the verify: commands
	Toolchain string   // the Containerfile RUN stanza(s) that install the toolchain
}

// table maps a canonical language id to its preset. The keys are the canonical
// ids; Get and Languages derive everything from this one map.
var table = map[string]Preset{
	"go": {
		Stack:   "go",
		Display: "Go",
		Verify: []string{
			"go build ./...",
			`test -z "$(gofmt -l .)"`,
			"go vet ./...",
			"go test ./...",
		},
		Toolchain: `ARG GO_VERSION=1.25.0
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz \
      | tar -C /usr/local -xz \
    && ln -s /usr/local/go/bin/go /usr/local/bin/go`,
	},
	"python": {
		Stack:   "python",
		Display: "Python",
		Verify:  []string{"ruff check .", "pytest"},
		Toolchain: `RUN apt-get update && apt-get install -y python3 python3-pip python3-venv \
    && rm -rf /var/lib/apt/lists/*
RUN pip3 install --no-cache-dir --break-system-packages pytest ruff`,
	},
	"node": {
		Stack:     "node",
		Display:   "JavaScript / TypeScript",
		Verify:    []string{"npm ci", "npm run build", "npm test"},
		Toolchain: `RUN npm install -g typescript`,
	},
	"rust": {
		Stack:   "rust",
		Display: "Rust",
		Verify:  []string{"cargo build", "cargo test"},
		Toolchain: `RUN apt-get update && apt-get install -y cargo rustc \
    && rm -rf /var/lib/apt/lists/*`,
	},
}

// Get returns the preset for a canonical language id. ok is false for an unknown
// id or "other" — the caller maps that to the neutral skeleton. The returned
// Verify slice is a copy, so callers cannot mutate the shared table.
func Get(id string) (Preset, bool) {
	p, ok := table[id]
	if !ok {
		return Preset{}, false
	}
	v := make([]string, len(p.Verify))
	copy(v, p.Verify)
	p.Verify = v
	return p, true
}

// Languages returns the canonical ids the table defines, in stable (sorted)
// order. It is derived from the table keys, so a new table entry surfaces here
// and in Get with no other change.
func Languages() []string {
	ids := make([]string, 0, len(table))
	for id := range table {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
