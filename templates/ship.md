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

### Review Output

{{REVIEW_OUTPUT}}

### Pipeline Shape

{{PIPELINE_SHAPE}}

### Commit Log

{{COMMIT_LOG}}

## Instructions

Use the `/pr-composition` skill to compose the PR description. Follow the pr-composition skill structure: status line, implementation narrative, pipeline shape, review findings, out-of-scope discoveries, and follow-ups (if any).

The PR body must:

1. Include `Closes #{{ISSUE_NUMBER}}` as the first line
2. Follow the pr-composition structure:
   - **Status line**: "All ACs passed" or "N of M ACs passed — see below"
   - **Implementation narrative**: what happened that commits don't capture
   - **Pipeline shape**: what the commit history reveals (use the pipeline shape above)
   - **Review findings**: honest reporting of all findings from review output
   - **Follow-ups**: explicitly deferred items (omit section if none)

Output the complete PR body as your final response. Do not create the PR itself — the system will handle that.
