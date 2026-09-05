package tui

import (
	"bytes"
	"strings"
	"testing"
)

func goNodeForm() InitForm {
	return InitForm{
		Languages: []LangOption{
			{ID: "go", Display: "Go"},
			{ID: "node", Display: "JavaScript / TypeScript"},
		},
		ImageDefault: "themis-demo:latest",
		ProviderDef:  "github",
	}
}

func TestForm_HappyPathDefaults(t *testing.T) {
	// language: accept default (go), image: accept default, provider: accept default (github).
	f := goNodeForm()
	ans, err := f.Run(New(strings.NewReader("\n\n\n"), &bytes.Buffer{}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := InitAnswers{Language: "go", Image: "themis-demo:latest", Provider: "github"}
	if ans != want {
		t.Errorf("got %+v, want %+v", ans, want)
	}
}

func TestForm_ExplicitChoices(t *testing.T) {
	// language 2 (node), custom image, provider 2 (gitea).
	f := goNodeForm()
	ans, err := f.Run(New(strings.NewReader("2\nthemis-mine:latest\n2\n"), &bytes.Buffer{}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := InitAnswers{Language: "node", Image: "themis-mine:latest", Provider: "gitea"}
	if ans != want {
		t.Errorf("got %+v, want %+v", ans, want)
	}
}

func TestForm_OtherSelectsLast(t *testing.T) {
	f := goNodeForm() // options: Go, JavaScript / TypeScript, Other -> index 3
	ans, err := f.Run(New(strings.NewReader("3\n\n\n"), &bytes.Buffer{}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ans.Language != "other" {
		t.Errorf("last option must map to 'other'; got %q", ans.Language)
	}
}

func TestForm_ProviderDefaultGitea(t *testing.T) {
	f := goNodeForm()
	f.ProviderDef = "gitea"
	ans, err := f.Run(New(strings.NewReader("\n\n\n"), &bytes.Buffer{}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ans.Provider != "gitea" {
		t.Errorf("provider default gitea must be pre-selected; got %q", ans.Provider)
	}
}

func TestForm_ImageConflictRequiresOverride(t *testing.T) {
	f := goNodeForm()
	// The default tag "exists"; a second tag does not. The form must not accept
	// the conflicting default — it re-prompts until a free tag is entered.
	f.ImageExists = func(tag string) bool { return tag == "themis-demo:latest" }
	out := &bytes.Buffer{}
	// language default, image: accept default (conflict) then type a free one, provider default.
	ans, err := f.Run(New(strings.NewReader("\n\nthemis-free:latest\n\n"), out))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ans.Image != "themis-free:latest" {
		t.Errorf("form must require a non-conflicting tag; got %q", ans.Image)
	}
	if !strings.Contains(out.String(), "already exists") {
		t.Errorf("expected a conflict warning; got:\n%s", out.String())
	}
}
