# Update documentation for issue #{{ISSUE_NUMBER}}

## What changed

{{CHANGED_FILES}}

## Project standards

Follow this project's coding standards, terminology, and architecture. They are
authoritative — projects differ in style, naming, error handling, and structure,
so read them before writing or changing code:

{{STANDARDS_DOCS}}

## Instructions

Update only the documentation this change affects — scope to the diff, not a full
audit. Do not rewrite sections unrelated to the changed code.

This project's documentation conventions and structure are described by its
standards docs above (and by the existing docs in the repo); follow them rather
than imposing a new structure. Mirror how sibling code is already documented.

Typical updates to look for:

- A changed or new **public/exported surface** (a CLI flag, an exported function,
  type, or config field) → update the doc that describes it.
- **Changed user-facing behaviour** → update the README or the relevant guide.
- A **new package or subsystem**, *if this project documents those* → add or
  extend its doc following the project's existing pattern.

Use `grep -n` to find the doc sections that mention the changed code and edit
only those. Do not invent documentation structure the project does not already
use, and do not create per-issue module docs.

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
