---
description: Run code review using specialized review agents. Supports full review, scoped review, or individual perspectives (--security, --architecture, --complexity, --conventions, --coverage, --numerical, --quick). In autonomous mode (--autonomous), writes structured findings to .themis/review-results.json for pipeline consumption.
argument-hint: [scope] [--flags]
allowed-tools: Read, Glob, Grep, Bash, Task
---

# Review Command

You orchestrate code reviews by delegating to specialized review agents. Each agent has a focused perspective and runs on the appropriate model.

## Step 0: Parse arguments

Parse `$ARGUMENTS` for:
- **Scope**: a directory path, package name, or `--last-plan` (reviews files from most recently completed plan)
- **Mode**: `--autonomous` (write JSON results for pipeline, skip human interaction)
- **Perspective flags**: which reviewers to run

| Flag | Agent | Model |
|---|---|---|
| `--architecture` | architecture-reviewer | sonnet |
| `--security` | security-reviewer | sonnet |
| `--complexity` | complexity-reviewer | haiku |
| `--conventions` | convention-reviewer | haiku |
| `--coverage` | coverage-reviewer | haiku |
| `--numerical` | numerical-reviewer | sonnet |
| `--quick` | complexity + conventions | haiku |
| (no flags) | all five standard agents | mixed |

If `--last-plan` is specified, find the most recently modified `.md` file in `.claude/plans/` and pass it as scope context to each agent.

## Step 1: Confirm with user

**If `--autonomous` is set: skip this step entirely.**

Show what you're about to do:

"**Code Review**
- **Scope**: <full project | directory | last plan: plan-name>
- **Reviewers**: <list of agents to run>
- **Estimated cost**: <N sonnet + M haiku calls>

Proceed?"

Wait for confirmation.

## Step 2: Gather context

Before running reviewers:
- Read CLAUDE.md (needed by architecture-reviewer and convention-reviewer)
- Read CODING_STANDARDS.md if it exists (pass to convention-reviewer and coverage-reviewer)
- Read UBIQUITOUS_LANGUAGE.md if it exists (pass to convention-reviewer and architecture-reviewer)
- If scoped to a plan, read the plan file to know which files changed
- If scoped to a directory, verify the directory exists

## Step 3: Run reviewers

Delegate to each selected review agent using Task. Pass them:
- The scope (directory, file list from plan, or "full project")
- CLAUDE.md content summary (for architecture and convention reviewers)

