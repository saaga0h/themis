package profile

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AgentConfig holds per-reviewer model assignments.
// Valid values: "haiku", "sonnet", "opus", "skip".
type AgentConfig struct {
	Security     string `yaml:"security"`
	Architecture string `yaml:"architecture"`
	Complexity   string `yaml:"complexity"`
	Conventions  string `yaml:"conventions"`
	Coverage     string `yaml:"coverage"`
	Numerical    string `yaml:"numerical"`
	Depth        string `yaml:"depth"`
}

// ReviewConfig controls the review step.
type ReviewConfig struct {
	Agents         AgentConfig `yaml:"agents"`
	Round3         string      `yaml:"round3"`
	Round3Surfaces []string    `yaml:"round3_surfaces"`
}

// ImplementConfig controls the implement step.
type ImplementConfig struct {
	Model           string `yaml:"model"`
	TestFixAttempts int    `yaml:"test_fix_attempts"`
}

// RefactorConfig controls the refactor step.
type RefactorConfig struct {
	Enabled bool `yaml:"enabled"`
}

// DocsConfig controls the docs step.
type DocsConfig struct {
	Enabled   bool  `yaml:"enabled"`
	SkipTiers []int `yaml:"skip_tiers"`
}

// BlockingConfig defines what counts as a blocking finding.
type BlockingConfig struct {
	Includes []string `yaml:"includes"`
}

// Profile is the per-project pipeline configuration loaded from .themis/profile.yaml.
type Profile struct {
	Review    ReviewConfig    `yaml:"review"`
	Implement ImplementConfig `yaml:"implement"`
	Refactor  RefactorConfig  `yaml:"refactor"`
	Docs      DocsConfig      `yaml:"docs"`
	Blocking  BlockingConfig  `yaml:"blocking"`
}

const profileFile = ".themis/profile.yaml"

var validModels = map[string]bool{
	"haiku":  true,
	"sonnet": true,
	"opus":   true,
	"skip":   true,
	"":       true,
}

var validRound3Values = map[string]bool{
	"auto":   true,
	"always": true,
	"never":  true,
	"":       true,
}

// Load reads .themis/profile.yaml from dir. Returns sensible defaults when
// the file does not exist.
func Load(dir string) (*Profile, error) {
	path := filepath.Join(dir, profileFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaults(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading profile: %w", err)
	}

	var p Profile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("parsing profile: %w", err)
	}

	if err := validate(&p); err != nil {
		return nil, err
	}

	applyDefaults(&p)
	return &p, nil
}

// Save writes p to <dir>/.themis/profile.yaml, creating directories as needed.
func Save(dir string, p *Profile) error {
	path := filepath.Join(dir, profileFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating profile directory: %w", err)
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshalling profile: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func validate(p *Profile) error {
	agents := map[string]string{
		"security":     p.Review.Agents.Security,
		"architecture": p.Review.Agents.Architecture,
		"complexity":   p.Review.Agents.Complexity,
		"conventions":  p.Review.Agents.Conventions,
		"coverage":     p.Review.Agents.Coverage,
		"numerical":    p.Review.Agents.Numerical,
		"depth":        p.Review.Agents.Depth,
	}
	for name, model := range agents {
		if !validModels[model] {
			return fmt.Errorf("invalid model %q for review agent %q (must be haiku, sonnet, opus, or skip)", model, name)
		}
	}
	if !validRound3Values[p.Review.Round3] {
		return fmt.Errorf("invalid round3 value %q (must be auto, always, or never)", p.Review.Round3)
	}
	return nil
}

func applyDefaults(p *Profile) {
	d := defaults()
	a := &p.Review.Agents
	if a.Security == "" {
		a.Security = d.Review.Agents.Security
	}
	if a.Architecture == "" {
		a.Architecture = d.Review.Agents.Architecture
	}
	if a.Complexity == "" {
		a.Complexity = d.Review.Agents.Complexity
	}
	if a.Conventions == "" {
		a.Conventions = d.Review.Agents.Conventions
	}
	if a.Coverage == "" {
		a.Coverage = d.Review.Agents.Coverage
	}
	if a.Numerical == "" {
		a.Numerical = d.Review.Agents.Numerical
	}
	if a.Depth == "" {
		a.Depth = d.Review.Agents.Depth
	}
	if p.Review.Round3 == "" {
		p.Review.Round3 = d.Review.Round3
	}
	if p.Implement.TestFixAttempts == 0 {
		p.Implement.TestFixAttempts = d.Implement.TestFixAttempts
	}
	if p.Implement.Model == "" {
		p.Implement.Model = d.Implement.Model
	}
}

func defaults() *Profile {
	return &Profile{
		Review: ReviewConfig{
			Agents: AgentConfig{
				Security:     "sonnet",
				Architecture: "sonnet",
				Complexity:   "haiku",
				Conventions:  "haiku",
				Coverage:     "haiku",
				Numerical:    "sonnet",
				Depth:        "sonnet",
			},
			Round3: "auto",
		},
		Implement: ImplementConfig{
			Model:           "sonnet",
			TestFixAttempts: 3,
		},
		Refactor: RefactorConfig{Enabled: true},
		Docs:     DocsConfig{Enabled: true},
		Blocking: BlockingConfig{
			Includes: []string{
				"security",
				"ac-coverage",
				"compile-failure",
				"data-loss",
				"abstraction-boundary",
			},
		},
	}
}
