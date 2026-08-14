package tracker

import (
	"strings"
	"testing"
)

func TestParseExports_SinglePackage(t *testing.T) {
	body := "## Exports\n\n```exports\ninternal/allergen: Filter, NewFilter\n```\n"
	got := ParseExports(body)
	if len(got) != 1 || got[0].Package != "internal/allergen" {
		t.Fatalf("got %+v, want one budget for internal/allergen", got)
	}
	if len(got[0].Names) != 2 || got[0].Names[0] != "Filter" || got[0].Names[1] != "NewFilter" {
		t.Errorf("names = %v, want [Filter NewFilter]", got[0].Names)
	}
}

func TestParseExports_MultiplePackages(t *testing.T) {
	body := "```exports\ninternal/a: A1\ninternal/b: B1, B2\n```"
	got := ParseExports(body)
	if len(got) != 2 || got[0].Package != "internal/a" || got[1].Package != "internal/b" {
		t.Fatalf("got %+v, want budgets for a then b", got)
	}
	if len(got[1].Names) != 2 {
		t.Errorf("b names = %v, want 2", got[1].Names)
	}
}

func TestParseExports_Absent(t *testing.T) {
	if got := ParseExports("## Acceptance Criteria\n- [ ] x\n"); got != nil {
		t.Errorf("no exports block should yield nil, got %+v", got)
	}
}

// Fence-aware, like ParseFootprint: an exports block nested inside an example
// fence must be ignored.
func TestParseExports_IgnoresNestedExample(t *testing.T) {
	body := "````markdown\n```exports\ninternal/example: X\n```\n````\n"
	if got := ParseExports(body); got != nil {
		t.Errorf("nested example exports must not be parsed, got %+v", got)
	}
}

// The generated check extracts exported names, allowlists the declared ones, and
// fails on any surplus.
func TestExportCheckCommand_AllowsDeclaredFlagsSurplus(t *testing.T) {
	cmd := ExportCheckCommand([]ExportBudget{{Package: "internal/allergen", Names: []string{"Filter", "NewFilter"}}})
	for _, want := range []string{
		"go doc -short ./internal/allergen",    // extracts the package's surface
		"grep -vxF -e 'Filter' -e 'NewFilter'", // allowlists declared names
		`if [ -n "$extra" ]`,                   // surplus → fail
		"export budget: undeclared exports in internal/allergen",
		"exit $fail",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("ExportCheckCommand missing %q\ngot: %s", want, cmd)
		}
	}
}

// A package declared with no names must export nothing — no allowlist filter, so
// any export is surplus.
func TestExportCheckCommand_EmptyNamesForbidsAllExports(t *testing.T) {
	cmd := ExportCheckCommand([]ExportBudget{{Package: "internal/x", Names: nil}})
	if strings.Contains(cmd, "grep -vxF") {
		t.Errorf("a zero-name budget must not allowlist anything, got: %s", cmd)
	}
	if !strings.Contains(cmd, "go doc -short ./internal/x") {
		t.Errorf("expected the extract for internal/x, got: %s", cmd)
	}
}

func TestExportCheckCommand_EmptyWhenNoBudgets(t *testing.T) {
	if cmd := ExportCheckCommand(nil); cmd != "" {
		t.Errorf("no budgets must produce no check, got %q", cmd)
	}
}
