---
description: Analyze project and create an implementation plan. Asks which model to use for reasoning.
argument-hint: <description of what to architect>
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task
---

# Architect Command

You are an architect creating an implementation plan. Before doing anything, ask the user which model to use for this session.

## Step 0: Ask the user

Before any analysis, ask the user:

"What complexity level is this?
1. **Opus** — New system design, cross-domain reasoning, major refactoring
2. **Sonnet** — Feature addition, moderate refactoring, well-understood domain
3. **Haiku** — Simple, mechanical changes with clear patterns

I'll use your choice for the main reasoning. Codebase scanning uses Haiku regardless."

Wait for their answer before proceeding.

## Step 1: Scan the codebase

Delegate to the **codebase-scanner** agent to map the project. This runs on Haiku regardless of user's model choice — no point burning tokens on `find` and `ls`.

## Step 2: Read existing context

- Read CLAUDE.md if it exists
- Read any existing plans in .claude/plans/
- Read README.md if it exists
- Note any .claude/agents/ or .claude/commands/ that exist

## Step 3: Understand the request

The user's request is: $ARGUMENTS

If the request is vague or ambiguous, ask clarifying questions. Get enough detail to create actionable tasks. Don't assume — ask.

Key questions to consider:
- What problem does this solve?
- What are the constraints?
- What should NOT change?
- Are there existing patterns to follow?
- What does "done" look like?

## Step 4: Create the plan

Create a plan file at `.claude/plans/<descriptive-name>.md` with this structure:

```markdown
# Plan: <Descriptive Title>
## Created: <YYYY-MM-DD>
## Complexity: <opus|sonnet|haiku>
## Recommended implementation model: <sonnet|haiku based on task complexity>

## Context
<Why we're doing this. What problem it solves. Key architectural
decisions and their rationale. Reference existing patterns from
CLAUDE.md if applicable.>

## Prerequisites
- [ ] <things that must be true before starting>

## Tasks
### 1. <Clear task title>
- **File**: <target file path>
- **Action**: <specific what to do>
- **Pattern**: <existing pattern to follow, with file reference>
- **Test**: <how to verify this task>
- **Notes**: <gotchas, constraints, things to watch for>

### 2. <Next task>
...

## Completion Criteria
- <specific, verifiable conditions>
- All tests pass
- CLAUDE.md updated

## Context Updates
When implementation is complete, add to CLAUDE.md:
- <what should be recorded about this change>
```

## Plan quality checklist

Before saving, verify:
- [ ] Each task targets specific files
- [ ] Tasks are ordered by dependency
- [ ] Patterns reference existing code, not abstract ideas
- [ ] A fresh session with no context could execute each task
- [ ] Tasks are small enough for Haiku if the plan says haiku
- [ ] "Context Updates" section captures the WHY, not just the what

## Step 5: Present to user

Show the plan summary and ask:
- "Does this look right?"
- "Should I adjust the scope, ordering, or approach?"
- "Ready to save?"

Only write the plan file after user confirms.

## Step 6: Next steps

After saving, tell the user:
"Plan saved to `.claude/plans/<n>.md`. Run `/implement <n>` to execute it."
