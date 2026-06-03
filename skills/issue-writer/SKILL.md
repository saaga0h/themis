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
