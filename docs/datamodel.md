# Data Model

<!-- @tier: 1 -->
<!-- @source: internal/pipeline/, internal/runner/, internal/profile/ -->

## Overview

Themis has no database. All persisted state lives in three files under `.themis/` in the repository working directory. They are written and read by the runner on every pipeline execution; they are not committed to git.

```
.themis/
├── state.json          # PipelineState — step cursor, history, counters
├── review-results.json # ReviewResults — structured findings from the review agent
└── profile.yaml        # Profile — per-project pipeline configuration
```

All three files are scoped to a single issue run: `state.json` and `review-results.json` are keyed by `IssueNumber`; `profile.yaml` is project-wide and survives across runs.

## `.themis/state.json`

Written by `pipeline.SaveState` (called after every step) and read by `pipeline.LoadState` at startup. Encoded as indented JSON, mode `0600`, directory mode `0700`.

### `PipelineState` fields

| Field | JSON key | Go type | Meaning |
|-------|----------|---------|---------|
| `IssueNumber` | `IssueNumber` | `int` | Issue number this run is for. Mismatch on load → discard and start fresh. |
| `CurrentStep` | `CurrentStep` | `Step` (int) | Step to execute next. |
| `ReviewCycle` | `ReviewCycle` | `int` | Number of completed review+fix cycles in this run. |
| `MaxReviewCycles` | `MaxReviewCycles` | `int` | Cycle limit before blocking (default `2`; absolute cap is `3`). |
| `TestFixAttempts` | `TestFixAttempts` | `map[string]int` | Per-AC-key count of failed TestRed retries. Key is `cfg.TestACKey` (falls back to `"tests"`). Limit is 3 per key. |
| `Commits` | `Commits` | `[]string` | Reserved. Declared on the struct but not currently populated by the runner — commit detection happens via `internal/git` snapshots, not this field. |
| `StartedAt` | `StartedAt` | `time.Time` | Wall-clock time the run was initialized. |
| `StepHistory` | `StepHistory` | `[]StepResult` | Appended after each step completes via `recordStep`. |
| `CodeVersion` | `CodeVersion` | `string` | Value of `cfg.CodeVersion` at run start. Version mismatch on load → warning only, run continues. |

### `Step` enum

Stored as an integer. Values in pipeline order:

| Value | Name | String |
|-------|------|--------|
| 0 | `StepFetch` | `"Fetch"` |
| 1 | `StepScan` | `"Scan"` |
| 2 | `StepBranch` | `"Branch"` |
| 3 | `StepTestRed` | `"TestRed"` |
| 4 | `StepImplement` | `"Implement"` |
| 5 | `StepRefactor` | `"Refactor"` |
| 6 | `StepReview` | `"Review"` |
| 7 | `StepFix` | `"Fix"` |
| 8 | `StepDocs` | `"Docs"` |
| 9 | `StepShip` | `"Ship"` |

### `StepResult` fields

Appended to `StepHistory` after each step; also returned by `Advance` to drive the next step.

| Field | Go type | Meaning |
|-------|---------|---------|
| `Success` | `bool` | Whether the step completed successfully. |
| `BlockingFindings` | `bool` | Set by the Review step when blocking findings were found in `review-results.json`. |
| `Round3Trigger` | `Round3Trigger` (int) | Reason a third review cycle is permitted. |
| `TestACKey` | `string` | AC key that failed; used to gate retry count in `TestFixAttempts`. |

### `Round3Trigger` enum

| Value | Name | Meaning |
|-------|------|---------|
| 0 | `TriggerNone` | No round-3 extension. |
| 1 | `TriggerSecurity` | Security finding warrants a third cycle. |
| 2 | `TriggerPublicAPI` | Public API concern warrants a third cycle. |
| 3 | `TriggerNumerical` | Numerical correctness concern warrants a third cycle. |
| 4 | `TriggerContextArtifact` | Context artifact concern warrants a third cycle. |

