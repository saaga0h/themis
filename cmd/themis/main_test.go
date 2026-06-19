package main_test

import (
	"io/fs"
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

// go.mod declares module github.com/saaga0h/themis (AC1)
func TestGoModDeclaredPath(t *testing.T) {
	content, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("go.mod not found at repo root: %v", err)
	}
	const want = "module github.com/saaga0h/themis"
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "module ") {
			got := strings.TrimSpace(line)
			if got != want {
				t.Errorf("go.mod module directive: got %q, want %q", got, want)
			}
			return
		}
	}
	t.Errorf("go.mod has no module directive")
}

// No .go file contains the old private Gitea hostname in any import path (AC2)
func TestNoOldHostnameInGoFiles(t *testing.T) {
	// split to avoid matching this source file when the tests themselves run
	const oldHostname = "git.home.federation" + ".fi"
	root, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("cannot resolve repo root: %v", err)
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden and vendor directories
			name := d.Name()
			if name == ".git" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("cannot read file %s: %v", path, readErr)
			return nil
		}
		if strings.Contains(string(content), oldHostname) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("file %s still contains old hostname %q", rel, oldHostname)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("error walking repo: %v", err)
	}
}
