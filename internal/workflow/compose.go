package workflow

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/saaga0h/themis/internal/preset"
	"gopkg.in/yaml.v3"
)

// InitConfig is the resolved input for scaffolding a project's config. It is
// produced by `themis init` from flags (or the interactive form) and consumed by
// the writers below.
type InitConfig struct {
	// Language is a canonical preset id (go/python/node/rust) or "other"/unknown.
	// A known id composes a working config; anything else yields the neutral
	// skeleton (deliberately-failing verify + toolchain TODO).
	Language string
	// Stack overrides the workflow.yaml stack: label. Empty defaults to the
	// preset's stack for a known language, or "unconfigured" otherwise.
	Stack string
	// Image is the sandbox image tag written to image:. Empty renders image: "".
	Image string
	// Provider, when set (github|gitea), is written uncommented; empty leaves the
	// provider line commented so auto-detection wins.
	Provider string
	// Runtime, when set (podman|docker), is written uncommented; empty leaves the
	// runtime line commented so the launcher autodetects.
	Runtime string
}

// neutralVerify is the deliberately-failing Green Gate command for an
// unconfigured project — it exits non-zero so a project that never configured
// verify cannot green-gate by accident.
const neutralVerify = "echo 'themis: no verify commands configured in .themis/workflow.yaml' && exit 1"

// neutralConfig is the InitConfig for the "other"/unconfigured path.
func neutralConfig() InitConfig { return InitConfig{Language: "other"} }

// WriteWorkflow scaffolds .themis/workflow.yaml for cfg (force/skip semantics as
// writeScaffold). A known language composes a working descriptor from its preset;
// "other"/unknown writes the neutral skeleton.
func WriteWorkflow(dir string, cfg InitConfig, force bool) (created bool, err error) {
	content, err := renderWorkflow(cfg)
	if err != nil {
		return false, err
	}
	return writeScaffold(filepath.Join(dir, workflowFile), content, force)
}

// WriteContainerfileFor scaffolds a Containerfile for cfg. A known language
// substitutes its preset toolchain stanza; "other"/unknown leaves a toolchain
// TODO that points at the reference docs.
func WriteContainerfileFor(dir string, cfg InitConfig, force bool) (created bool, err error) {
	content, err := renderContainerfile(cfg)
	if err != nil {
		return false, err
	}
	return writeScaffold(filepath.Join(dir, "Containerfile"), content, force)
}

// workflowView is the rendering data for workflowTmpl.
type workflowView struct {
	Stack          string
	VerifyBlock    string
	FootprintBlock string
	ProviderLine   string
	Image          string
	RuntimeLine    string
}

func renderWorkflow(cfg InitConfig) (string, error) {
	p, known := preset.Get(cfg.Language)

	stack := cfg.Stack
	verify := []string{neutralVerify}
	var exempt []string
	if known {
		verify = p.Verify
		exempt = p.FootprintExempt
		if stack == "" {
			stack = p.Stack
		}
	} else if stack == "" {
		stack = "unconfigured"
	}

	var vb strings.Builder
	vb.WriteString("verify:")
	for _, cmd := range verify {
		vb.WriteString("\n  - " + yamlScalar(cmd))
	}

	footprintBlock := "# footprint_exempt:   # e.g. your manifest + lockfile"
	if len(exempt) > 0 {
		var fb strings.Builder
		fb.WriteString("footprint_exempt:")
		for _, e := range exempt {
			fb.WriteString("\n  - " + yamlScalar(e))
		}
		footprintBlock = fb.String()
	}

	providerLine := "# provider: github   # github | gitea"
	if cfg.Provider != "" {
		providerLine = "provider: " + cfg.Provider + "   # github | gitea"
	}
	runtimeLine := "# runtime: podman"
	if cfg.Runtime != "" {
		runtimeLine = "runtime: " + cfg.Runtime
	}

	return mustRender(workflowTmpl, workflowView{
		Stack:          stack,
		VerifyBlock:    vb.String(),
		FootprintBlock: footprintBlock,
		ProviderLine:   providerLine,
		Image:          cfg.Image,
		RuntimeLine:    runtimeLine,
	})
}

// containerfileView is the rendering data for containerfileTmpl.
type containerfileView struct {
	ToolchainBlock string
}

func renderContainerfile(cfg InitConfig) (string, error) {
	p, known := preset.Get(cfg.Language)
	block := toolchainTODO
	if known {
		block = fmt.Sprintf(toolchainGenerated, cfg.Language, p.Toolchain)
	}
	return mustRender(containerfileTmpl, containerfileView{ToolchainBlock: block})
}

// yamlScalar renders s as a single valid YAML scalar (quoted only as needed), so
// verify commands containing quotes or shell metacharacters round-trip through
// the loader unchanged.
func yamlScalar(s string) string {
	out, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Sprintf("%q", s)
	}
	return strings.TrimRight(string(out), "\n")
}

func mustRender(tmplText string, data any) (string, error) {
	t, err := template.New("scaffold").Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("parsing scaffold template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering scaffold template: %w", err)
	}
	return buf.String(), nil
}