### Lifecycle

| Event | Effect on `state.json` |
|-------|----------------------|
| Fresh start (no file, or `IssueNumber` mismatch) | File removed; new `PipelineState` initialized with `IssueNumber`, `CurrentStep=StepFetch`, `MaxReviewCycles=2`, empty `TestFixAttempts`. |
| `CodeVersion` mismatch | Warning logged to stderr; existing state used as-is. |
| After every step | `SaveState` overwrites the file. |
| After `StepShip` | File is **not** deleted; final state is preserved for inspection. |

## `.themis/review-results.json`

Written by the review agent following the `review.md` prompt template. Read by `cfg.ReviewResultsLoader` (defaulting to `review.ReadReviewResults`) after the Review step completes. Encoded as flat JSON, mode `0644`.

### `ReviewResults` schema

```
{
  "findings": [ <ReviewFinding>, ... ]
}
```

### `ReviewFinding` fields

| Field | JSON key | Go type | Required | Meaning |
|-------|----------|---------|----------|---------|
| `Severity` | `severity` | `string` | yes | One of `"critical"`, `"high"`, `"medium"`, `"low"`. |
| `Description` | `description` | `string` | yes | Human-readable finding text. |
| `File` | `file` | `string` | no (`omitempty`) | Source file the finding refers to. |
| `Line` | `line` | `int` | no (`omitempty`) | Line number within `File`. |

### Severity → blocking mapping

| Severity | Blocking |
|----------|----------|
| `critical` | yes |
| `high` | yes |
| `medium` | yes (`review.BlockingThreshold = "medium"`) |
| `low` | no |

Any blocking finding (`blocking > 0`) sets `StepResult.BlockingFindings = true`, which causes `Advance` to increment `ReviewCycle` and route to `StepFix` instead of `StepDocs`. Blocking determination is performed by `review.DetermineBlockingStatus` in `internal/review`.

If `review-results.json` is absent after the Review step, the runner treats the outcome as blocking (fail-safe).

### Lifecycle

| Event | Effect on `review-results.json` |
|-------|--------------------------------|
| `StepBranch` (fresh start only) | File seeded with `{"findings":[]}` so Review is non-blocking by default. |
| Fresh start (`IssueNumber` mismatch or no state) | File deleted with `os.Remove` before the new state is initialized. |
| After `StepShip` | File deleted with `os.Remove`. |
| Review agent invocation | Agent overwrites the file with its structured output per `review.md`. |

## `.themis/profile.yaml`

Per-project pipeline configuration. Read by `profile.Load` at run start. Absent file → all defaults applied. Parsed with `yaml.NewDecoder` + `KnownFields(true)`: unknown YAML keys are a fatal parse error.

Written by `profile.Save` (mode `0600`, directory `0700`). The file is optional; the pipeline runs without it.

### Full schema

```yaml
review:
  agents:
    security:     <model>   # default: sonnet
    architecture: <model>   # default: sonnet
    complexity:   <model>   # default: haiku
    conventions:  <model>   # default: haiku
    coverage:     <model>   # default: haiku
    numerical:    <model>   # default: sonnet
    depth:        <model>   # default: sonnet
  round3:          <auto|always|never>  # default: auto
  round3_surfaces: []       # default: empty

implement:
  model:             <model>  # default: sonnet
  test_fix_attempts: <int>    # default: 3

refactor:
  enabled: <bool>   # default: true

docs:
  enabled:    <bool>   # default: true
  skip_tiers: []       # default: empty

blocking:
  includes: []         # default: [security, ac-coverage, compile-failure, data-loss, abstraction-boundary]
```

### Field reference

