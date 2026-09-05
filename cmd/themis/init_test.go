package main

import "testing"

func TestDecideInitMode(t *testing.T) {
	cases := []struct {
		contentSet, tty bool
		want            initMode
	}{
		{true, true, modeHeadless},
		{true, false, modeHeadless},
		{false, true, modeInteractive},
		{false, false, modeErrNoTTY},
	}
	for _, c := range cases {
		if got := decideInitMode(c.contentSet, c.tty); got != c.want {
			t.Errorf("decideInitMode(content=%v, tty=%v) = %d, want %d", c.contentSet, c.tty, got, c.want)
		}
	}
}

func TestDefaultImageTag(t *testing.T) {
	cases := []struct {
		workDir string
		want    string
	}{
		{"/home/me/MyProj", "themis-myproj:latest"},
		{"/home/me/bar baz", "themis-bar-baz:latest"},
		{"/home/me/already-lower", "themis-already-lower:latest"},
	}
	for _, c := range cases {
		if got := defaultImageTag(c.workDir); got != c.want {
			t.Errorf("defaultImageTag(%q) = %q, want %q", c.workDir, got, c.want)
		}
	}
}
