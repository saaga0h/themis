package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/saaga0h/themis/internal/launcher"
	"github.com/saaga0h/themis/internal/workflow"
)

// maybeLaunchSandbox enforces the sandbox boundary for factory commands
// (run/issue). On the host it launches the sandbox container, or refuses when it
// cannot; inside the sandbox it returns handled=false so the caller runs the
// pipeline in-process. innerArgs is the themis subcommand + args to run inside
// (e.g. ["run", "--provider", "gitea"]).
//
// It must never let the pipeline run in-process on the host — see internal/launcher.
func maybeLaunchSandbox(workDir string, desc *workflow.Descriptor, innerArgs []string) (handled bool, err error) {
	inSandbox := sandboxDetected(os.Getenv, inContainer)
	runtime := resolveRuntime(desc.Runtime)
	sandboxable := desc.Image != "" && runtime != "" && imageExistsIn(runtime, desc.Image)

	switch launcher.Decide(inSandbox, sandboxable) {
	case launcher.InProcess:
		// Loud, never silent: if the pipeline runs in-process it is because we
		// confirmed the sandbox (marker AND container). This line is the audit trail.
		fmt.Fprintln(os.Stderr, "themis: sandbox confirmed (marker + container) — running the pipeline in-process")
		return false, nil
	case launcher.Launch:
		return true, execSandbox(runtime, desc.Image, workDir, innerArgs)
	default: // Refuse
		return true, sandboxRefusal(desc, runtime)
	}
}

// sandboxDetected reports whether the process is genuinely inside the themis
// sandbox. Two independent conditions, both required:
//   - the marker equals exactly "1" (not merely non-empty — "0"/"false"/stray
//     values do NOT count, closing the accidental-inheritance surface), AND
//   - a container is corroborated independently of the marker.
//
// The marker alone is host-settable (spoofable, and easy to inherit from a shell
// profile or CI), so it is never trusted by itself — see the zero-context audit.
// getenv/inContainer are injected for testing.
func sandboxDetected(getenv func(string) string, inContainer func() bool) bool {
	return getenv(launcher.SandboxMarker) == "1" && inContainer()
}

// inContainer corroborates that we are running inside a container, independently
// of the (host-settable) marker: the runtime-created sentinels first (podman:
// /run/.containerenv, docker: /.dockerenv), then a cgroup fallback. On a host
// (no such file, init cgroup) it returns false — so a spoofed marker on the host
// cannot reach an in-process run.
func inContainer() bool {
	for _, p := range []string{"/run/.containerenv", "/.dockerenv"} {
		if fileExists(p) {
			return true
		}
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(data)
		for _, needle := range []string{"docker", "podman", "libpod", "containerd", "kubepods"} {
			if strings.Contains(s, needle) {
				return true
			}
		}
	}
	return false
}

// resolveRuntime returns the container runtime: the pinned one if set and present,
// else podman, else docker, else "" (none available).
func resolveRuntime(pinned string) string { return resolveRuntimeWith(pinned, exec.LookPath) }

func resolveRuntimeWith(pinned string, lookPath func(string) (string, error)) string {
	if pinned != "" {
		if _, err := lookPath(pinned); err == nil {
			return pinned
		}
		return "" // pinned but not installed → not sandboxable (refuse, don't guess another)
	}
	for _, rt := range []string{"podman", "docker"} {
		if _, err := lookPath(rt); err == nil {
			return rt
		}
	}
	return ""
}

// imageExistsIn reports whether the image tag exists locally for the runtime.
func imageExistsIn(runtime, tag string) bool {
	switch runtime {
	case "podman":
		return exec.Command("podman", "image", "exists", tag).Run() == nil
	case "docker":
		return exec.Command("docker", "image", "inspect", tag).Run() == nil
	}
	return false
}

// execSandbox launches the sandbox container and runs themis inside it, streaming
// I/O and propagating the exit status.
func execSandbox(runtime, image, workDir string, innerArgs []string) error {
	envFile := ""
	if p := filepath.Join(workDir, ".env"); fileExists(p) {
		envFile = p
	}
	argv := launcher.BuildRunArgs(launcher.RunSpec{
		Runtime: runtime, Image: image, WorkDir: workDir, EnvFile: envFile, Args: innerArgs,
	})
	fmt.Fprintf(os.Stderr, "themis: launching sandbox (%s, image %s)\n", runtime, image)
	cmd := exec.Command(runtime, argv...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// sandboxRefusal is the hard stop when the factory is on the host but cannot be
// sandboxed. It explains the cause and how to fix it, and NEVER runs unsandboxed.
func sandboxRefusal(desc *workflow.Descriptor, runtime string) error {
	const tail = "The factory only runs inside its sandbox container; it will not run unsandboxed."
	switch {
	case desc.Image == "":
		return fmt.Errorf("refusing to run the factory: no image set in .themis/workflow.yaml — set image: to your built sandbox tag (see docs/getting-started). %s", tail)
	case runtime == "":
		return fmt.Errorf("refusing to run the factory: no container runtime found — install podman or docker (or set runtime: in .themis/workflow.yaml). %s", tail)
	default:
		return fmt.Errorf("refusing to run the factory: image %q is not built — build it first: %s build -t %s . (docker: -f Containerfile). %s", desc.Image, runtime, desc.Image, tail)
	}
}
