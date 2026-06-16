package profile

// AgentConfig holds per-reviewer model assignments.
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
	Agents        AgentConfig `yaml:"agents"`
	Round3        string      `yaml:"round3"`
	Round3Surfaces []string   `yaml:"round3_surfaces"`
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

// Profile is the per-project pipeline configuration.
type Profile struct {
	Review    ReviewConfig    `yaml:"review"`
	Implement ImplementConfig `yaml:"implement"`
	Refactor  RefactorConfig  `yaml:"refactor"`
	Docs      DocsConfig      `yaml:"docs"`
	Blocking  BlockingConfig  `yaml:"blocking"`
}

// Load reads .themis/profile.yaml from dir. Returns sensible defaults when the
// file does not exist.
func Load(dir string) (*Profile, error) {
	panic("not implemented")
}

// Save writes p to <dir>/.themis/profile.yaml.
func Save(dir string, p *Profile) error {
	panic("not implemented")
}
