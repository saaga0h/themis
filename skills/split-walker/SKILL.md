---
name: split-walker
description: "Walk through a resolved design conversation (live or from a markdown file) and decompose it into vertical slices, one topic at a time. For each topic: propose slices, discuss sizing, resolve the decomposition interactively, then create issues using the issue-writer skill. Produces issues ready for the factory pipeline. Use after grill-me has resolved the design, or when handed a markdown summary of a previous grilling session."
---
 
# Split Walker
 
You are decomposing a resolved design into factory-ready issues. The design
decisions are already made — your job is to find the right cuts that turn
them into independently implementable, testable work units.
 
## Input
 
Either:
- A live conversation where grill-me has resolved the design decisions
- A markdown file summarizing a previous grilling session (uploaded or pasted)
Both are valid. A markdown file from a previous session is preferred when the
grilling was long — it preserves the decisions without consuming context budget
on the back-and-forth that produced them.
 
## How you work
 
You do not produce a batch DAG and ask for approval. You walk through the
design topic by topic, interactively, resolving each one before moving on.
 
```
1. Identify the high-level topics from the conversation/file
2. Present the topic list — ask if the ordering and grouping make sense
3. For each topic:
   a. Propose vertical slices with intent, scope, and notes
   b. Discuss with the human — adjust, merge, split, defer
   c. When the topic's slices are settled, create issues using issue-writer
   d. Record cross-topic dependencies for the final summary
4. After all topics: present the full dependency graph and any deferred items
5. Human reviews, then labels issues ready-for-agent when satisfied
```
 
Issues are created without the `ready-for-agent` label. That label is applied
by the human after the full walk is complete and the dependency graph is
reviewed.
 
---
 
## The vertical slice principle
 
The first principle of decomposition is: **every slice should be independently
testable as a vertical cut through the system.**
 
A vertical slice means a user-facing behavior works end-to-end — or at minimum,
there exists a meaningful test (integration or unit) that exercises the slice
in isolation. "Add a database table" is not a vertical slice. "User can create
an item" — touching handler, store, and schema — is.
 
### Slicing priority
 
When decomposing a topic into issues, follow this priority:
 
1. **Vertical slice** — the slice is independently testable end-to-end.
   This is the default. Always try this first.
2. **Smaller vertical slices** — if a vertical slice is too large for the
   factory's context budget, look for a thinner vertical cut. "User can
   create an item" might split into "User can create an item with required
   fields only" and "User can create an item with optional metadata."
3. **Horizontal sequence** — if the vertical slice cannot be made smaller
   and remains too large, split horizontally into a planned sequence of
   rounds. Each round must still have a meaningful test at its own level:
   - Round 1: store method + unit test against mock/test DB
   - Round 2: handler that calls the store + integration test
   - Round 3: frontend wiring + e2e test
   When using horizontal sequencing:
   - The sequence is planned upfront — all rounds are created as issues
   - Each issue states which round it is and what the integration point is
   - Dependencies between rounds are explicit
   - The final round's issue describes the end-to-end test
### What makes horizontal splitting legitimate
 
Horizontal splitting is a fallback, not a failure. It's legitimate when:
- The vertical slice genuinely exceeds the factory's working capacity
- Infrastructure that multiple features share (middleware, service clients,
  message bus setup) — testable on its own as infrastructure
- Schema migrations that are mechanically complex but conceptually simple
Horizontal splitting is NOT legitimate when:
- "The database part" of a feature is split out because it felt like a
  natural boundary — if the only test is "table exists," it's not a slice
- Comfort with horizontal decomposition — AI defaults to layer-by-layer;
  resist this
### The test question
 
For every proposed slice, ask: **"What test can I write for this slice alone?"**
 
- If the answer is an integration test that exercises real behavior → vertical slice, good
- If the answer is a unit test with meaningful assertions → acceptable
- If the answer is "the table exists" or "the function compiles" → not a slice,
  combine with something that produces a testable behavior
---
 
## Sizing
 
The factory operates in a bounded context window. Quality degrades when the
factory must hold too much in context simultaneously. Sizing is a guideline,
not a hard rule — but oversized issues produce worse implementations.
 
### What contributes to context load
 
