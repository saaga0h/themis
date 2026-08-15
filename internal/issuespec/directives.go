// Package issuespec parses the directive syntax embedded in issue bodies —
// checkbox ACs, ```check blocks, destructive-AC validation, ```footprint, and
// ```exports declarations. It has no knowledge of any specific issue tracker;
// internal/tracker fetches and queries issues and hands their body text here.
package issuespec

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var checkboxRE = regexp.MustCompile(`(?m)^- \[[ xX]\] (.+)$`)

// fence is a top-level (outermost) fenced code block found in an issue body.
// A fence nested inside a longer outer fence — e.g. a ```check block shown
// literally inside a ````markdown example — is not reported as its own
// fence; it is content of the outer one, per CommonMark fence-length rules.
type fence struct {
	start, end               int // byte span of the whole fence, opening line through closing line
	contentStart, contentEnd int // byte span of the fence's inner content
	info                     string
}

// topLevelFences scans body for fenced code blocks (``` or ~~~, three or
// more marker characters) and returns only the outermost ones in source
// order. It is what makes ParseCheckboxes, ParseCheckBlocks, and
// ValidateDestructiveChecks fence-aware: checkbox and check-block syntax
// shown inside a fenced example must not be mistaken for real ACs or check
// directives.
func topLevelFences(body string) []fence {
	lines := strings.Split(body, "\n")
	offsets := make([]int, len(lines)+1)
	off := 0
	for i, l := range lines {
		offsets[i] = off
		off += len(l)
		if i != len(lines)-1 {
			off++ // account for the "\n" strings.Split consumed
		}
	}
	offsets[len(lines)] = off

	var fences []fence
	i := 0
	for i < len(lines) {
		indent, rest := leadingIndent(lines[i])
		if indent <= 3 {
			ch, runLen := fenceMarker(rest)
			if runLen >= 3 {
				info := strings.TrimSpace(rest[runLen:])
				closeIdx := -1
				for j := i + 1; j < len(lines); j++ {
					cIndent, cRest := leadingIndent(lines[j])
					if cIndent > 3 {
						continue
					}
					cCh, cRunLen := fenceMarker(cRest)
					if cRunLen >= runLen && cCh == ch && strings.TrimSpace(cRest[cRunLen:]) == "" {
						closeIdx = j
						break
					}
				}
				var endOffset, contentEnd, nextI int
				if closeIdx == -1 {
					endOffset = len(body)
					contentEnd = len(body)
					nextI = len(lines)
				} else {
					endOffset = offsets[closeIdx] + len(lines[closeIdx])
					contentEnd = offsets[closeIdx]
					nextI = closeIdx + 1
				}
				fences = append(fences, fence{
					start:        offsets[i],
					end:          endOffset,
					contentStart: offsets[i+1],
					contentEnd:   contentEnd,
					info:         info,
				})
				i = nextI
				continue
			}
		}
		i++
	}
	return fences
}

// leadingIndent splits off up to a line's leading spaces, per CommonMark's
// rule that a fence marker may be indented at most three spaces.
func leadingIndent(line string) (int, string) {
	n := 0
	for n < len(line) && line[n] == ' ' {
		n++
	}
	return n, line[n:]
}

// fenceMarker reports the run of leading backticks or tildes at the start of
// s, if any (a run shorter than 3 is not a fence marker and is reported as
// such by the caller checking the returned length).
func fenceMarker(s string) (byte, int) {
	if len(s) == 0 || (s[0] != '`' && s[0] != '~') {
		return 0, 0
	}
	ch := s[0]
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	return ch, n
}

// inFence reports whether byte offset pos falls inside any fence in fences.
// It binary-searches fences, relying on topLevelFences returning them in
// strictly ascending, non-overlapping order by start.
func inFence(fences []fence, pos int) bool {
	i := sort.Search(len(fences), func(i int) bool { return fences[i].start > pos })
	return i > 0 && pos < fences[i-1].end
}

