# Profile

<!-- @tier: 2 -->
<!-- @parent: ARCHITECTURE.md -->
<!-- @source: internal/profile/ -->

## Overview

`internal/profile` loads per-project pipeline configuration from `.themis/profile.yaml`. It is the single point of truth for which model the security review agent uses, how round-3 gating behaves, how many test-fix attempts the implement step makes, and whether the docs and refactor steps run.

`Load(dir string) (*Profile, error)` is the only entry point callers need. When `.themis/profile.yaml` is absent it returns a fully-populated `*Profile` holding the hardcoded defaults — no error. When the file exists it is parsed with strict YAML decoding (`KnownFields(true)`) followed by semantic validation; any failure returns a non-nil error and a nil `*Profile`. Fields left empty in the file are back-filled with the same defaults that `Load` uses when the file is absent.

`Save(dir string, p *Profile) error` marshals a `*Profile` back to `.themis/profile.yaml`, creating `.themis/` if needed. It is used by the pipeline initialiser, not by callers that only read configuration.

## Key Files & Entry Points

| File | Role |
|---|---|
| `internal/profile/profile.go` | All logic: types, `Load`, `Save`, `validate`, `applyDefaults`, `defaults` |
| `internal/profile/profile_test.go` | Unit tests covering defaults, valid/invalid inputs, round-trip |

## Profile Schema

The full YAML structure accepted by `.themis/profile.yaml`. Every field is optional; absent fields receive the defaults listed below.

### `review`

| YAML key | Go field | Type | Valid values | Default |
|---|---|---|---|---|
| `review.agents.security` | `Review.Agents.Security` | string | `haiku`, `sonnet`, `opus`, `skip` | `sonnet` |
| `review.round3` | `Review.Round3` | string | `auto`, `always`, `never` | `auto` |
| `review.round3_surfaces` | `Review.Round3Surfaces` | `[]string` | arbitrary strings | `nil` (empty list) |

Setting an agent to `skip` disables that reviewer for the pipeline run.

### `implement`

| YAML key | Go field | Type | Valid values | Default |
|---|---|---|---|---|
| `implement.model` | `Implement.Model` | string | `haiku`, `sonnet`, `opus`, `skip` | `sonnet` |
| `implement.test_fix_attempts` | `Implement.TestFixAttempts` | int | any positive integer | `3` |

### `refactor`

| YAML key | Go field | Type | Valid values | Default |
|---|---|---|---|---|
| `refactor.enabled` | `Refactor.Enabled` | `*bool` | `true`, `false` | `true` |

`Enabled` is a pointer so that an absent field (`nil`) is distinguishable from an explicit `false` during `applyDefaults`. Callers should check for `nil` before dereferencing.

### `docs`

| YAML key | Go field | Type | Valid values | Default |
|---|---|---|---|---|
| `docs.enabled` | `Docs.Enabled` | `*bool` | `true`, `false` | `true` |
| `docs.skip_tiers` | `Docs.SkipTiers` | `[]int` | tier numbers to skip | `nil` (empty list) |

Same pointer semantics as `refactor.enabled`.

### `blocking`

`BlockingConfig` is currently empty — there are no configurable fields in this section. The `blocking:` key is accepted by the strict YAML decoder but carries no effect.

## Defaults When Absent

When `.themis/profile.yaml` does not exist, `Load` returns the struct produced by the private `defaults()` function — identical to what `applyDefaults` would fill in for a completely empty file:

```
Review.Agents.Security    = "sonnet"
Review.Round3             = "auto"
Review.Round3Surfaces     = nil
Implement.Model           = "sonnet"
Implement.TestFixAttempts = 3
Refactor.Enabled          = &true
Docs.Enabled              = &true
Docs.SkipTiers            = nil
```

## Validation Rules

Validation runs after strict YAML decoding and before `applyDefaults`. Both checks happen inside `validate(*Profile)`.

**Agent model names** — the `security` agent field must be one of: `haiku`, `sonnet`, `opus`, `skip`, or the empty string (empty triggers the default). Any other value returns:

```
invalid model "X" for review agent "Y" (must be haiku, sonnet, opus, or skip)
```

**`review.round3`** — must be one of: `auto`, `always`, `never`, or empty string. Any other value returns:

```
invalid round3 value "X" (must be auto, always, or never)
```

**Unknown YAML fields** — `yaml.NewDecoder` is configured with `KnownFields(true)`. Any key not present in the Go structs causes a parse error prefixed `parsing profile: ...`. This catches typos in field names.

**Unreadable file** — if the file exists but cannot be read (permissions, I/O error), `Load` returns `reading profile: <wrapped os error>`.

