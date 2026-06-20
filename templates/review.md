# Review implementation for issue #{{ISSUE_NUMBER}}

## What was implemented

{{ISSUE_TITLE}}

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

Read `CODING_STANDARDS.md` and `UBIQUITOUS_LANGUAGE.md` from the workspace root
for coding standards and terminology rules.

## Diff size

{{DIFF_LINES}} lines changed.

If under 50 lines and no security/numerical signals detected, use:
> /review --autonomous --quick

Otherwise use:
> /review --autonomous

## Instructions

Run the **review command** via Task to review the implementation on the current
branch. Pass `--autonomous` — the review command scopes to the branch diff
automatically. Choose `--quick` or the full review per the **Diff size** section
above:

> /review --autonomous

Running in autonomous mode. Include this instruction in the delegation:
> Running in autonomous mode. Skip human confirmation and recommendation steps.
> Write structured findings to .themis/review-results.json.

The review command will:
1. Run all standard review agents (complexity, convention, coverage, security, architecture)
2. Classify each finding by severity (CRITICAL, HIGH, MEDIUM, LOW)
3. Write structured results to `.themis/review-results.json`

After the review command returns, verify that `.themis/review-results.json` exists.
If it does not exist, something went wrong — report the error.

Do not classify findings yourself. Do not write the JSON file yourself.
The review command handles all of this.

## Completion

When the review command has written .themis/review-results.json, output:

STEP COMPLETE

Do not re-analyze, do not suggest additional improvements, do not read more files.
