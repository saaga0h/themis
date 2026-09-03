# Review implementation for issue #{{ISSUE_NUMBER}}

## What was implemented

{{ISSUE_TITLE}}

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Project standards

Consult this project's standards and terminology when judging findings:

{{STANDARDS_DOCS}}

## What this review is — and is not

This is the factory's **single-pass safety gate**, not a comprehensive audit. The
comprehensive audit is the human's job at PR review (and the interactive
`/review` command); your job is only to decide whether this change is safe and
faithful enough to put in front of a human.

Run it **once**. Do not loop, do not re-analyze, do not fix anything — you
produce findings, you never edit code. Ask only two questions, both with ground
truth:

### 1. Security (delegate)

Delegate to the **security-reviewer** subagent via Task, scoped to this branch's
diff against the base. Include in the delegation:

> Running in autonomous mode. Review only the changes on this branch. Report
> concrete, locus-bound vulnerabilities — a named risk at a named file:line
> (injection, hardcoded secret, unsafe input handling, exposed endpoint). Do not
> report style, naming, or speculative concerns.

### 2. AC coverage

AC coverage is checked **deterministically by the factory**, not here: the runner
reads test-architect's `.themis/ac-targets.json` mapping at Review and records any
behavioral AC with no test as a blocking finding. You do not need to grep for it.
If you happen to notice an acceptance criterion whose test is obviously absent or
vacuous and the mapping missed it, note it as a finding — but the mechanical check
is the source of truth.

## Write the results

Write `.themis/review-results.json` with exactly this shape:

    {"findings": [{"severity": "high", "description": "...", "file": "path.go", "line": 42}]}

Severity rules — keep blocking findings to genuine, actionable gates:

- **critical** / **high** — a concrete security vulnerability from the security
  pass, or an acceptance criterion with no test.
- **low** — everything else the security pass happened to observe (style,
  readability, speculative concerns). These are notes for the human reviewer,
  never blockers.

Do not emit "medium". If there are no findings, write `{"findings": []}`.

## Completion

When `.themis/review-results.json` is written, output:

STEP COMPLETE

Do not re-analyze, do not fix, do not read more files.
