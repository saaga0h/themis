# Prompt

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: internal/prompt/, templates/ -->

## Overview

`internal/prompt` provides template placeholder substitution for pipeline agent prompts. Its single exported function, `Substitute`, replaces `{{KEY}}` tokens in a template string with caller-supplied values and enforces **bidirectional validation**: both missing args and unused args are errors.

The package is called exclusively by `internal/runner`, which pre-filters its arg map before each call so that only the placeholders actually present in the target template are passed in.

## Key Files & Entry Points

| File | Role |
|------|------|
| `internal/prompt/prompt.go` | Package implementation; exports `Substitute` |
| `internal/prompt/prompt_test.go` | Unit tests covering all validation and substitution cases |
| `internal/runner/runner.go` | Sole caller; contains `buildTemplateArgs`, `filterArgs`, and the `prompt.Substitute` call sites |
| `templates/*.md` | Template files consumed after substitution |

## Placeholder Grammar

A placeholder is matched by:

```
\{\{([A-Z0-9_]+)\}\}
```

- Delimiters: `{{` and `}}`
- Key characters: uppercase ASCII letters (`A-Z`), digits (`0-9`), underscores (`_`)
- **Case-sensitive**: `{{name}}` and `{{Name}}` are not recognised as placeholders; they pass through the template string unchanged and generate no error
- Single-brace constructs (`{value}`) are not matched and pass through unchanged
- A key may appear multiple times in a template; it counts as one unique placeholder and requires exactly one arg entry

## Substitute()

```go
func Substitute(template string, args map[string]string) (string, error)
```

**What it does:**

1. Scans `template` for all `{{KEY}}` tokens; collects the set of unique keys.
2. For every key found in the template, checks that `args[key]` exists. If any key is absent: returns `""` and an error.
3. For every key in `args`, checks that the template contained a matching `{{KEY}}`. If any arg is unused: returns `""` and an error.
4. Replaces every `{{KEY}}` occurrence with `args[key]` using `regexp.ReplaceAllStringFunc`.

**Bidirectional validation — exact error messages:**

| Condition | Error message |
|-----------|--------------|
| Template has `{{KEY}}` but `args` has no entry for `KEY` | `template placeholder {{KEY}} has no corresponding argument` |
| `args` has entry `KEY` but template has no `{{KEY}}` | `argument "KEY" has no corresponding {{KEY}} placeholder in template` |

Both checks run before any substitution occurs. The first key that fails either check determines the error; iteration order over maps is non-deterministic, so callers must not rely on which key is named when multiple keys violate the same condition.

An empty template with an empty args map is valid and returns `("", nil)`.

## Interaction with the Runner

`internal/runner` is the sole caller of `Substitute`. The interaction has two stages:

**Stage 1 — build a superset arg map** (`buildTemplateArgs`):

`buildTemplateArgs` always produces the same 13-key map regardless of which template is being processed:

| Key | Source |
|-----|--------|
| `ISSUE_NUMBER` | `cfg.IssueNumber` (stringified) |
| `ISSUE_TITLE` | `issue.Title` |
| `ACCEPTANCE_CRITERIA` | `tracker.ParseCheckboxes` formatted as checkboxes |
| `AC_STATUS` | same as `ACCEPTANCE_CRITERIA` |
| `CODING_STANDARDS` | contents of `CODING_STANDARDS.md` (empty string if absent) |
| `UBIQUITOUS_LANGUAGE` | contents of `UBIQUITOUS_LANGUAGE.md` (empty string if absent) |
| `BRANCH_NAME` | current git branch |
| `CHANGED_FILES` | `git.ChangedFiles` output |
| `REVIEW_CYCLE` | `state.ReviewCycle + 1` (stringified) |
| `BLOCKING_FINDINGS` | formatted blocking findings from last review cycle |
| `REVIEW_OUTPUT` | stdout from the review step agent |
| `PIPELINE_SHAPE` | conventional commit prefixes extracted from branch commit log |
| `COMMIT_LOG` | full branch commit log |

**Stage 2 — pre-filter to only what the template uses** (`filterArgs`):

Before calling `Substitute`, the runner calls `filterArgs(tmplContent, allArgs)`, which scans the template with the same `[A-Z0-9_]` regex and returns a new map containing only the keys that have a matching `{{KEY}}` in the template.

```
allArgs (13 keys) → filterArgs(template) → filteredArgs (N keys, N ≤ 13) → Substitute
```

**Why this matters:** `Substitute` treats any arg without a matching placeholder as an error. Because each template only uses a subset of the 13 available keys, passing `allArgs` directly would always fail on the unused keys. `filterArgs` is what makes the strict "no unused arg" check workable: by the time `Substitute` is called, the arg map is guaranteed to contain exactly the keys the template declares.

