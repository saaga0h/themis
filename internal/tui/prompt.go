// Package tui provides small, dependency-free interactive prompts for themis's
// setup wizard. It is a thin line-based layer — numbered selects and
// default-able text inputs read from an io.Reader and written to an io.Writer —
// deliberately not a full-screen TUI framework: `themis init` is a handful of
// questions, so this stays portable (any OS/shell, no raw-terminal mode) and
// testable (feed a reader, assert the writer). It imports only the standard
// library — no third-party UI framework and no design-system dependency — and it
// writes no files (the caller composes config through internal/workflow).
package tui

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Prompter reads line-based answers from in and renders prompts to out.
type Prompter struct {
	in  *bufio.Reader
	out io.Writer
}

// New returns a Prompter over in/out (e.g. os.Stdin/os.Stdout, or a
// strings.Reader/bytes.Buffer in tests).
func New(in io.Reader, out io.Writer) *Prompter {
	return &Prompter{in: bufio.NewReader(in), out: out}
}

// Input prints label (showing def when non-empty) and returns the entered line;
// empty input returns def.
func (p *Prompter) Input(label, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(p.out, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(p.out, "%s: ", label)
	}
	line, err := p.readLine()
	if err != nil {
		return "", err
	}
	if line == "" {
		return def, nil
	}
	return line, nil
}

// Select prints a numbered menu and returns the chosen zero-based index. Empty
// input selects defIdx when in range; non-numeric or out-of-range input
// re-prompts until valid input or the reader is exhausted.
func (p *Prompter) Select(label string, options []string, defIdx int) (int, error) {
	hasDefault := defIdx >= 0 && defIdx < len(options)
	for {
		fmt.Fprintln(p.out, label)
		for i, opt := range options {
			marker := " "
			if i == defIdx {
				marker = "*"
			}
			fmt.Fprintf(p.out, "  %s %d) %s\n", marker, i+1, opt)
		}
		if hasDefault {
			fmt.Fprintf(p.out, "choose [%d]: ", defIdx+1)
		} else {
			fmt.Fprint(p.out, "choose: ")
		}
		line, err := p.readLine()
		if err != nil {
			return 0, err
		}
		if line == "" && hasDefault {
			return defIdx, nil
		}
		if n, convErr := strconv.Atoi(line); convErr == nil && n >= 1 && n <= len(options) {
			return n - 1, nil
		}
		fmt.Fprintf(p.out, "please enter a number 1-%d\n", len(options))
	}
}

// readLine returns the next trimmed line. A final line without a trailing
// newline is returned; a genuine end-of-input (nothing left) returns the error
// so callers do not loop forever.
func (p *Prompter) readLine() (string, error) {
	line, err := p.in.ReadString('\n')
	if err != nil && (line == "" || err != io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
