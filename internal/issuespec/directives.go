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

// shellBuiltins are shell keywords/builtins that are not external binaries. The
// runnability preflight (#97) must not treat them as a missing tool.
var shellBuiltins = map[string]bool{
	"!": true, "[": true, "[[": true, "]]": true, "test": true,
	"cd": true, "pwd": true, "echo": true, "printf": true, "read": true,
	"true": true, "false": true, ":": true, "exit": true, "return": true,
	"export": true, "unset": true, "set": true, "shift": true, "eval": true,
	"source": true, ".": true, "local": true, "declare": true, "trap": true,
	"if": true, "then": true, "else": true, "elif": true, "fi": true,
	"for": true, "while": true, "until": true, "do": true, "done": true,
	"case": true, "esac": true, "in": true, "function": true, "select": true,
	"break": true, "continue": true, "wait": true, "time": true,
	"command": true, "type": true, "hash": true, "getopts": true, "umask": true,
}

var envAssignRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// splitTopLevelSegments splits a shell command on control operators (||, &&, ;,
// |, newline) that appear at the TOP level only — not inside single/double
// quotes, `$(...)` command substitution, or backticks. This is what keeps a `|`
// inside a grep regex (`grep -E 'a|b'`) or inside `$(… | …)` from being read as a
// pipe. Single `&` is not a separator, so redirections like `2>&1` stay attached.
func splitTopLevelSegments(s string) []string {
	var segs []string
	var cur strings.Builder
	var single, double, backtick bool
	depth := 0 // $(...) nesting
	flush := func() { segs = append(segs, cur.String()); cur.Reset() }
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case single:
			cur.WriteRune(c)
			if c == '\'' {
				single = false
			}
		case double:
			cur.WriteRune(c)
			if c == '"' {
				double = false
			} else if c == '$' && i+1 < len(r) && r[i+1] == '(' {
				cur.WriteRune(r[i+1])
				i++
				depth++
			}
		case backtick:
			cur.WriteRune(c)
			if c == '`' {
				backtick = false
			}
		case depth > 0:
			cur.WriteRune(c)
			switch c {
			case '(':
				depth++
			case ')':
				depth--
			case '\'':
				single = true
			case '"':
				double = true
			}
		default: // top level
			switch {
			case c == '\'':
				single = true
				cur.WriteRune(c)
			case c == '"':
				double = true
				cur.WriteRune(c)
			case c == '`':
				backtick = true
				cur.WriteRune(c)
			case c == '$' && i+1 < len(r) && r[i+1] == '(':
				cur.WriteRune(c)
				cur.WriteRune(r[i+1])
				i++
				depth++
			case c == '\n' || c == ';':
				flush()
			case c == '|':
				flush()
				if i+1 < len(r) && r[i+1] == '|' {
					i++
				}
			case c == '&' && i+1 < len(r) && r[i+1] == '&':
				flush()
				i++
			default:
				cur.WriteRune(c)
			}
		}
	}
	flush()
	return segs
}

// CheckBlockCommands extracts the external command names a check block invokes,
// so the runnability preflight (#97) can verify each resolves on PATH. It is a
// heuristic tokenizer, not a shell parser: it splits the check into top-level
// segments (quote/substitution-aware, see splitTopLevelSegments), then for each
// segment takes the leading command word after stripping a `!` negation and
// VAR=val assignments. It omits shell builtins/keywords and anything it cannot
// resolve to a concrete name — variables ($X, ${X}), command substitutions
// ($(...)/backticks), and subshells — so the preflight only ever flags a command
// it is certain about. Commands nested inside `$(...)` are intentionally not
// extracted (residue → skill guidance). The result is deduped, order-preserving.
func CheckBlockCommands(check string) []string {
	var out []string
	seen := map[string]bool{}
	for _, seg := range splitTopLevelSegments(check) {
		fields := strings.Fields(seg)
		i := 0
		for i < len(fields) && (fields[i] == "!" || envAssignRE.MatchString(fields[i])) {
			i++
		}
		if i >= len(fields) {
			continue
		}
		cmd := fields[i]
		if cmd == "" || shellBuiltins[cmd] {
			continue
		}
		// Anything we cannot resolve to a concrete binary name is omitted, not guessed:
		// variables, command substitutions, subshells, and quoted/leading-quote words.
		if r := []rune(cmd)[0]; r == '$' || r == '`' || r == '(' || r == '\'' || r == '"' {
			continue
		}
		if seen[cmd] {
			continue
		}
		seen[cmd] = true
		out = append(out, cmd)
	}
	return out
}

// leadingRemovalRE matches destructive ACs whose leading assertion is a removal:
// "No X remains" / "Removed X" (per skills/issue-writer/SKILL.md). Both are
// anchored to the start of the AC, so they fire only when the AC's main
// assertion is the removal.
var leadingRemovalRE = regexp.MustCompile(`(?i)^no\b.*\bremains?\b|^removed\b`)

// noLongerExistsRE matches the "X no longer exists" absence phrasing. Unlike the
// leading forms this can appear mid-sentence, so on its own it over-matches a
// negation used as a *precondition* ("…because it no longer exists, the runner
// resets…"). destructiveTrigger therefore treats it as destructive only when it
// is not governed by a preceding subordinating conjunction (subordinatorRE).
var noLongerExistsRE = regexp.MustCompile(`(?i)\bno longer exists?\b`)

// subordinatorRE matches a subordinating conjunction. When one precedes a
// "no longer exists" phrase in the AC, that phrase describes a condition rather
// than asserting a removal the change must effect, so it is not destructive.
// The set is deliberately limited to strong, unambiguous precondition markers:
// broader ones (as, since, after, before, once, where) can appear innocuously in
// a genuine removal AC, and wrongly excluding a real removal — silently dropping
// its check requirement — is the worse failure than a false positive (which
// yields an actionable "reword or add a check" error).
var subordinatorRE = regexp.MustCompile(`(?i)\b(because|when|whenever|while|if|unless)\b`)

// destructiveTrigger returns the phrase that classifies ac as a destructive AC,
// or "" when ac is not destructive. A leading "No X remains" / "Removed X" always
// qualifies; a "no longer exists" phrase qualifies only as a main assertion — one
// with no subordinating conjunction before it (a subordinated phrase is a
// precondition, not a removal). The returned trigger drives the rejection message.
func destructiveTrigger(ac string) string {
	ac = strings.TrimSpace(ac)
	if s := leadingRemovalRE.FindString(ac); s != "" {
		return s
	}
	if m := noLongerExistsRE.FindStringIndex(ac); m != nil && !subordinatorRE.MatchString(ac[:m[0]]) {
		return ac[m[0]:m[1]]
	}
	return ""
}

// IsDestructiveAC reports whether ac describes a destructive/negative AC — one
// asserting that something must no longer exist after the change (e.g. "No
// GiteaQuerier struct remains in cmd/themis", "The GiteaQuerier struct no longer
// exists", "Removed hardcoded Finna config from GetSources handler"). A negation
// that merely states a precondition ("…because the branch no longer exists…") is
// not destructive. Destructive ACs require a paired check block; see
// ValidateDestructiveChecks.
func IsDestructiveAC(ac string) bool {
	return destructiveTrigger(ac) != ""
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
		trigger := destructiveTrigger(ac)
		if trigger == "" {
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
			return fmt.Errorf("destructive AC %q (matched %q) requires a paired ```check``` block — add one, or reword if this phrase is a precondition rather than a removal", ac, trigger)
		}
	}
	return nil
}