## Templates Using It

All six agent-step templates and the ship template are processed through `filterArgs` + `Substitute`. The placeholders each template actually uses:

| Template file | Placeholders used |
|---------------|-------------------|
| `test-red.md` | `ISSUE_NUMBER`, `ACCEPTANCE_CRITERIA`, `CODING_STANDARDS` |
| `implement.md` | `ISSUE_NUMBER`, `ACCEPTANCE_CRITERIA`, `CODING_STANDARDS`, `ISSUE_TITLE` |
| `refactor.md` | `ISSUE_NUMBER`, `CODING_STANDARDS`, `UBIQUITOUS_LANGUAGE` |
| `review.md` | `ISSUE_NUMBER`, `ISSUE_TITLE`, `ACCEPTANCE_CRITERIA`, `CODING_STANDARDS` |
| `fix-findings.md` | `ISSUE_NUMBER`, `REVIEW_CYCLE`, `BLOCKING_FINDINGS`, `ACCEPTANCE_CRITERIA`, `CODING_STANDARDS` |
| `update-docs.md` | `ISSUE_NUMBER`, `CHANGED_FILES`, `CODING_STANDARDS` |
| `ship.md` | `ISSUE_NUMBER`, `ISSUE_TITLE`, `ACCEPTANCE_CRITERIA`, `BRANCH_NAME`, `AC_STATUS`, `REVIEW_OUTPUT`, `PIPELINE_SHAPE`, `COMMIT_LOG` |

## How Do I Add a New Placeholder?

1. **Add `{{NEW_KEY}}` to the template file** (key must be `[A-Z0-9_]` only).
2. **Add `"NEW_KEY": value` to `buildTemplateArgs`** in `internal/runner/runner.go`. The `filterArgs` call will automatically include it for templates that reference it and exclude it from templates that do not.
3. No changes to `internal/prompt` are needed.

If the value should come from a new data source (e.g., a file read or a git command), add that read inside `buildTemplateArgs` alongside the existing ones.

## How Do I Diagnose Problems?

Errors from `Substitute` surface as `fmt.Errorf("substituting template %s: %w", templateFile[step], err)` in the runner.

| Symptom | Likely cause | Check |
|---------|-------------|-------|
| `template placeholder {{KEY}} has no corresponding argument` | A `{{KEY}}` in a template has no matching entry in `buildTemplateArgs` | Verify the key is spelled correctly (`[A-Z0-9_]`) in both the template and `buildTemplateArgs` |
| `argument "KEY" has no corresponding {{KEY}} placeholder in template` | An arg was present after `filterArgs` but the template has no matching `{{KEY}}` | This should not happen if `filterArgs` is used correctly; check that `filterArgs` is called before `Substitute` and that its regex matches the template's actual placeholder syntax |
| Placeholder text appears unchanged in agent prompt | The key uses lowercase or mixed case (e.g. `{{name}}`); not matched by `[A-Z0-9_]+` | Rename the placeholder to uppercase |
| `reading template <file>: ...` error before substitution | Template file not found at `cfg.TemplateDir` | Check `cfg.TemplateDir` is set correctly |

Because `filterArgs` uses the same regex (`\{\{([A-Z0-9_]+)\}\}`) as `Substitute`, any placeholder that `filterArgs` misses will also be missed by `Substitute`; the only way to get an "unused arg" error through `filterArgs` would be a regex mismatch or a bug in `filterArgs` itself.

## What Is the Failure Behavior?

`Substitute` returns `("", error)` immediately on the first validation failure. No partial substitution is returned.

| Failure | Return value |
|---------|-------------|
| Placeholder in template with no arg | `("", fmt.Errorf("template placeholder {{%s}} has no corresponding argument", key))` |
| Arg with no placeholder in template | `("", fmt.Errorf("argument %q has no corresponding {{%s}} placeholder in template", key, key))` |
| OS error reading template file (runner side) | Runner returns `fmt.Errorf("reading template %s: %w", ...)` before `Substitute` is called |

There are no silent fallbacks. An arg value of `""` (empty string) is valid; the placeholder is replaced with the empty string and no error is returned.

## Dependencies

`internal/prompt` depends only on the Go standard library:

| Import | Use |
|--------|-----|
| `regexp` | Compile and apply `\{\{([A-Z0-9_]+)\}\}` |
| `fmt` | Format error messages |

## Related Documents

- `ARCHITECTURE.md` — component inventory and pipeline overview
- `docs/subsystems/runner/README.md` — runner subsystem; documents `buildTemplateArgs`, `filterArgs`, and the full pipeline step loop that calls `Substitute`
