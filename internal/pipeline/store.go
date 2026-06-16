package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const stateFile = ".themis/state.json"

// SaveState writes state to <dir>/.themis/state.json, creating the directory as needed.
func SaveState(dir string, state *PipelineState) error {
	statePath := filepath.Join(dir, stateFile)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}
	return os.WriteFile(statePath, data, 0o600)
}

// LoadState reads state from <dir>/.themis/state.json.
// Returns nil, nil when the file does not exist — start fresh.
func LoadState(dir string) (*PipelineState, error) {
	statePath := filepath.Join(dir, stateFile)
	data, err := os.ReadFile(statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading state file: %w", err)
	}
	var state PipelineState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}
	return &state, nil
}
