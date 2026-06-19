# Ship PR for issue #{{ISSUE_NUMBER}}

## Issue

{{ISSUE_TITLE}}

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Branch

{{BRANCH_NAME}}

## Context

### AC Status

{{AC_STATUS}}

### Pipeline Shape

{{PIPELINE_SHAPE}}

### Commit Log

{{COMMIT_LOG}}

## Instructions

Use the `/pr-composition` skill to compose the PR description. Follow the pr-composition skill structure: status line, implementation narrative, pipeline shape, review findings, out-of-scope discoveries, and follow-ups (if any).

### Review findings — authoritative source

Read `.themis/review-results.json` for the review findings. This file was written
by the review command during the Review step and contains the authoritative
severity classifications. **Do not re-analyze the code or re-classify findings.**

Use the severities from the JSON file as-is:
- **CRITICAL / HIGH / MEDIUM** findings that were fixed during review cycles
  should be listed under "Fixed during review cycles"
- **CRITICAL / HIGH / MEDIUM** findings still present should be listed under
  "Blocking — not fixed" (this should be rare — the fix cycle should have
  resolved them)
- **LOW** findings should be listed under "Non-blocking — deferred" with brief
  justification for deferral

Pre-existing issues (violations that existed before this PR, not introduced by
this change) are classified LOW by the review command per CODING_STANDARDS.md.
Report them as LOW. Do not reclassify pre-existing issues as CRITICAL or HIGH —
that is the review command's job, and it already made the classification.

If `.themis/review-results.json` is absent or empty, state "No review findings"
in the review section. Do not invent findings.

### PR body structure

The PR body must:

1. Include `Closes #{{ISSUE_NUMBER}}` as the first line
2. Follow the pr-composition structure:
   - **Status line**: "All ACs passed" or "N of M ACs passed — see below"
   - **Implementation narrative**: what happened that commits don't capture
   - **Pipeline shape**: what the commit history reveals (use the pipeline shape above)
   - **Review findings**: from `.themis/review-results.json` only — not from your own analysis
   - **Follow-ups**: explicitly deferred items (omit section if none)

Output the complete PR body as your final response. Do not create the PR itself — the system will handle that.
