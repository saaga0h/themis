package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/workflow"
)

func TestResolveRuntimeWith(t *testing.T) {
	has := func(names ...string) func(string) (string, error) {
		set := map[string]bool{}
		for _, n := range names {
			set[n] = true
		}
		return func(name string) (string, error) {
			if set[name] {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not found")
		}
	}
	cases := []struct {
		name    string
		pinned  string
		lookHas []string
		want    string
	}{
		{"pinned podman present", "podman", []string{"podman", "docker"}, "podman"},
		{"pinned docker absent → empty (no guessing)", "docker", []string{"podman"}, ""},
		{"autodetect prefers podman", "", []string{"podman", "docker"}, "podman"},
		{"autodetect falls back to docker", "", []string{"docker"}, "docker"},
		{"none available → empty", "", []string{}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveRuntimeWith(c.pinned, has(c.lookHas...)); got != c.want {
				t.Errorf("resolveRuntimeWith(%q) = %q, want %q", c.pinned, got, c.want)
			}
		})
	}
}

// The hardening from the zero-context audit: the marker must equal "1" AND a
// container must be corroborated. A spoofed marker on the host (no container) is
// defeated; non-"1" values don't count.
func TestSandboxDetected(t *testing.T) {
	cases := []struct {
		name      string
		marker    string
		container bool
		want      bool
	}{
		{"genuine sandbox", "1", true, true},
		{"host spoof: marker set but not in a container", "1", false, false},
		{"specific value: 0 is not 1", "0", true, false},
		{"specific value: false is not 1", "false", true, false},
		{"unset", "", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sandboxDetected(
				func(string) string { return c.marker },
				func() bool { return c.container },
			)
			if got != c.want {
				t.Errorf("sandboxDetected(marker=%q, container=%v) = %v, want %v", c.marker, c.container, got, c.want)
			}
		})
	}
}

func TestSandboxRefusal_ExplainsCauseAndNeverFallsBack(t *testing.T) {
	cases := []struct {
		name    string
		desc    *workflow.Descriptor
		runtime string
		want    string // substring that identifies the cause
	}{
		{"no image", &workflow.Descriptor{Image: ""}, "podman", "no image set"},
		{"no runtime", &workflow.Descriptor{Image: "x:latest"}, "", "no container runtime"},
		{"image not built", &workflow.Descriptor{Image: "x:latest"}, "podman", "not built"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := sandboxRefusal(c.desc, c.runtime)
			if err == nil {
				t.Fatal("expected a refusal error, got nil")
			}
			msg := err.Error()
			if !strings.Contains(msg, c.want) {
				t.Errorf("refusal must name the cause %q; got: %s", c.want, msg)
			}
			// Every refusal must make clear it will not run unsandboxed.
			if !strings.Contains(msg, "unsandboxed") {
				t.Errorf("refusal must state it will not run unsandboxed; got: %s", msg)
			}
		})
	}
}
