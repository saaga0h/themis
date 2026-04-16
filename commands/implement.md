---
description: Execute an implementation plan. Reads plan from .claude/plans/, asks which model for main work, delegates testing and docs to appropriate agents.
argument-hint: [plan-name]
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task
---

# Implement Command

You execute implementation plans created by `/architect`. You ask the user which model to use for the main implementation work, then handle everything including testing and documentation updates.

## Step 0: Find the plan

If `$ARGUMENTS` is provided, read `.claude/plans/$ARGUMENTS.md`
If not, delegate to the **plan-reader** agent to list available plans and ask the user which one to execute.

If no plans exist, tell the user to run `/architect` first.

## Step 1: Ask the user

"Which model for the main implementation work?
1. **Sonnet** — Moderate complexity, needs some judgment
2. **Haiku** — Straightforward, following clear patterns
3. **Opus** — Complex logic, needs deep reasoning

Testing, docs, and context updates are handled automatically by specialized agents — you don't need to think about those."

Wait for their answer before proceeding.

## Step 2: Read context

- Read the plan file fully
- Read CLAUDE.md if it exists (for build commands, constraints, gotchas)
- Understand current state and what's already done (check for [x] marks)
- If the plan has a `## Docs` section with paths listed: read those files now.
  These are the pre-resolved scoped docs for this plan — read them all, they are minimal by design.
  Do not scan docs/ beyond what is listed here.

## Step 3: Execute tasks sequentially

For each unchecked task in the plan:

### Before the task
- Read the target file(s)
- Read any referenced pattern files
- Understand what exists before changing it

### Do the task
- Implement exactly what the plan says
- Follow the pattern references from the plan
- If something in the plan doesn't make sense given the actual code, STOP and ask the user rather than guessing

### After each task
- If the task specifies a test, run it immediately by delegating to the **test-runner** agent
- If the test fails, attempt to fix. If the fix isn't obvious, STOP and report to user

### Refactor (while tests stay green)
- Review the code just written for: duplication, naming, readability
- Make improvements. Delegate to **test-runner** after EACH change.
- If any test turns red, revert the refactor immediately — do not fix forward
- Only mark the task `[x]` complete after tests are green AND a refactor pass is done

### If stuck
- Don't guess. Don't improvise outside the plan scope.
- Report what's wrong and ask the user for guidance
- The plan may have been written before seeing the actual code state

## Step 4: Run full test suite

After all tasks are complete, delegate to the **test-runner** agent to run the full test suite.

If tests fail:
- Check if failures are related to your changes
- Fix if straightforward
- Report to user if not obvious

## Step 4b: Verify Against Spec

If a `.claude/ac/<plan-name>.md` file exists:
- Read it
- For each criterion, confirm a test exists that covers it (search test files for the criterion's key behavior) and that test is currently passing
- Report as a table:

```
| AC # | Criterion | Test exists | Passes |
|------|-----------|-------------|--------|
| 1    | <title>   | Yes/No      | Yes/No |
```

If any criterion has no test or a failing test:
- Flag it explicitly in the final report
- Do not mark implementation complete without user acknowledgement

## Step 5: Update CLAUDE.md (only if needed)

Check if the plan's "New Constraints or Gotchas" section has content. Also check if any of the following changed:
- Build or test commands
- External dependencies or tooling requirements
- Protocol contracts or message formats
- Environment requirements

**If yes**: delegate to the **context-updater** agent.
**If no**: skip — don't update CLAUDE.md just because code changed. The code speaks for itself.

## Step 6: Update documentation

Check if README.md or other user-facing docs need updates:
- If the plan's changes affect anything documented in README, delegate to the **doc-updater** agent
- If unsure, ask the user

## Step 7: Final report

```
## Implementation Complete

**Plan**: <plan name>
**Tasks**: <X/Y completed>

### What was done
<brief summary of changes>

### Test Results
<pass/fail summary from test-runner>

### Spec Verification
<AC table from Step 4b, or "No AC file found">

### Documentation Updated
<what was updated, or "No updates needed">

### Files Changed
<list of modified files>
```

## Error recovery

If a session dies mid-implementation:
- The plan file tracks progress via [x] checkboxes
- Running `/implement <same-plan>` again will pick up from the first unchecked task
- No work is lost, no tasks are repeated

## Important

- Stay within the plan's scope. Don't improve things that aren't in the plan.
- The plan is the contract. If reality differs from the plan, ask the user.
- Each task should result in a working state — don't leave things half-done between tasks.
- Commit early and often if the project uses git.
