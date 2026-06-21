# Update documentation for issue #{{ISSUE_NUMBER}}

## What this issue changed

{{ISSUE_TITLE}}

{{ACCEPTANCE_CRITERIA}}

### Files changed

{{CHANGED_FILES}}

## Project standards (terminology and conventions)

{{STANDARDS_DOCS}}

## Instructions

This is a **minimal, issue-scoped** documentation pass — not an audit. A full
documentation sweep (thoroughness, drift cleanup) is a separate `/document` run;
that is its job, not yours.

Ask one question: **does what this issue changed alter something a documentation
reader needs to know?** — a CLI flag, an exported function or type, a config
field, or user-facing behaviour.

- **If yes:** find the single doc (or few) that describe that surface and make the
  minimal edit. Use `grep -n` to locate the exact section and edit only it.
  Mirror the project's existing doc structure — do not invent new docs or sections.
- **If the change is internal-only** (no reader-facing surface changed), output
  `STEP COMPLETE — no changes`. Do not document internal refactors and do not
  scan for unrelated doc drift — that is the full documentation run's job.

## Commit

If any docs were updated:
```
docs(<scope>): update documentation for issue #{{ISSUE_NUMBER}}
```

If no docs needed updating, do not create an empty commit.

## Completion

First decide in one pass whether any documentation needs updating.
If nothing needs updating, output immediately:

STEP COMPLETE — no changes

Do not re-scan, do not re-read source files looking for more doc opportunities.

If docs were updated, commit and output:

STEP COMPLETE
