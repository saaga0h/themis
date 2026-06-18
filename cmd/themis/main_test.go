package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "themis-test-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(dir)

	binary = filepath.Join(dir, "themis")

	build := exec.Command("go", "build", "-o", binary, "./cmd/themis/")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// themis version prints a version string to stdout and exits 0
func TestVersionPrintsNonEmptyString(t *testing.T) {
	cmd := exec.Command(binary, "version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("themis version must exit 0, got: %v", err)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		t.Error("themis version must print a non-empty version string to stdout")
	}
}

// go.mod exists at repo root with module path git.home.federation.fi/lavernea/themis
func TestGoModModulePath(t *testing.T) {
	content, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("go.mod not found at repo root: %v", err)
	}
	if !strings.Contains(string(content), "module git.home.federation.fi/lavernea/themis") {
		t.Errorf("go.mod must declare module git.home.federation.fi/lavernea/themis\ngot:\n%s", string(content))
	}
}

// Makefile exists with build, test, and lint targets that succeed
func TestMakefileHasGoBuildTarget(t *testing.T) {
	assertMakefileTarget(t, "build")
}

func TestMakefileHasGoTestTarget(t *testing.T) {
	assertMakefileTarget(t, "test")
}

func TestMakefileHasGoLintTarget(t *testing.T) {
	assertMakefileTarget(t, "lint")
}

func assertMakefileTarget(t *testing.T, target string) {
	t.Helper()
	content, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatalf("Makefile not found at repo root: %v", err)
	}
	needle := target + ":"
	if !strings.Contains(string(content), needle) {
		t.Errorf("Makefile must contain target %q", needle)
	}
}
