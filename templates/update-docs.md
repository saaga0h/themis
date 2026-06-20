# Update documentation for issue #{{ISSUE_NUMBER}}

## What changed

{{CHANGED_FILES}}

## Project standards

Follow this project's coding standards, terminology, and architecture. They are
authoritative — projects differ in style, naming, error handling, and structure,
so read them before writing or changing code:

{{STANDARDS_DOCS}}

## Instructions

Update documentation scoped to what this issue changed — not a full audit,
just the diff. Do not rewrite sections unrelated to the changed code.

### Determine scope

Group changed files by package or subsystem.

### Check for required updates

For each changed package:

1. Does `docs/subsystems/<package-name>/README.md` exist? If the package is
   new and substantial (new types, interfaces, or functions), it MUST be created.
2. Does `ARCHITECTURE.md` list this package in its component inventory?
   If not, it MUST be updated.
3. Does `docs/content-plan.md` reference any new docs? If not, update it.

### Update drifted docs

For each doc that needs updating:
- Update only the drifted sections — do not rewrite the whole doc
- New package → update `ARCHITECTURE.md` component inventory and create
  `docs/subsystems/<name>/README.md` if the package is substantial
- New public type or function → update the relevant subsystem README
- Existing behaviour changed → update any doc that describes that behaviour

Do not create module-level docs per issue — that is a separate documentation pass.

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