The `implement.model` field passes through `applyDefaults` but is **not** currently validated by `validate`. Only agent models and `round3` are validated at load time.

## Example `.themis/profile.yaml`

The following file reproduces the built-in defaults exactly:

```yaml
review:
  agents:
    security: sonnet
  round3: auto

implement:
  model: sonnet
  test_fix_attempts: 3

refactor:
  enabled: true

docs:
  enabled: true
```

## Architecture

```
Load(dir)
  └─ os.ReadFile(".themis/profile.yaml")
       absent → defaults()                   [no error]
       present → yaml.NewDecoder + KnownFields(true) + Decode
                  parse error → return nil, error
                  validate(&p)
                    invalid model/round3 → return nil, error
                  applyDefaults(&p)          [fills empty fields from defaults()]
                  return &p, nil
```

`applyDefaults` checks each field individually: a zero value (`""` for strings, `0` for int, `nil` for `*bool`) causes the default to be applied. This means a partial file — one that sets only a few fields — inherits all remaining defaults transparently.

## Data Flow

`.themis/profile.yaml` (YAML on disk) → `Load` → `*Profile` → pipeline runner selects agent models, decides round-3 gate, sets attempt counts, conditionally skips docs/refactor steps.

The `*Profile` is read-only after `Load` returns. Nothing in the pipeline mutates it.

## Interfaces & Contracts

```go
// Load reads .themis/profile.yaml from dir. Returns sensible defaults when
// the file does not exist.
func Load(dir string) (*Profile, error)

// Save writes p to <dir>/.themis/profile.yaml, creating directories as needed.
func Save(dir string, p *Profile) error
```

`Load` and `Save` are the only exported symbols. All struct types (`Profile`, `ReviewConfig`, `AgentConfig`, `ImplementConfig`, `RefactorConfig`, `DocsConfig`, `BlockingConfig`) are also exported for use by callers.

## How to Add a New Configurable Field

1. Add the field to the appropriate config struct in `profile.go` with a `yaml:"..."` tag.
2. Add a corresponding entry in `defaults()` with the desired default value.
3. Add a corresponding branch in `applyDefaults` that applies the default when the field holds its zero value. Use `*bool` (via `boolPtr`) for boolean flags so that absent and explicit-false are distinguishable.
4. If the field is an enum, add a `validXxx` map and a check inside `validate`. Return a descriptive error using the same format as the existing checks.
5. Add a test case in `profile_test.go` covering the default, a valid override, and (for enums) an invalid value.

## How to Diagnose Problems

| Symptom | Check | Fix |
|---|---|---|
| `parsing profile: line N: field X not found in type profile.Y` | Unknown or misspelled key in `.themis/profile.yaml` | Correct the key name against the schema table above |
| `invalid model "X" for review agent "Y"` | Agent field set to an unsupported model name | Use `haiku`, `sonnet`, `opus`, or `skip` |
| `invalid round3 value "X"` | `review.round3` set to an unsupported value | Use `auto`, `always`, or `never` |
| `reading profile: open .themis/profile.yaml: permission denied` | File permissions prevent read | Check file and directory permissions (`0o600`/`0o700` expected) |
| Pipeline ignores `.themis/profile.yaml` entirely | `Load` received the wrong `dir` argument | Confirm `dir` is the project root containing `.themis/` |
| Invalid `implement.model` accepted at load | `validate` only checks `review.agents.security` and `round3` — `implement.model` is **not** validated | An unsupported value passes `Load` and only surfaces later when handed to `claude --model`. Use `haiku`, `sonnet`, `opus`, or `skip`. |

## What is the Failure Behavior

`Load` returns `(nil, error)` on any of:

- A YAML parse error (including unknown fields due to strict mode).
- An invalid model name in the `review.agents.security` field.
- An invalid `review.round3` value.
- An OS error reading the file (other than `ErrNotExist`).

`Load` returns `(*Profile, nil)` — never an error — when the file is absent.

`Save` returns an error only if directory creation or file write fails.

## Dependencies

| Package | Role |
|---|---|
| `gopkg.in/yaml.v3` | YAML decoding (strict via `KnownFields(true)`) and marshalling |
| `os`, `path/filepath` | File I/O and path construction |
| `bytes`, `errors`, `fmt` | Standard library utilities |

## Related Documents

- `ARCHITECTURE.md` — pipeline stage overview; profile is consumed at runner startup
- `docs/datamodel.md` — persisted data model; `Profile` is not persisted to the database, only to `.themis/profile.yaml`
- `docs/subsystems/runner/README.md` — how the runner reads and applies the loaded `*Profile`
