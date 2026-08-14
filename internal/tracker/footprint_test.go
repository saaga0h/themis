package tracker

import (
	"strings"
	"testing"
)

func TestParseFootprint_PackageList(t *testing.T) {
	body := "## Footprint\n\n```footprint\ninternal/tracker\ncmd/themis/\n```\n"
	fp := ParseFootprint(body)
	if !fp.Declared() || fp.Wide {
		t.Fatalf("expected a declared, non-wide footprint; got %+v", fp)
	}
	if len(fp.Packages) != 2 || fp.Packages[0] != "internal/tracker" || fp.Packages[1] != "cmd/themis" {
		t.Errorf("packages = %v, want [internal/tracker cmd/themis] (trailing slash trimmed)", fp.Packages)
	}
}

func TestParseFootprint_WideWithReason(t *testing.T) {
	body := "```footprint\nwide: module-path rename touches every import\n```"
	fp := ParseFootprint(body)
	if !fp.Declared() || !fp.Wide {
		t.Fatalf("expected a wide footprint; got %+v", fp)
	}
	if fp.Reason != "module-path rename touches every import" {
		t.Errorf("reason = %q, want the captured justification", fp.Reason)
	}
	if fp.CheckCommand(nil) != "" {
		t.Error("a wide footprint must produce no check command (gate waived)")
	}
}

func TestParseFootprint_Absent(t *testing.T) {
	if fp := ParseFootprint("## Acceptance Criteria\n- [ ] x\n"); fp.Declared() {
		t.Errorf("no footprint block should yield Declared()=false, got %+v", fp)
	}
}

// A ```footprint shown inside an example fence must be ignored (fence-aware, same
// as ParseCheckBlocks) — the outer fence is a longer/other-language example.
func TestParseFootprint_IgnoresNestedExample(t *testing.T) {
	body := "````markdown\n```footprint\ninternal/example\n```\n````\n"
	if fp := ParseFootprint(body); fp.Declared() {
		t.Errorf("a footprint nested inside an example fence must not be parsed; got %+v", fp)
	}
}

func TestParseFootprint_SkipsCommentsAndBlanks(t *testing.T) {
	body := "```footprint\n# only these two packages\ninternal/tracker\n\ncmd/themis\n```"
	fp := ParseFootprint(body)
	if len(fp.Packages) != 2 {
		t.Errorf("packages = %v, want 2 (comments/blanks skipped)", fp.Packages)
	}
}

// CheckCommand allows declared packages + exempt files, flags anything else, and
// stays inert when BASE is unresolvable.
func TestFootprint_CheckCommand_AllowedAndExempt(t *testing.T) {
	fp := ParseFootprint("```footprint\ninternal/tracker\n```")
	cmd := fp.CheckCommand([]string{"go.mod", "go.sum"})
	for _, want := range []string{
		`^internal/tracker/`, // package prefix allowed
		`^go\.mod$`,          // exempt file (regex-escaped)
		`^go\.sum$`,
		`git diff --name-only "$BASE" ${COMMIT}`, // backtest-compatible BASE/COMMIT contract
		`if [ -z "$BASE" ]; then exit 0; fi`,     // inert when base unknown
		"exit 1",                                 // fails on an out-of-footprint file
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("CheckCommand missing %q\ngot: %s", want, cmd)
		}
	}
}

func TestFootprint_CheckCommand_EmptyWhenUndeclared(t *testing.T) {
	if cmd := (Footprint{}).CheckCommand(nil); cmd != "" {
		t.Errorf("undeclared footprint must produce no check command, got %q", cmd)
	}
}
