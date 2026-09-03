package issuespec_test

import (
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/issuespec"
)

func TestParseCheckboxes_ExtractsBothCheckedAndUnchecked(t *testing.T) {
	body := "## Summary\n\nSome text.\n\n## Acceptance Criteria\n\n- [ ] First item\n- [x] Second item already done\n- [ ] Third item\n\n## Notes\n\nSome notes."
	got := issuespec.ParseCheckboxes(body)
	want := []string{"First item", "Second item already done", "Third item"}
	if len(got) != len(want) {
		t.Fatalf("ParseCheckboxes: got %d items %v, want %d %v", len(got), got, len(want), want)
	}
	for i, g := range got {
		if g != want[i] {
			t.Errorf("item[%d]: got %q, want %q", i, g, want[i])
		}
	}
}

func TestParseCheckboxes_EmptyBody(t *testing.T) {
	got := issuespec.ParseCheckboxes("")
	if len(got) != 0 {
		t.Errorf("ParseCheckboxes(\"\") = %v, want empty", got)
	}
}

func TestParseCheckboxes_NoCheckboxes(t *testing.T) {
	body := "## Description\n\nJust some text without checkboxes."
	got := issuespec.ParseCheckboxes(body)
	if len(got) != 0 {
		t.Errorf("ParseCheckboxes (no checkboxes) = %v, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Issue #98: extract issue-declared `check` blocks into the green gate.
// ---------------------------------------------------------------------------

func TestParseCheckBlocks_ExtractsFencedCommand(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] No Foo remains\n\n```check\n! grep -rq 'type Foo' pkg/\n```\n"
	got := issuespec.ParseCheckBlocks(body)
	want := []string{"! grep -rq 'type Foo' pkg/"}
	if len(got) != len(want) {
		t.Fatalf("ParseCheckBlocks: got %d blocks %v, want %d %v", len(got), got, len(want), want)
	}
	if got[0] != want[0] {
		t.Errorf("ParseCheckBlocks[0]: got %q, want %q", got[0], want[0])
	}
}

func TestParseCheckBlocks_NoBlocksReturnsEmpty(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] Something behavioural\n\n" +
		"```\nplain fenced block, no tag\n```\n\n" +
		"```text\nsome other tagged fence\n```\n"
	got := issuespec.ParseCheckBlocks(body)
	if len(got) != 0 {
		t.Errorf("ParseCheckBlocks (no check fences) = %v, want empty", got)
	}
}

func TestParseCheckBlocks_MultipleBlocksPreserveOrder(t *testing.T) {
	body := "```check\nfirst command\n```\n\nSome prose in between the two check blocks.\n\n```check\nsecond command\n```\n"
	got := issuespec.ParseCheckBlocks(body)
	want := []string{"first command", "second command"}
	if len(got) != len(want) {
		t.Fatalf("ParseCheckBlocks: got %d blocks %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("block[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseCheckBlocks_IgnoresNonCheckFences(t *testing.T) {
	body := "```go\nfunc main() {}\n```\n\n```\nuntagged fence, not a check\n```\n\n```check\nreal check command\n```\n"
	got := issuespec.ParseCheckBlocks(body)
	want := []string{"real check command"}
	if len(got) != len(want) {
		t.Fatalf("ParseCheckBlocks: got %d blocks %v, want %d %v (must ignore go/untagged fences)", len(got), got, len(want), want)
	}
	if got[0] != want[0] {
		t.Errorf("ParseCheckBlocks[0]: got %q, want %q", got[0], want[0])
	}
}

func TestIsDestructiveAC_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		ac   string
		want bool
	}{
		{"no struct remains", "No GiteaQuerier struct remains in cmd/themis", true},
		{"old thing no longer exists", "the old GiteaFetcher no longer exists in cmd/themis", true},
		{"removed hardcoded config", "Removed hardcoded Finna config from GetSources handler", true},
		{"plain behavioural AC", "CreateItem returns 400 for invalid itemType values", false},
		// #121: a negation used as a precondition (subordinate clause) is not a removal.
		{"precondition because", "when the issue branch cannot be checked out because it no longer exists, the runner resets to a fresh start", false},
		{"precondition when", "returns a fresh start when the target branch no longer exists", false},
		{"precondition if", "if the label no longer exists, the run skips the issue", false},
		// A real removal that merely mentions a subordinator *after* the phrase stays destructive.
		{"removal with trailing when", "the cache entry no longer exists when the run finishes", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := issuespec.IsDestructiveAC(tt.ac)
			if got != tt.want {
				t.Errorf("IsDestructiveAC(%q) = %v, want %v", tt.ac, got, tt.want)
			}
		})
	}
}

func TestValidateDestructiveChecks_ErrorsWhenNoCheckBlockPresent(t *testing.T) {
	const offendingAC = "No GiteaQuerier struct remains in cmd/themis"
	body := "## Acceptance Criteria\n- [ ] " + offendingAC + "\n"
	err := issuespec.ValidateDestructiveChecks(body)
	if err == nil {
		t.Fatal("ValidateDestructiveChecks: expected error when a destructive AC has no check block, got nil")
	}
	if !strings.Contains(err.Error(), offendingAC) {
		t.Errorf("ValidateDestructiveChecks error must name the offending AC %q, got %q", offendingAC, err.Error())
	}
}

// #121: a precondition-style AC (negation in a subordinate clause) must pass
// intake with no check block demanded — the #56 wording that hard-failed before.
func TestValidateDestructiveChecks_PassesWhenNegationIsPrecondition(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] when the issue branch cannot be checked out because it no longer exists, the runner resets to a fresh start\n"
	if err := issuespec.ValidateDestructiveChecks(body); err != nil {
		t.Errorf("ValidateDestructiveChecks: got %v, want nil (negation is a precondition, not a removal)", err)
	}
}

// #121: the rejection message names the matched trigger and suggests the fix.
func TestValidateDestructiveChecks_ErrorNamesTriggerAndFix(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] the old GiteaFetcher no longer exists in cmd/themis\n"
	err := issuespec.ValidateDestructiveChecks(body)
	if err == nil {
		t.Fatal("ValidateDestructiveChecks: expected error for an unpaired destructive AC, got nil")
	}
	if !strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("error should name the trigger phrase %q, got %q", "no longer exists", err.Error())
	}
	if !strings.Contains(err.Error(), "reword") {
		t.Errorf("error should suggest the fix (reword/add a check), got %q", err.Error())
	}
}

