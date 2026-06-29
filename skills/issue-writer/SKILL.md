---
name: issue-writer
description: "When creating issues for factory-processed work, follow this structure and these rules."
---
 
# Issue Writing Skill
 
When creating issues for factory-processed work, follow this structure and these rules.
 
## Issue Structure
 
Every issue follows this template:
 
```markdown
# Title
 
## Context
Why this issue exists. What review finding, conversation, or observation triggered it.
Reference the specific file, line number, and standards document if applicable.
 
## What to build
### 1. First deliverable
Specific changes with file paths. Code snippets or interface definitions where they clarify intent.
 
### 2. Second deliverable
...
 
## Acceptance Criteria
- [ ] Concrete, testable criterion — each becomes one or more tests
- [ ] Include rejection/failure cases, not just happy path
- [ ] Reference specific values, status codes, error messages
- [ ] Frontend and backend criteria in the same issue when they must change together
 
## Notes
- Constraints, dependencies, gotchas
- "Do not implement until labeled `ready-for-agent`"
```
 
## Rules for Acceptance Criteria
 
ACs are the most important part of the issue. The factory implements exactly what the ACs say, nothing more. If an AC is vague, the implementation will be vague.
 
### Good ACs
- `CreateItem returns 400 for invalid itemType values` — specific status code, specific condition
- `No string literal "api.finna.fi" remains in handler code` — verifiable with grep
- `Test verifies GetSources response contains the configured host and port` — defines what the test must check
- `409 response body uses "existingItem" not "existingBook"` — exact values, both positive and negative
### Bad ACs
- `Error handling is improved` — improved how? What does the test assert?
- `Tests are added` — for what? What do they verify?
- `Code is cleaned up` — by what standard?
- `Performance is acceptable` — what metric, what threshold?
### Every AC must answer: "What does the test assert?"
If you can't write a test name from the AC, the AC is too vague. The factory will write a test for each AC — if the AC says "error handling works," the test will check "no error" and nothing else.
 
### Kinds of AC — not every AC becomes a runtime test

"What does the test assert?" is the right question for *behavioural* ACs, but some ACs are verified statically or at compile time, not by a runtime test. Name the kind so the factory checks it the right way — you cannot unit-test that a type does *not* exist, because a runtime test for absence has nothing to assert against.

- **Behavioural** — an observable runtime outcome. Becomes a runtime test.
  `CreateItem returns 400 for invalid itemType values`
- **Negative / absence** — something must NOT exist after the change. Becomes a static check (grep / go-analysis), not a runtime test.
  `No GiteaQuerier struct remains in cmd/themis — verify: grep -rn 'type GiteaQuerier' cmd/themis/ returns zero hits`
- **Placement / structural** — a type, interface, or function must live in a specific package or stay at a boundary. Becomes a compile-time guard.
  `The IssueQuerier interface stays in cmd/themis (the consumer), not in internal/tracker`
- **Delegation** — a value must be obtained through a specific path. Becomes a compile-time guard or a behavioural test.
  `cmd/themis constructs the querier via tracker.NewGiteaQuerier, not by composing the struct directly`

Negative, placement, and delegation ACs map to static or compile-time checks. Write them with the verification spelled out (the grep, the package, the constructor) — the same way behavioural ACs spell out the assertion.

### Declaring the check (machine-extractable)

For negative, placement, and delegation ACs, do not stop at describing the verification in prose — **declare it as a runnable command** the factory can extract and run as part of the green gate for this issue's run. Put it in a fenced block tagged `check`, holding a shell command that **exits 0 exactly when the AC is satisfied**:

```check
! grep -rq 'type GiteaQuerier' cmd/themis/
```

