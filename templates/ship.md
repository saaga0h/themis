# Ship PR for issue #{{ISSUE_NUMBER}}

## Issue

{{ISSUE_TITLE}}

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Branch

{{BRANCH_NAME}}

## Instructions

Create a pull request for the completed work. The PR must:

1. Target the integration branch (never another feature branch)
2. Include `Closes #{{ISSUE_NUMBER}}` in the first line of the body
3. Summarise what was implemented
4. Include an AC verification table mapping every AC to the test that proves it
5. Include a **Review Notes** section listing every non-blocking finding
   captured during the review cycle(s), for the human reviewer to decide on
6. List any documentation changes made

### AC verification table format

| AC | Test | Status |
|----|------|--------|
| <AC description> | `<test file>:<test name>` | ✓ |

### After creating the PR

Update the issue label: remove `ready-for-agent`, add `needs-review`.
