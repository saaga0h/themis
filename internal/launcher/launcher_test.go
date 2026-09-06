package launcher

import (
	"reflect"
	"strings"
	"testing"
)

// The safety invariant, exhaustively: InProcess iff inSandbox. On the host it is
// never InProcess — Launch when sandboxable, Refuse otherwise.
func TestDecide_SafetyInvariant(t *testing.T) {
	cases := []struct {
		inSandbox, sandboxable bool
		want                   Mode
	}{
		{true, true, InProcess},
		{true, false, InProcess}, // already inside: run, regardless of sandboxable
		{false, true, Launch},
		{false, false, Refuse}, // host + can't sandbox → refuse, NOT in-process
	}
	for _, c := range cases {
		if got := Decide(c.inSandbox, c.sandboxable); got != c.want {
			t.Errorf("Decide(inSandbox=%v, sandboxable=%v) = %v, want %v", c.inSandbox, c.sandboxable, got, c.want)
		}
	}
}

func TestDecide_NeverInProcessOnHost(t *testing.T) {
	// The load-bearing property, stated directly: not being in the sandbox must
	// never yield an in-process run, whatever else is true.
	for _, sandboxable := range []bool{true, false} {
		if got := Decide(false, sandboxable); got == InProcess {
			t.Errorf("Decide(inSandbox=false, sandboxable=%v) = InProcess — the factory would run unsandboxed on the host", sandboxable)
		}
	}
}

func TestBuildRunArgs_Podman(t *testing.T) {
	got := BuildRunArgs(RunSpec{
		Runtime: "podman", Image: "themis-x:latest", WorkDir: "/home/me/proj",
		EnvFile: ".env", Args: []string{"run", "--provider", "gitea"},
	})
	want := []string{
		"run", "--rm", "-i", "--userns=keep-id",
		"-v", "/home/me/proj:/workspace", "-w", "/workspace",
		"--env-file", ".env",
		"-e", "THEMIS_IN_SANDBOX=1", "themis-x:latest",
		"themis", "run", "--provider", "gitea",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("podman argv:\n got:  %v\n want: %v", got, want)
	}
}

func TestBuildRunArgs_DockerNoUserns(t *testing.T) {
	got := BuildRunArgs(RunSpec{
		Runtime: "docker", Image: "img:latest", WorkDir: "/w", Args: []string{"issue", "3"},
	})
	joined := strings.Join(got, " ")
	if strings.Contains(joined, "--userns") {
		t.Errorf("docker must not get --userns=keep-id: %v", got)
	}
	// No env file → no --env-file flag.
	if strings.Contains(joined, "--env-file") {
		t.Errorf("omitted env file must not add --env-file: %v", got)
	}
	// Marker + image + inner command present and ordered.
	if !strings.Contains(joined, "-e THEMIS_IN_SANDBOX=1 img:latest themis issue 3") {
		t.Errorf("expected marker, image, then inner command: %v", got)
	}
}

func TestBuildRunArgs_MarkerAlwaysSet(t *testing.T) {
	for _, rt := range []string{"podman", "docker"} {
		got := BuildRunArgs(RunSpec{Runtime: rt, Image: "i", WorkDir: "/w", Args: []string{"run"}})
		found := false
		for i := 0; i < len(got)-1; i++ {
			if got[i] == "-e" && got[i+1] == "THEMIS_IN_SANDBOX=1" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: the sandbox marker must always be set: %v", rt, got)
		}
	}
}