The negated `grep` exits 0 when the type is absent — i.e. the move is complete. The factory appends each `check` block to the verify gate for that run only; it is **never committed** (these checks are scaffolding for "done," not permanent tests — see the Refactor/Move/Rename/Delete rules and #81). Rules:

- One `check` block per negative/placement/delegation AC. A deterministic meta-check fails the issue if a destructive ("Removed …") AC has no `check`.
- The command runs via `bash -c` like every verify command — exit 0 on success, non-zero on failure.
- Make it specific and hard to satisfy by accident, and pair it with the behavioural/"added" AC so renaming or hiding the old code cannot pass both.
- Behavioural ACs do **not** get a `check` block — they stay committed runtime tests.

(Strict convention is a deliberate starting point to keep extraction deterministic — observe and tune; see #81.)

## Rules for Exhaustive Enumeration
 
This is the most common source of incomplete fixes. The factory does not generalize — it implements exactly the sites listed. When a fix applies to multiple call sites, every site must be named.
 
### The problem
If an issue says "Add `io.LimitReader` to `auth.go`, `search.go`, and `rss/opac.go` response body reads," the factory will add LimitReader to error-path reads in those files. If there's also an unbounded success-path XML decode in `rss/opac.go:94`, the factory won't touch it unless the AC says so — even though it's the same pattern in the same file.
 
If an issue says "Validate `itemType` in `CreateItem` and `UpdateItem`," the factory will add validation to exactly those two handlers. If `ListItems` also accepts `itemType` without validation, it stays broken — the factory won't generalize from two handlers to a third.
 
### The rule
When writing ACs for a pattern that applies to multiple sites:
 
1. **Search the codebase first.** Before writing the AC, grep for every instance of the pattern. Don't assume you know all the call sites.
2. **List every site explicitly**, or use an unambiguous quantifier with a verification AC:
   - Explicit: "Add LimitReader in `client.go:94` (JSON decode), `auth.go:38` (JSON decode), `search.go:131` (JSON decode), `rss/opac.go:90` (error body), and `rss/opac.go:94` (XML decode)"
   - Quantifier: "Every `json.NewDecoder` and `xml.NewDecoder` call that reads from `resp.Body` must be wrapped with `io.LimitReader`"
3. **Add a sweep AC** when using a quantifier: "No unbounded `resp.Body` reads remain in `internal/koha/` or `internal/finna/` — verify with grep"
4. **Distinguish success and error paths.** "Response body reads are bounded" is ambiguous — the factory may only bound error paths (the `io.ReadAll` calls) and miss success paths (the decoder calls). Say "both success-path decoding and error-path reads."
### Examples
 
Bad: "Add `io.LimitReader` on Koha response bodies"
→ Factory adds it to 3 of 5 paths
 
Bad: "Validate `itemType` in write handlers"
→ Factory validates Create and Update, misses List
 
Good: "Every handler that accepts `itemType` as a query parameter must validate it against `validItemTypes` before passing to the Koha client. Currently: `CreateItem` (line 113), `UpdateItem` (line 170), `ListItems` (line 80)"
→ Factory validates all three
 
Good: "All `json.NewDecoder(resp.Body)` and `xml.NewDecoder(resp.Body)` calls in `internal/koha/` must use `io.LimitReader`. Verify: `grep -rn 'NewDecoder(resp.Body)' internal/koha/` returns zero hits after the change — all should be `NewDecoder(io.LimitReader(resp.Body, ...))`"
→ Factory can verify its own work
 
## Rules for Refactor / Move / Rename / Delete Issues

A move is not done when the new thing exists — it is done when the OLD thing is GONE. The factory implements exactly what the ACs say; if no AC asserts the original's absence, the original survives and the issue is half-done. Duplicate types compile, pass vet, and pass stale tests, so the green gate does not catch it. This is exactly what happened in #68: the queriers were copied into `internal/tracker` but never deleted from `cmd/themis`, and the PR shipped half-done.

For any issue that moves, extracts, renames, or deletes, write the ACs as a **pair**:

- **What is added / moved to** — behavioural or placement AC:
  `tracker.GiteaQuerier exists in internal/tracker and satisfies the IssueQuerier interface`
- **What is removed** — negative / absence AC (this is the one that gets forgotten):
  `No GiteaQuerier type remains in cmd/themis — verify: grep -rn 'type GiteaQuerier' cmd/themis/ returns zero hits`

State explicitly, per moved/renamed/deleted thing: where it now lives, and that the original is gone. A rename needs a negative AC for the old name. "Consolidate A and B into C" needs negative ACs for both A and B. The removal AC is a check the factory must satisfy before the work counts as done — not a politeness.

## Rules for Coordinated Changes
 
When a change requires both backend and frontend updates (e.g., renaming a JSON field), both must be in the same issue. If they're in separate issues, the first one breaks the second one's tests until both merge.
 
Example: renaming `existingBook` to `existingItem` in the 409 response requires updating both the Go handler AND the TypeScript type. One issue, one PR, one merge.
 
## Rules for Scope
 
### One concern per issue
An issue should be about one thing. "Refactor UpdateItem AND add rate limiting AND fix terminology" is three issues pretending to be one. The factory handles them better when separated.
 
### Exception: small related fixes
Multiple small fixes that touch different files and share a theme can be grouped. "Dead code cleanup" with 8 small items across the codebase is fine as one issue — each item is independent and the PR is reviewable as a unit.
 
### When in doubt, split
Two focused issues are better than one sprawling issue. The factory processes them sequentially anyway.
 
## Rules for References
 
### Reference standards documents
If the issue exists because of a CODING_STANDARDS.md or UBIQUITOUS_LANGUAGE.md violation, say so explicitly. The factory reads these documents and will apply them — but only if it knows to look.
 
### Reference specific files and lines
Don't say "the handler." Say "`internal/api/handlers/items.go:59-61`." The factory has to find the code — give it coordinates.
 
### Reference the pattern to follow
If an existing implementation is the template, point to it: "Follow the pattern in `internal/finna/interface.go`." The factory will read that file and replicate the structure.
 
## Rules for Dependencies
 
### State dependencies explicitly
If issue B depends on issue A being merged first, say so in the Notes section: "Depends on #39 being merged first — needs the interface it introduces."
 
### Don't assume the factory reads issue comments
The factory reads the issue body. Comments are for human discussion. If an AC is added in a comment, the factory will miss it. Everything actionable must be in the body.
 
If new requirements emerge after issue creation, either:
1. Edit the issue body to include them (if the issue hasn't been picked up yet)
2. Create a new follow-up issue (if the factory is already working on it)
## Rules for the Notes Section
 
Always end with: `Do not implement until labeled ready-for-agent`
 
Include:
- Dependencies on other issues
- Known constraints ("this is a breaking API change — frontend must update simultaneously")
- What NOT to do ("do not add new hardcoded values following this pattern")
- Scope boundaries ("pre-existing issues in this file are out of scope")
## Labels
 
- `foundation` — core infrastructure or foundational work (applied at creation)
- `ready-for-agent` — factory picks this up (applied when ready to run)
- `needs-review` — PR is ready for human review (applied by factory)
- `blocked` — waiting on a dependency
## Severity as Title Signal
 
The title should hint at the severity and type:
- `Remove hardcoded Finna config from GetSources handler` — fix, specific
- `Test coverage: response layer and transform pipeline` — test gap, scoped
- `Security: LimitReader on success paths, ItemType validation` — security, clear items
- `Terminology: book→item, existingBook→existingItem` — standards compliance, exact changes
Avoid generic titles like "Fix issues" or "Improve code quality."
