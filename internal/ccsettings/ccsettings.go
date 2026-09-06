// Package ccsettings reads a project's Claude Code settings
// (.claude/settings.json) — specifically its top-level "env" block — so the
// factory can honor the same per-project environment that interactive Claude
// Code applies. This unifies telemetry configuration: the OTEL_* vars a user
// puts in {project}/.claude/settings.json drive interactive CC, the sandbox CC,
// and Themis's own OTLP emitter from one file. It only reads; it never writes.
package ccsettings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Env returns the top-level "env" object from {dir}/.claude/settings.json as a
// string map. Contracts:
//   - absent file → empty map, nil error (settings are optional);
//   - malformed JSON → nil map, error (the caller decides whether to warn);
//   - no "env" key, or "env" not an object → empty map, nil error;
//   - object values that are not strings are skipped (CC env values are strings).
func Env(dir string) (map[string]string, error) {
	path := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, err // malformed JSON (or top level is not an object)
	}
	raw, ok := top["env"]
	if !ok {
		return map[string]string{}, nil
	}
	var envRaw map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envRaw); err != nil {
		return map[string]string{}, nil // "env" is present but not an object
	}
	out := make(map[string]string, len(envRaw))
	for k, v := range envRaw {
		var s string
		if json.Unmarshal(v, &s) == nil {
			out[k] = s
		}
	}
	return out, nil
}