- The issue body itself (context, deliverables, ACs, notes)
- Source files the factory must read to understand the change
- Standards documents referenced (CODING_STANDARDS.md, UBIQUITOUS_LANGUAGE.md)
- Test files being created or modified
- Pattern files referenced ("follow the pattern in X")
### Sizing heuristics
 
- **File count**: a slice touching 3-4 files is comfortable. 6+ files means
  the factory loads a lot of context before writing a line.
- **Existing patterns**: if a pattern file exists to follow, the factory
  needs less reasoning — larger slices are feasible.
- **Greenfield vs modification**: new files are cheaper than modifying
  existing complex files (less context to load).
- **Coordinated changes**: backend + frontend in one issue is necessary
  when they must change together, but increases context load — keep these
  focused on the coordination point.
### When sizing conflicts with vertical slicing
 
Vertical slicing takes priority. If a vertical slice is slightly large but
represents an irreducible testable behavior, keep it as one issue. Only
split to horizontal when the slice is genuinely too wide for the factory
to handle well — not as a precaution.
 
---
 
## Walking a topic
 
When you reach a topic in the walk, follow this sequence:
 
### 1. Summarize the topic
 
State in 2-3 sentences what was decided about this topic during the grilling.
What's being built, what approach was chosen, what was explicitly ruled out.
 
### 2. Propose slices
 
Present 1-4 vertical slices with this format:
 
```
**Slice: <slug>**
Intent: <what + why in 1-2 sentences>
Scope: <files/packages — rough estimate>
Test: <what test exercises this slice>
Depends on: <other slices or "none">
Notes: <constraints, failure modes, settled decisions>
```
 
The Notes field is the AC quality gate. Ask: "If someone wrote this issue
without reading the conversation, what would they get wrong?" That answer
goes in Notes.
 
Good notes: specific values, error conditions, edge cases, things NOT to do,
references to standards or pattern files.
Bad notes: "needs tests," "should be clean," vague restatements of intent.
 
### 3. Discuss
 
Wait for the human's response. They may:
- Agree with the proposed slices
- Merge two slices that are too fine
- Split a slice that's too large
- Defer a slice that needs more thinking
- Add constraints you missed
- Challenge whether a slice is truly vertical
Adjust until the topic's decomposition is settled.
 
### 4. Create issues
 
When the human confirms the slices for this topic, create the issues using
the issue-writer skill structure. Each issue gets:
 
- **Context** — from the slice's intent and the conversation's reasoning
- **What to build** — specific deliverables with file paths, code snippets
  where they clarify intent
- **Acceptance Criteria** — testable assertions derived from the Notes field.
  Every AC must answer "what does the test assert?"
- **Notes** — constraints, dependencies, "Do not implement until labeled
  `ready-for-agent`"
