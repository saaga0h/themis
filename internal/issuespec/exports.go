package issuespec

import (
	"fmt"
	"strings"
)

// ExportBudget is a parsed per-package declaration from an issue's ` ```exports `
// block: the package exposes exactly these top-level exported identifiers and no
// others (design #111). It bounds the *surplus exported surface* — the options
// structs, one-implementation interfaces, and Manager types over-engineering adds
// — the way footprint bounds the changed files. It does not bound unexported
// machinery (Fable's stated limit); it is the right partial.
type ExportBudget struct {
	Package string
	Names   []string
}

// ParseExports reads the top-level ` ```exports ` block from body (fence-aware,
// like ParseFootprint — nested examples ignored). Each non-empty, non-`#` line is
// `<package>: Name1, Name2, ...`; multiple packages are multiple lines.
func ParseExports(body string) []ExportBudget {
	for _, f := range topLevelFences(body) {
		if f.info != "exports" {
			continue
		}
		return parseExportsContent(body[f.contentStart:f.contentEnd])
	}
	return nil
}

func parseExportsContent(content string) []ExportBudget {
	var budgets []ExportBudget
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pkg, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		pkg = strings.TrimSuffix(strings.TrimSpace(pkg), "/")
		if pkg == "" {
			continue
		}
		var names []string
		for _, n := range strings.Split(rest, ",") {
			if n = strings.TrimSpace(n); n != "" {
				names = append(names, n)
			}
		}
		budgets = append(budgets, ExportBudget{Package: pkg, Names: names})
	}
	return budgets
}

// exportExtract lists a package's exported identifiers from `go doc -short`:
// top-level types/consts/vars/funcs and constructors (`func Name`, which go doc
// indents under their return type), excluding methods (`func (recv) Name`, which
// belong to a type). One %s — the package path.
const exportExtract = `go doc -short ./%s 2>/dev/null | grep -E '^[[:space:]]*(const|type|var) [A-Z]|^[[:space:]]*func [A-Z]' | sed -E 's/^[[:space:]]*(const|type|var|func) ([A-Za-z0-9_]+).*/\2/' | sort -u`

// ExportCheckCommand renders the budgets as a green-gate check (bash; exit 0 when
// every declared package exports only its declared identifiers). Returns "" for no
// budgets. A package with an empty name list must export nothing. A go doc failure
// (missing/unbuildable package) yields no output — fail-safe pass, not a block.
func ExportCheckCommand(budgets []ExportBudget) string {
	if len(budgets) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("fail=0; ")
	for _, bud := range budgets {
		extract := fmt.Sprintf(exportExtract, bud.Package)
		if len(bud.Names) > 0 {
			var allow strings.Builder
			for _, n := range bud.Names {
				fmt.Fprintf(&allow, " -e '%s'", n) // Go identifiers can't contain quotes
			}
			fmt.Fprintf(&b, `extra=$(%s | grep -vxF%s); `, extract, allow.String())
		} else {
			fmt.Fprintf(&b, `extra=$(%s); `, extract)
		}
		fmt.Fprintf(&b, `if [ -n "$extra" ]; then echo "export budget: undeclared exports in %s:"; echo "$extra"; fail=1; fi; `, bud.Package)
	}
	b.WriteString("exit $fail")
	return b.String()
}
