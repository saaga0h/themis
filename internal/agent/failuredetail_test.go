package agent

import (
	"strings"
	"testing"
)

// failureDetail must surface stderr (where claude writes its real error) so a
// failed invocation is diagnosable rather than just "exit status 1".
func TestFailureDetail_PrefersStderr(t *testing.T) {
	got := failureDetail("API error: overloaded_error (529)", "some stdout")
	if !strings.Contains(got, "stderr") {
		t.Errorf("expected stderr to be surfaced; got %q", got)
	}
	if !strings.Contains(got, "overloaded_error (529)") {
		t.Errorf("expected the stderr cause in the detail; got %q", got)
	}
}

// When stderr is empty, fall back to stdout so partial --print output is still
// visible.
func TestFailureDetail_FallsBackToStdout(t *testing.T) {
	got := failureDetail("   ", "partial output before crash")
	if !strings.Contains(got, "stdout") {
		t.Errorf("expected stdout fallback; got %q", got)
	}
	if !strings.Contains(got, "partial output before crash") {
		t.Errorf("expected stdout content; got %q", got)
	}
}

// With nothing captured, detail is empty so the caller keeps the bare exit error.
func TestFailureDetail_EmptyWhenNoOutput(t *testing.T) {
	if got := failureDetail("", "  \n "); got != "" {
		t.Errorf("expected empty detail; got %q", got)
	}
}

// tailLines bounds the surfaced output to the last n lines.
func TestTailLines_Bounds(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("line\n")
	}
	got := tailLines(b.String(), 30)
	if n := strings.Count(got, "\n") + 1; n != 30 {
		t.Errorf("expected 30 lines, got %d", n)
	}
}

func TestTailLines_ShorterThanN(t *testing.T) {
	if got := tailLines("a\nb", 30); got != "a\nb" {
		t.Errorf("short input should pass through; got %q", got)
	}
}
