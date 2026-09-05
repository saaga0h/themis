package main

import "testing"

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
