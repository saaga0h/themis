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

Read `.themis/review-results.json` for the review findings. The review step wrote
it and it is authoritative. **Do not re-analyze the code or re-classify findings.**

The factory never auto-fixes and never auto-merges — it hands findings to the
human. Report the severities from the JSON as-is:
- **CRITICAL / HIGH** findings (concrete security vulnerabilities, or acceptance
  criteria with no test) are **blocking**. List them under "Blocking findings —
  must be resolved before merge". When any exist, this PR is opened as a **draft**
  (WIP) so it cannot be merged until a maintainer resolves them.
- **LOW** findings are non-blocking notes. List them under "Reviewer observations
  (not addressed — for maintainer triage)".

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

Output ONLY the PR body, wrapped exactly between the two marker lines below — nothing before the first marker, and nothing between the markers except the body itself (no preamble, no commentary, no code fences):

<<<THEMIS_PR_BODY>>>
Closes #{{ISSUE_NUMBER}}

...the rest of the composed PR body...
<<<END_THEMIS_PR_BODY>>>

Do not create the PR itself — the system will handle that.

## Completion

After the end marker, on its own line, output:

STEP COMPLETE
