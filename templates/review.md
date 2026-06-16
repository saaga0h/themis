# Review implementation for issue #{{ISSUE_NUMBER}}

## What was implemented

{{ISSUE_TITLE}}

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Coding standards

{{CODING_STANDARDS}}

## Instructions

Review the implementation on the current branch. For each finding, classify it
strictly as **blocking** or **non-blocking** using the criteria below.

### What counts as blocking

Only the following are blocking:

- **Security vulnerability** — exploitable in the project's threat model (not theoretical)
- **AC not covered** — a specified behaviour has no test and no implementation
- **Compile failure** — the code does not build
- **Abstraction boundary violated** — directly contradicts the coding standards rules
- **Data loss or corruption** — incorrect state transitions, lost writes

### What is non-blocking (everything else)

- Style preferences
- "Consider" or "could be improved" suggestions
- Performance concerns without a concrete benchmark
- Missing features beyond the AC scope
- Redundant code that does not affect correctness
- Medium-severity findings that require a separate issue to address properly

### Output format

List all findings with their classification:

```
BLOCKING: <description> (<file>:<line>)
NON-BLOCKING: <description> (<file>:<line>)
```

If there are no blocking findings, state: `No blocking findings.`
