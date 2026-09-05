package tui

import (
	"fmt"
	"strings"
)

// LangOption is a selectable language: ID is the canonical preset id, Display is
// the label shown in the menu.
type LangOption struct {
	ID      string
	Display string
}

// InitForm is the interactive `themis init` questionnaire. It collects answers
// and returns them — it writes no files. The caller composes the config through
// internal/workflow, so the interactive and headless paths cannot diverge.
type InitForm struct {
	// Languages are the known languages, in menu order; the form appends "Other".
	Languages []LangOption
	// ImageDefault pre-fills the image prompt (e.g. themis-<dir>:latest).
	ImageDefault string
	// ProviderDef pre-selects the provider ("github" or "gitea").
	ProviderDef string
	// ImageExists, when non-nil, reports whether a tag already exists; a
	// conflicting default is not pre-accepted — the user must enter another tag.
	ImageExists func(tag string) bool
}

// InitAnswers is what the form collected.
type InitAnswers struct {
	Language string // canonical id, or "other"
	Image    string
	Provider string // "github" or "gitea"
}

// Run walks the questionnaire against p and returns the answers.
func (f InitForm) Run(p *Prompter) (InitAnswers, error) {
	// Language — the one genuine decision; everything else is a confirmable default.
	labels := make([]string, 0, len(f.Languages)+1)
	for _, l := range f.Languages {
		labels = append(labels, l.Display)
	}
	labels = append(labels, "Other")
	idx, err := p.Select("Language", labels, 0)
	if err != nil {
		return InitAnswers{}, err
	}
	lang := "other"
	if idx < len(f.Languages) {
		lang = f.Languages[idx].ID
	}

	// Image — default themis-<dir>:latest; a conflicting tag is not pre-accepted.
	img, err := p.Input("Sandbox image tag", f.ImageDefault)
	if err != nil {
		return InitAnswers{}, err
	}
	for f.ImageExists != nil && strings.TrimSpace(img) != "" && f.ImageExists(img) {
		fmt.Fprintf(p.out, "image %q already exists — choose another tag\n", img)
		img, err = p.Input("Sandbox image tag", "")
		if err != nil {
			return InitAnswers{}, err
		}
	}

	// Provider — pre-selected from the git remote.
	provOpts := []string{"github", "gitea"}
	provDef := 0
	if f.ProviderDef == "gitea" {
		provDef = 1
	}
	pi, err := p.Select("Provider (issue tracker / forge)", provOpts, provDef)
	if err != nil {
		return InitAnswers{}, err
	}

	return InitAnswers{Language: lang, Image: img, Provider: provOpts[pi]}, nil
}