func TestValidateDestructiveChecks_PassesWhenCheckBlockPresent(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] No GiteaQuerier struct remains in cmd/themis\n\n" +
		"```check\n! grep -rq 'type GiteaQuerier' cmd/themis/\n```\n"
	if err := issuespec.ValidateDestructiveChecks(body); err != nil {
		t.Errorf("ValidateDestructiveChecks: got %v, want nil (destructive AC is paired with a check block)", err)
	}
}

func TestValidateDestructiveChecks_PassesWhenNoDestructiveACs(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] CreateItem returns 400 for invalid itemType values\n"
	if err := issuespec.ValidateDestructiveChecks(body); err != nil {
		t.Errorf("ValidateDestructiveChecks: got %v, want nil (no destructive ACs present)", err)
	}
}

func TestValidateDestructiveChecks_FailsWhenBothCheckBlocksPrecedeSecondDestructiveAC(t *testing.T) {
	const offendingAC = "No Bar remains in pkg/bar"
	body := "## Acceptance Criteria\n" +
		"- [ ] No Foo remains in pkg/foo\n\n" +
		"```check\n! grep -rq 'type Foo' pkg/foo\n```\n\n" +
		"```check\n! grep -rq 'type FooHelper' pkg/foo\n```\n\n" +
		"- [ ] " + offendingAC + "\n"
	err := issuespec.ValidateDestructiveChecks(body)
	if err == nil {
		t.Fatal("ValidateDestructiveChecks: expected error when the second destructive AC has no check block of its own, got nil")
	}
	if !strings.Contains(err.Error(), offendingAC) {
		t.Errorf("ValidateDestructiveChecks error must name the offending AC %q, got %q", offendingAC, err.Error())
	}
}

func TestValidateDestructiveChecks_PassesWhenEachDestructiveACHasOwnCheckBlock(t *testing.T) {
	body := "## Acceptance Criteria\n" +
		"- [ ] No Foo remains in pkg/foo\n\n" +
		"```check\n! grep -rq 'type Foo' pkg/foo\n```\n\n" +
		"- [ ] No Bar remains in pkg/bar\n\n" +
		"```check\n! grep -rq 'type Bar' pkg/bar\n```\n"
	if err := issuespec.ValidateDestructiveChecks(body); err != nil {
		t.Errorf("ValidateDestructiveChecks: got %v, want nil (each destructive AC is paired with its own check block)", err)
	}
}

