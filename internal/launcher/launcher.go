// Package launcher decides how a factory command (themis run / themis issue)
// executes: in-process (already inside the sandbox), by launching the sandbox
// container, or — when on the host with no sandbox available — by refusing.
//
// The safety invariant is the whole point of this package: a factory command
// must NEVER run the pipeline in-process on the host, because the pipeline drives
// Claude Code with --dangerously-skip-permissions. The container is the blast
// radius; without it, refuse. Decide encodes that invariant; BuildRunArgs builds
// the container invocation. Both are pure and exhaustively tested; the actual
// exec lives in cmd/themis.
package launcher

// Mode is how a factory command should execute.
type Mode int

const (
	// InProcess runs the pipeline in the current process. Returned ONLY when the
	// process is already inside the sandbox.
	InProcess Mode = iota
	// Launch runs the pipeline by starting the sandbox container.
	Launch
	// Refuse declines to run at all — on the host with no sandbox available. The
	// pipeline must not run unsandboxed, so this is a hard stop, never a fallback.
	Refuse
)

func (m Mode) String() string {
	switch m {
	case InProcess:
		return "in-process"
	case Launch:
		return "launch"
	default:
		return "refuse"
	}
}

// Decide returns how a factory command should execute.
//
//   - inSandbox:   the THEMIS_IN_SANDBOX marker is present (we are already sandboxed).
//   - sandboxable: a sandbox can be launched (image configured + built, runtime available).
//
// Invariant: Decide returns InProcess if and only if inSandbox is true. On the
// host (inSandbox false) it is Launch when sandboxable, otherwise Refuse — never
// InProcess. This is the safety boundary; do not add a host in-process path.
func Decide(inSandbox, sandboxable bool) Mode {
	if inSandbox {
		return InProcess
	}
	if sandboxable {
		return Launch
	}
	return Refuse
}

// RunSpec is the resolved input for the container invocation.
type RunSpec struct {
	Runtime string   // "podman" or "docker"
	Image   string   // the sandbox image tag
	WorkDir string   // absolute host path of the repo, bind-mounted into the sandbox
	EnvFile string   // path to an env file to pass (e.g. ".env"); empty to omit
	Args    []string // the themis subcommand + args to run inside (e.g. ["run","--provider","gitea"])
}

const (
	// sandboxMount is where the repo is bind-mounted inside the sandbox.
	sandboxMount = "/workspace"
	// SandboxMarker is the env var that tells an in-container themis it is already
	// sandboxed (so it runs in-process instead of trying to launch again). The
	// launcher sets it; the scaffolded Containerfile also bakes it in.
	SandboxMarker = "THEMIS_IN_SANDBOX"
)

// BuildRunArgs builds the argv for `<runtime> run …` that launches the sandbox and
// re-invokes themis inside it with the marker set. It mounts the repo at
// /workspace, sets that as the working dir, passes the env file when given, and —
// for podman — maps the host user with --userns=keep-id so the bind-mount stays
// writable. The image runs `themis <Args>`; inside, the marker makes themis run
// the pipeline in-process.
func BuildRunArgs(s RunSpec) []string {
	args := []string{"run", "--rm", "-i"}
	if s.Runtime == "podman" {
		args = append(args, "--userns=keep-id")
	}
	args = append(args, "-v", s.WorkDir+":"+sandboxMount, "-w", sandboxMount)
	if s.EnvFile != "" {
		args = append(args, "--env-file", s.EnvFile)
	}
	args = append(args, "-e", SandboxMarker+"=1", s.Image, "themis")
	args = append(args, s.Args...)
	return args
}