| YAML path | Go field | Default | Valid values / notes |
|-----------|----------|---------|----------------------|
| `review.agents.security` | `Profile.Review.Agents.Security` | `"sonnet"` | `haiku`, `sonnet`, `opus`, `skip` |
| `review.agents.architecture` | `Profile.Review.Agents.Architecture` | `"sonnet"` | `haiku`, `sonnet`, `opus`, `skip` |
| `review.agents.complexity` | `Profile.Review.Agents.Complexity` | `"haiku"` | `haiku`, `sonnet`, `opus`, `skip` |
| `review.agents.conventions` | `Profile.Review.Agents.Conventions` | `"haiku"` | `haiku`, `sonnet`, `opus`, `skip` |
| `review.agents.coverage` | `Profile.Review.Agents.Coverage` | `"haiku"` | `haiku`, `sonnet`, `opus`, `skip` |
| `review.agents.numerical` | `Profile.Review.Agents.Numerical` | `"sonnet"` | `haiku`, `sonnet`, `opus`, `skip` |
| `review.agents.depth` | `Profile.Review.Agents.Depth` | `"sonnet"` | `haiku`, `sonnet`, `opus`, `skip` |
| `review.round3` | `Profile.Review.Round3` | `"auto"` | `auto`, `always`, `never` |
| `review.round3_surfaces` | `Profile.Review.Round3Surfaces` | `[]` | List of surface names; semantics defined by review templates |
| `implement.model` | `Profile.Implement.Model` | `"sonnet"` | `haiku`, `sonnet`, `opus`, `skip` |
| `implement.test_fix_attempts` | `Profile.Implement.TestFixAttempts` | `3` | Max retries per AC key before the pipeline errors |
| `refactor.enabled` | `Profile.Refactor.Enabled` | `true` | `*bool`; nil = file absent, treated as `true` |
| `docs.enabled` | `Profile.Docs.Enabled` | `true` | `*bool`; nil = file absent, treated as `true` |
| `docs.skip_tiers` | `Profile.Docs.SkipTiers` | `[]` | List of doc tier numbers to skip |
| `blocking.includes` | `Profile.Blocking.Includes` | `["security","ac-coverage","compile-failure","data-loss","abstraction-boundary"]` | Reserved. Declared and defaulted, but the runner does **not** read it — blocking is currently decided by a hardcoded severity threshold (`blockingThreshold = "medium"`) in `internal/runner`. Not yet wired. |

### Validation

`profile.validate` rejects:

- Any `review.agents.*` value not in `{haiku, sonnet, opus, skip}` (empty string is also valid before defaults are applied).
- Any `review.round3` value not in `{auto, always, never}` (empty string valid before defaults).
- Any unknown YAML key (enforced by `KnownFields(true)`).

Defaults are applied by `applyDefaults` after validation, so an empty string in any agent field is replaced by its default model value.

### Step-to-model mapping at runtime

`modelForStep` in `runner.go` maps steps to profile fields:

| Step | Model field used |
|------|-----------------|
| `StepReview` | `Profile.Review.Agents.Security` |
| All other agent steps | `Profile.Implement.Model` |

## Persisted vs Computed

| Data | Where | When |
|------|-------|------|
| `state.json` | `.themis/state.json` | Persisted; written after every step |
| `review-results.json` | `.themis/review-results.json` | Persisted; written by review agent, read by runner |
| `profile.yaml` | `.themis/profile.yaml` | Persisted; read once per run, never mutated by runner |
| `lastBlockingFindings` | in-memory `string` in `runner.Run` | Computed from `review-results.json`; injected into Fix/Ship templates as `{{BLOCKING_FINDINGS}}` |
| `reviewOutput` | in-memory `string` in `runner.Run` | Agent stdout from the Review step; injected into subsequent templates as `{{REVIEW_OUTPUT}}` |
| `StepHistory` | embedded in `state.json` | Persisted; accumulated `[]StepResult` for the run |

## Related Documents

- `ARCHITECTURE.md` — component inventory and system structure
- `go doc ./internal/pipeline` and `go doc ./internal/checkpoint` — the state machine and per-step verification, from the code
