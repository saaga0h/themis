package ccsettings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeSettings(t *testing.T, dir, content string) {
	t.Helper()
	cd := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cd, "settings.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestEnv_ReadsEnvBlock(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{
	  "includeCoAuthoredBy": false,
	  "env": {
	    "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": "http://localhost:9428/insert/opentelemetry/v1/logs",
	    "CLAUDE_CODE_ENABLE_TELEMETRY": "1"
	  }
	}`)
	got, err := Env(dir)
	if err != nil {
		t.Fatalf("Env: %v", err)
	}
	want := map[string]string{
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": "http://localhost:9428/insert/opentelemetry/v1/logs",
		"CLAUDE_CODE_ENABLE_TELEMETRY":     "1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Env = %v, want %v", got, want)
	}
}

func TestEnv_AbsentFileIsEmptyNilError(t *testing.T) {
	got, err := Env(t.TempDir()) // no .claude/settings.json
	if err != nil {
		t.Fatalf("absent file must not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("absent file must yield empty map, got %v", got)
	}
}

func TestEnv_NoEnvKeyIsEmpty(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{"includeCoAuthoredBy": false}`)
	got, err := Env(dir)
	if err != nil || len(got) != 0 {
		t.Errorf("no env key → empty, nil; got %v, %v", got, err)
	}
}

func TestEnv_EnvNotObjectIsEmpty(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{"env": "nope"}`)
	got, err := Env(dir)
	if err != nil {
		t.Errorf("env-not-object must not error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("env-not-object → empty, got %v", got)
	}
}

func TestEnv_MalformedJSONErrors(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{ this is not json `)
	if _, err := Env(dir); err == nil {
		t.Error("malformed JSON must return an error")
	}
}

func TestEnv_SkipsNonStringValues(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{"env": {"A": "1", "B": 2, "C": "3"}}`)
	got, _ := Env(dir)
	want := map[string]string{"A": "1", "C": "3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("non-string values must be skipped; got %v, want %v", got, want)
	}
}
