package main

import "testing"

func TestApplyProjectEnv_ExistingWins(t *testing.T) {
	existing := map[string]string{"ALREADY_SET": "launch-value"}
	set := map[string]string{}
	lookup := func(k string) (string, bool) { v, ok := existing[k]; return v, ok }
	setter := func(k, v string) error { set[k] = v; return nil }

	applyProjectEnv(map[string]string{
		"ALREADY_SET":                      "settings-value",        // must NOT override
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": "http://localhost:9428", // must be set
	}, lookup, setter)

	if _, overrode := set["ALREADY_SET"]; overrode {
		t.Error("an already-set var must not be overridden by settings.json")
	}
	if got := set["OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"]; got != "http://localhost:9428" {
		t.Errorf("a settings-only var must be set; got %q", got)
	}
}