// ParseCheckboxes extracts checkbox items from a markdown body. Both
// unchecked (- [ ]) and checked (- [x]) items are returned. Checkbox lines
// inside a fenced code block (e.g. an example showing AC syntax) are not
// real ACs and are ignored.
func ParseCheckboxes(body string) []string {
	fences := topLevelFences(body)
	matches := checkboxRE.FindAllStringSubmatchIndex(body, -1)
	items := make([]string, 0, len(matches))
	for _, m := range matches {
		if inFence(fences, m[0]) {
			continue
		}
		items = append(items, strings.TrimSpace(body[m[2]:m[3]]))
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

// ParseCheckBlocks extracts each top-level fenced ```check ... ``` block's
// inner command from a markdown issue body, trimmed, in source order. These
// are the issue-declared negative/placement/delegation-AC verifications
// described in skills/issue-writer/SKILL.md: the factory appends each to the
// Green Gate for that run only — issue checks are never committed and never
// touch the project's .themis/workflow.yaml verify contract. A check fence
// shown nested inside a longer outer fence (e.g. an example) is content of
// that outer fence, not a real check directive, and is ignored.
func ParseCheckBlocks(body string) []string {
	fences := topLevelFences(body)
	var blocks []string
	for _, f := range fences {
		if f.info != "check" {
			continue
		}
		blocks = append(blocks, strings.TrimSpace(body[f.contentStart:f.contentEnd]))
	}
	return blocks
}

// destructiveACRE matches the AC phrasings skills/issue-writer/SKILL.md
// prescribes for negative/absence ACs: "No X remains", "X no longer exists",
// and "Removed X".
var destructiveACRE = regexp.MustCompile(`(?i)^no\b.*\bremains?\b|\bno longer exists?\b|^removed\b`)

// IsDestructiveAC reports whether ac describes a destructive/negative AC — one
// asserting that something must no longer exist after the change (e.g. "No
// GiteaQuerier struct remains in cmd/themis", "Removed hardcoded Finna config
// from GetSources handler"). Destructive ACs require a paired check block; see
// ValidateDestructiveChecks.
func IsDestructiveAC(ac string) bool {
	return destructiveACRE.MatchString(strings.TrimSpace(ac))
}

// ValidateDestructiveChecks returns an error naming the offending AC when body
// contains a destructive AC (per IsDestructiveAC) with no check block
// positioned in its own span — from that AC's checkbox up to the next
// destructive AC (or end of body), so a behavioural AC paired after it does not
// orphan the check. Returns nil when every destructive AC has a
// check block in its own span, or when body has no destructive ACs at all.
// Pairing is by position rather than by count: two check blocks stacked after
// one destructive AC do not satisfy a later destructive AC that has none of
// its own, per the "one check block per negative/placement/delegation AC"
// rule in skills/issue-writer/SKILL.md.
func ValidateDestructiveChecks(body string) error {
	fences := topLevelFences(body)

	var acMatches [][]int
	for _, m := range checkboxRE.FindAllStringSubmatchIndex(body, -1) {
		if inFence(fences, m[0]) {
			continue
		}
		acMatches = append(acMatches, m)
	}
	if len(acMatches) == 0 {
		return nil
	}

	var checkBlockStarts []int
	for _, f := range fences {
		if f.info == "check" {
			checkBlockStarts = append(checkBlockStarts, f.start)
		}
	}

	for i, m := range acMatches {
		ac := strings.TrimSpace(body[m[2]:m[3]])
		if !IsDestructiveAC(ac) {
			continue
		}
		spanStart := m[0]
		// The span runs to the next DESTRUCTIVE AC (or end of body), not the next
		// checkbox of any kind — so a behavioural AC paired after the destructive
		// one does not orphan its check block. Each destructive AC is still bound
		// to a check in its own span, keeping the clustered-checks hole closed.
		spanEnd := len(body)
		for j := i + 1; j < len(acMatches); j++ {
			nextAC := strings.TrimSpace(body[acMatches[j][2]:acMatches[j][3]])
			if IsDestructiveAC(nextAC) {
				spanEnd = acMatches[j][0]
				break
			}
		}
		paired := false
		for _, cb := range checkBlockStarts {
			if cb >= spanStart && cb < spanEnd {
				paired = true
				break
			}
		}
		if !paired {
			return fmt.Errorf("destructive AC %q has no accompanying check block", ac)
		}
	}
	return nil
}
