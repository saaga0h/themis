---
description: Translate a concept/intent document from web conversation into an architect-ready plan grounded in the actual codebase.
argument-hint: <path to intent document>
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task
---

# Intent Bridge Command

You are a translator between conceptual thinking and executable architecture.
The intent document was produced in a web conversation — it describes *what* and *why*,
not *how*. Your job is to ground that intent in what the codebase actually is,
find the natural attachment points, surface what's still unresolved,
and produce something architect can turn into tasks.

This is NOT architect. Do not produce implementation tasks.
Produce a grounded intent document that architect can receive as input.

---

## Step 0: Read the intent document

The intent document is at: $ARGUMENTS

Read it fully before touching the codebase.
Understand:
- What is the core intent — what should the system understand or do differently?
- What constraints were stated?
- What was explicitly left open or unresolved?
- What questions were still alive at the end of the conversation?

Note: the intent document was written without knowledge of the codebase.
It may use domain language that maps to code in non-obvious ways.
It may assume things that don't exist yet.
It may be solving a problem the codebase has already partially solved differently.

---

## Step 1: Scan the codebase

Delegate to the **codebase-scanner** agent to map the project structure.
Runs on Haiku — no point burning tokens on find and ls.

Also read:
- CLAUDE.md if it exists
- README.md if it exists
- Existing plans in .claude/plans/ — especially any related to this domain
- Any domain-specific packages that the intent document touches

---

## Step 2: Map intent to codebase

For each significant concept in the intent document, find where it lives in the code
— or note that it doesn't exist yet.

Ask:
- What does the codebase already know about this intent?
- Where has this problem been partially addressed?
- What existing patterns are relevant?
- What does the architecture imply about how this wants to be solved?
- Where are the natural attachment points for this intent?
- What would have to change vs what can stay?

Look for:
- Concepts in the intent that map cleanly to existing structures
- Concepts that don't map — genuinely new territory
- Tension between the intent and existing architectural decisions
- Implicit assumptions in the intent that the codebase contradicts

---

## Step 3: Surface unresolved questions

The intent document came from a conversation that was exploring, not concluding.
Some things will still be open. Name them explicitly.

Distinguish between:
- **Questions the codebase answers** — the code already implies a direction
- **Questions that need a decision** — genuine open choices before implementation
- **Questions that need an experiment** — can't be decided without trying

Do not resolve the open questions. Surface them clearly so they can be
decided before architecture begins.

---

## Step 4: Identify risks and unknowns

What in the intent is:
- Technically uncertain — might not work as imagined
- Architecturally risky — conflicts with existing structure
- Scope uncertain — could be small or could be very large depending on unknowns
- Dependent on something not yet built or not yet understood

---

## Step 5: Produce the grounded intent document

Write to `.claude/plans/intent-<descriptive-name>.md`:

```markdown
# Intent: <Descriptive Title>
## Created: <YYYY-MM-DD>
## Source: <path to original intent document>
## Status: grounded — ready for /architect

## Core Intent
<The essential what and why, in one paragraph.
Not how. Not tasks. What the system should understand or do differently
and why that matters.>

## Codebase Attachment Points
<Where this intent connects to existing code.
Specific packages, files, patterns that are directly relevant.
What the existing architecture implies about the right approach.>

## What Already Exists
<Partial solutions, related patterns, relevant infrastructure
already in the codebase that this intent can build on or must work with.>

## What Doesn't Exist Yet
<Genuinely new territory. Things the intent requires that have no
current home in the codebase.>

## Architectural Tensions
<Places where the intent and existing architecture pull in different directions.
Not problems to solve here — tensions to be aware of when planning.>

## Open Questions
### Need a decision before implementation
- <question> — <what the decision affects>

### Codebase already implies an answer
- <question> → <what the code suggests>

### Need an experiment to answer
- <question> — <what would the experiment look like>

## Risks and Unknowns
- <risk> — <why it matters, what to watch for>

## Suggested Scope for Architect
<Not tasks — a framing of what /architect should tackle.
What's the minimal viable implementation of this intent?
What's the full version?
What should be deferred?>

## Constraints from Intent
<Things the intent explicitly said should NOT change,
or hard constraints that implementation must respect.>
```

---

## Step 6: Present and confirm

Show a summary:
- Core intent in one sentence
- How many attachment points found vs new territory
- Key open questions that need decisions
- Biggest risk

Ask:
- "Does this grounding look right?"
- "Are there open questions I missed?"
- "Should any risks change the scope?"

Only write the file after confirmation.

---

## Step 7: Next steps

After saving:
"Grounded intent saved to `.claude/plans/intent-<name>.md`.
Run `/architect .claude/plans/intent-<name>.md` to create the implementation plan."

---

## What this command is NOT doing

- Not producing implementation tasks — that's architect's job
- Not resolving open questions — those need decisions, not code
- Not assuming the intent is correct — if the codebase implies a better approach, say so
- Not describing how to code anything — only where and whether it connects

The output of this command is the input to architect.
The conversation that produced the intent document is the input to this command.
The codebase is the reality both have to respect.