Run the haiku agents first (they're faster), then sonnet agents.

**Order**:
1. complexity-reviewer (haiku) — fast structural scan
2. convention-reviewer (haiku) — fast pattern check
3. coverage-reviewer (haiku) — fast coverage map
4. security-reviewer (sonnet) — deeper analysis
5. architecture-reviewer (sonnet) — deepest analysis
6. numerical-reviewer (sonnet) — only when `--numerical` is passed or auto-detected

**Auto-detection for numerical review**: if the diff contains any of the following,
automatically add numerical-reviewer even if `--numerical` was not passed:
- Floating point arithmetic (`float32`, `float64`, `f32`, `f64`, `Float`, `Double`)
- Linear algebra operations (matrix multiply, decomposition, distance, norm)
- Statistical computation (mean, variance, distribution, regression)
- GPU/accelerator kernels (CUDA, ROCm, HIP, Metal, AMDGPU.jl, CUDA.jl)
- Numerical packages (numpy, scipy, LinearAlgebra, BLAS, LAPACK)

Collect each agent's output.

## Step 4: Synthesize and classify findings

Combine all agent outputs. For each finding, assign a severity:

- **CRITICAL** — security vulnerability exploitable in the project's threat model,
  data loss or corruption, compile failure
- **HIGH** — AC not covered (specified behaviour has no test and no implementation),
  abstraction boundary violated (directly contradicts CODING_STANDARDS.md rules)
- **MEDIUM** — untested error path, swallowed error, unthreaded context, hardcoded
  infrastructure value, missing HTTP timeout, cross-internal dependency not listed
  in CODING_STANDARDS.md
- **LOW** — style preferences, "consider" or "could be improved" suggestions,
  performance concerns without benchmark, redundant code that doesn't affect
  correctness, pre-existing patterns not introduced by this change

**Classification rules:**
- Default to MEDIUM when uncertain. Err on the side of catching issues, not deferring them.
- A finding that matches a specific CODING_STANDARDS.md rule is at least MEDIUM.
- A finding that matches a CODING_STANDARDS.md "blocking review finding" rule is HIGH.
- Pre-existing issues not introduced by this change are LOW unless they are security-relevant.

## Step 5: Write results

**If `--autonomous` is set:**

Write findings to `.themis/review-results.json`:

```json
{
  "findings": [
    {
      "severity": "critical",
      "description": "Three git init helpers missing --initial-branch=main",
      "file": "internal/runner/runner_checkpoint_test.go",
      "line": 32,
      "reviewer": "coverage-reviewer"
    },
    {
      "severity": "medium",
      "description": "BranchCommitLog fails in repos without a remote",
      "file": "internal/git/git.go",
      "line": 134,
      "reviewer": "architecture-reviewer"
    },
    {
      "severity": "low",
      "description": "Pre-existing test naming convention inconsistency",
      "file": "internal/git/git_test.go",
      "reviewer": "convention-reviewer"
    }
  ]
}
```

Each finding must have: `severity` (critical|high|medium|low), `description`,
`file`. The `line` field is optional but preferred. The `reviewer` field names
which agent produced the finding.

Write the file using Bash:

```bash
cat > .themis/review-results.json << 'REVIEW_EOF'
<json content>
REVIEW_EOF
```

Verify the file was written:

```bash
cat .themis/review-results.json | python3 -c "import json,sys; d=json.load(sys.stdin); print(f'{len(d[\"findings\"])} findings written')"
```

Do not write to `.claude/reviews/` in autonomous mode.

**If `--autonomous` is NOT set:**

Save the review report to `.claude/reviews/<scope-or-date>.md` as markdown
(existing behaviour). Do not write `.themis/review-results.json`.

## Step 6: Recommendation

**If `--autonomous` is set: skip this step entirely.**

Based on findings, tell the user:

- **READY**: "No blocking issues found. You can run `/ship` to create a PR."
- **NEEDS WORK**: "Found N issues to address. The action items above are ordered by priority."
- **SIGNIFICANT ISSUES**: "Found critical issues that should be resolved before shipping. Consider running `/architect` to plan the fixes if they're non-trivial."

## Important

- Don't duplicate agent work — let each agent do its job and synthesize their outputs
- If an agent finds nothing, that's a good result — say "No issues found" for that section
- The review report on disk should be self-contained — readable without the conversation
- If scoped to `--last-plan`, make sure the plan exists and has completed tasks
- Keep the synthesized report concise — details are in individual agent outputs
- Don't fix anything. Review only. Fixing is `/implement`'s job.
- In autonomous mode: no human interaction. No confirmation, no recommendation.
  Write the JSON and return.

---

## Numerical Reviewer — agent specification

When numerical-reviewer is invoked, it checks the following. These are **blocking
findings** if not satisfied; the `/issue` command's blocking threshold applies.

### 1. Algebraic property assertions

Tests must assert at least one algebraic property beyond shape and type:
- **Symmetry**: `f(a, b) == f(b, a)`
- **Identity**: `f(x, identity) == x`
- **Idempotence**: `f(f(x)) == f(x)`
- **Conservation**: sum/norm/trace preserved under operation
- **Monotonicity**: ordering preserved under transformation
- **Boundary**: result stays within expected range `[lo, hi]`

Shape and type assertions alone (`assert result.shape == (n, m)`) are insufficient
for numerical correctness. At least one property assertion is required.

### 2. Explicit tolerance regime

Every floating point comparison must use one of:
- `atol` (absolute tolerance) — for results near zero or with known absolute error
- `rtol` (relative tolerance) — for results that scale with input magnitude
- `eps` (machine epsilon multiples) — for results bounded by floating point precision
- `"exact"` — for integer arithmetic or operations guaranteed exact by the spec

These are **not interchangeable**. Using `atol=1e-6` when the correct regime is
`rtol=1e-6` will produce false passes at large magnitudes and false failures near
zero. The reviewer checks that the tolerance regime matches the mathematical
properties of the operation.

Vague tolerances (`assert abs(a - b) < 0.001`) are a blocking finding — they
must be replaced with a justified tolerance choice.

### 3. Named reference

Every numerical result must be checked against a named reference:
- **Analytical solution**: a closed-form result derived from the problem specification
- **Known-good implementation**: a trusted library result (e.g. scipy, numpy, Julia LinearAlgebra)
- **Published value**: a result from a paper or standard
- **Characterizing property**: a property that uniquely identifies the correct answer
  (e.g. "the eigenvectors must be orthonormal to within rtol=1e-10")

"The result looks reasonable" is not a named reference and is a blocking finding.

### 4. API pre-flight (GPU/accelerator code)

If the change involves GPU kernels or accelerator-specific packages (AMDGPU.jl,
CUDA.jl, HIP, ROCm), the implementation must list the specific API functions called
and their signatures in a comment or docstring. This prevents API hallucination —
models frequently call functions that do not exist or have wrong argument order in
GPU libraries. The reviewer checks that every called function exists in the package's
public API at the version in use.

### 5. Unit metadata

Every numerical result with physical units must document those units in the
variable name, type annotation, or docstring. Implicit unit assumptions (e.g.
treating km/h as m/s) are a blocking finding.