If the slice is part of a horizontal sequence, the issue body states:
- Which round this is (e.g., "Round 2 of 3")
- What the previous round produced (with issue reference)
- What test level this round targets (unit / integration / e2e)
- What the end-to-end test looks like (in the final round's issue)
Create via MCP (Gitea or GitHub) in the same conversation. Record the
issue numbers for the dependency graph.
 
### 5. Move on
 
Confirm the issues are created, note any cross-topic dependencies discovered,
and proceed to the next topic.
 
---
 
## After the walk
 
Once all topics are processed:
 
1. **Present the full dependency graph** — all issues, all dependencies,
   showing which can be processed in parallel and which must be sequential.
2. **List deferred items** — topics or slices that weren't ready, with a
   note on what's missing before they can become issues.
3. **List open questions** — anything unresolved that surfaced during the walk.
4. **Wait for the human to label issues `ready-for-agent`** — this is their
   decision, not yours. They may want to re-order, adjust dependencies, or
   hold certain issues.
---
 
## Rules
 
### Split by independent acceptance, not by topic
 
"Authentication" is a topic. "Add JWT validation to the session middleware"
is an issue. Two things belong in the same issue if merging one without the
other leaves the codebase broken. Two things belong in different issues if
they can merge independently.
 
### Record dependencies as a graph, not a list
 
If issue B requires issue A's code, that's `depends_on: [A]`. An ordered
list has no semantics. A dependency graph does. Two independent issues can
be processed in parallel — ordering loses that information.
 
### Collapse over-fine splits
 
If two slices touch the same file in overlapping ways, merge them. If a
developer could implement both in the same sitting without context switching,
they belong together.
 
### Flag readiness honestly
 
- **ready** — intent is clear, scope is known, ACs can be written now
- **needs-thinking** — approach isn't settled; defer until a decision is made
- **deferred** — explicitly decided not to do now; record why
Do not force speculative ideas into slice form. A deferred item with a clear
note on what's missing is more useful than a vague issue.
 
### Notes field is the AC quality gate
 
A slice with "Pure function, no LLM calls, Runtime defaults to go when empty,
ResponseFields render with unit metadata to prevent wrong conversions"
produces ACs like:
- "`RenderCodingPrompt` with empty Runtime renders `go`"
- "ResponseFields render as `Name: Type (Unit) → MapsTo`"
- "No import of any LLM or HTTP package"
A slice with "needs tests" produces ACs like:
- "Function is tested"
The difference is entirely in the Notes.
 
### Coordinated changes stay together
 
When backend and frontend must change together (renamed field, new API
contract), they are one issue. Splitting them means the first PR breaks
the second's tests.
 
### Before marking needs-thinking, check the codebase
 
If an open question can be answered by reading the code — existing patterns,
current interfaces, file structure — read the code instead of deferring.
The codebase is available; use it.
 
---
 
## What not to do
 
- **Do not produce a batch DAG and ask for approval.** Walk topic by topic.
- **Do not revisit design decisions.** Those were resolved during grilling.
  If something seems wrong, flag it — don't silently re-decide.
- **Do not default to horizontal splitting.** Vertical first, always.
  Horizontal is a planned fallback, not a comfort zone.
- **Do not create issues with vague ACs.** If the Notes aren't rich enough
  to produce specific ACs, the slice needs more discussion — go back to
  step 3.
- **Do not split smaller than a mergeable unit.** A 50-line change to a
  single function is not an issue — it's part of one.
- **Do not skip the test question.** Every slice must answer "what test
  exercises this slice alone?" before becoming an issue.
---
 
## Example: walking a topic
 
**Topic: Recipe search with allergen filtering**
 
Summary: The grilling settled on a two-phase approach — first implement
basic ingredient search against the existing corpus, then add allergen
filtering using the Neo4j FoodOn labels. Search uses embedding similarity.
Allergen filtering is a post-retrieval filter, not a query constraint.
 
Proposed slices:
 
**Slice: recipe-ingredient-search**
Intent: Add ingredient-based recipe search so users can find recipes by
what they have, using embedding similarity against the Finnish corpus.
Scope: `internal/search/ingredient.go` (new), `internal/search/ingredient_test.go`,
`internal/api/handlers/search.go` (new endpoint)
Test: Integration test — POST `/api/search` with `{"ingredients": ["tomaatti", "sipuli"]}`
returns recipes ranked by similarity, top result contains both ingredients.
Depends on: none
Notes: Embeddings are pre-computed (qwen3-embedding:8b, 4096-dim). Search
is cosine similarity against stored vectors — do not re-embed at query time.
Finnish compound ingredients ("tomaattikastike") must match component
ingredients. Return top 10 by default, configurable via query param.
Handler follows pattern in existing `handlers/sources.go`.
 
**Slice: recipe-allergen-filter**
Intent: Add allergen post-filter to recipe search results so users with
allergies get safe results, using the materialized Neo4j FoodOn labels.
Scope: `internal/allergen/filter.go` (new), `internal/allergen/filter_test.go`,
modification to search handler response pipeline
Test: Integration test — search with `{"ingredients": ["kala"], "exclude_allergens": ["CitrusFamily"]}`
returns no recipes containing citrus ingredients. Unit test — filter function
with known recipe ingredients returns correct include/exclude decisions.
Depends on: recipe-ingredient-search
Notes: Three materialized labels exist: CitrusFamily, TreeNutAllergen,
PeanutAllergen. Filter is post-retrieval — run search first, then filter
results. Do not modify the search query. A recipe is excluded if ANY of its
ingredients match ANY excluded allergen label. Neo4j query pattern:
`MATCH (i:FoodItem)-[:subClassOf*]->(a:CitrusFamily)`. Empty allergen list
means no filtering — return all results unchanged.