func TestValidateDestructiveChecks_PassesWithMixedNonDestructiveAndDestructiveACs(t *testing.T) {
	body := "## Acceptance Criteria\n" +
		"- [ ] CreateItem returns 400 for invalid itemType values\n" +
		"- [ ] No Foo remains in pkg/foo\n\n" +
		"```check\n! grep -rq 'type Foo' pkg/foo\n```\n"
	if err := issuespec.ValidateDestructiveChecks(body); err != nil {
		t.Errorf("ValidateDestructiveChecks: got %v, want nil (non-destructive AC excluded from pairing, sole destructive AC paired with sole check block)", err)
	}
}

// #103: a destructive AC followed by a non-destructive (behavioural, paired) AC and
// THEN its check block passes — the span runs to the next DESTRUCTIVE AC, so an
// intervening behavioural checkbox does not orphan the check. Under the older
// "next checkbox" span this failed, rejecting the encouraged pairing pattern.
func TestValidateDestructiveChecks_PassesWhenCheckFollowsPairedBehaviouralAC(t *testing.T) {
	body := "## Acceptance Criteria\n" +
		"- [ ] No Foo remains in pkg/foo\n" +
		"- [ ] Foo callers now use Bar\n\n" +
		"```check\n! grep -rq 'type Foo' pkg/foo\n```\n"
	if err := issuespec.ValidateDestructiveChecks(body); err != nil {
		t.Errorf("ValidateDestructiveChecks: got %v, want nil (check after an intervening behavioural AC still counts under the next-destructive-AC span)", err)
	}
}

// ---------------------------------------------------------------------------
// Issue #105: issue body parsers are fence-blind. Checkbox and check-block
// lines that appear inside a fenced example (e.g. an "## Example" section
// showing a reader what AC/check syntax looks like) must not be picked up as
// real ACs or check directives.
// ---------------------------------------------------------------------------

func TestParseCheckboxes_IgnoresCheckboxLinesInsideFencedExample(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] First real AC\n- [ ] Second real AC\n\n" +
		"## Example\n\nHere is what an AC checkbox looks like:\n\n" +
		"````markdown\n- [ ] Example fenced AC one\n- [ ] Example fenced AC two\n````\n"
	got := issuespec.ParseCheckboxes(body)
	want := []string{"First real AC", "Second real AC"}
	if len(got) != len(want) {
		t.Fatalf("ParseCheckboxes: got %d items %v, want %d %v", len(got), got, len(want), want)
	}
	for i, g := range got {
		if g != want[i] {
			t.Errorf("item[%d]: got %q, want %q", i, g, want[i])
		}
	}
}

func TestParseCheckBlocks_IgnoresCheckDirectiveInsideFencedExample(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] No Foo remains in pkg/foo\n\n" +
		"```check\n! grep -rq 'type Foo' pkg/foo\n```\n\n" +
		"## Example\n\nHere's what a check block looks like in an issue body:\n\n" +
		"````markdown\n```check\n! grep -rq 'type Bar' pkg/bar\n```\n````\n"
	got := issuespec.ParseCheckBlocks(body)
	want := []string{"! grep -rq 'type Foo' pkg/foo"}
	if len(got) != len(want) {
		t.Fatalf("ParseCheckBlocks: got %d blocks %v, want %d %v (must ignore check fence nested inside example fence)", len(got), got, len(want), want)
	}
	if got[0] != want[0] {
		t.Errorf("ParseCheckBlocks[0]: got %q, want %q", got[0], want[0])
	}
}

func TestValidateDestructiveChecks_IgnoresDestructiveCheckboxInsideFencedExample(t *testing.T) {
	body := "## Acceptance Criteria\n- [ ] CreateItem returns 400 for invalid itemType values\n\n" +
		"## Example\n\nHere's what a destructive AC looks like:\n\n" +
		"````markdown\n- [ ] No GiteaQuerier struct remains in cmd/themis\n````\n"
	if err := issuespec.ValidateDestructiveChecks(body); err != nil {
		t.Errorf("ValidateDestructiveChecks: got %v, want nil (destructive-looking checkbox line is inside a fenced example, not a real AC)", err)
	}
}
