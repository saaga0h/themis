| Placeholder | Source | Available in |
|-------------|--------|-------------|
| `{{ISSUE_NUMBER}}` | Issue metadata | All templates |
| `{{ISSUE_TITLE}}` | Issue metadata | All templates |
| `{{ACCEPTANCE_CRITERIA}}` | Parsed AC checkboxes | Agent-step templates |
| `{{AC_STATUS}}` | Same as ACCEPTANCE_CRITERIA | `ship.md` only |
| `{{REVIEW_OUTPUT}}` | Last Review step stdout | `ship.md` |
| `{{BLOCKING_FINDINGS}}` | Blocking findings from `.themis/review-results.json`, formatted as severity-ordered human-readable list (critical → high → medium, low omitted) | `fix-findings.md` |
| `{{REVIEW_CYCLE}}` | Current review cycle number (1-indexed) | `fix-findings.md` |
| `{{PIPELINE_SHAPE}}` | Distinct commit prefixes | `ship.md` |
| `{{COMMIT_LOG}}` | Branch commit log | `ship.md` |
| `{{CHANGED_FILES}}` | Files changed on branch | Templates that use it |