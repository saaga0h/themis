---
name: review-walker
description: "Walk through a code review report, triage findings by severity, discuss options for high-stakes items with the user, then create issues using the issue-writer skill. Use when the user provides a review report or says something like 'let's process these review findings' or 'walk me through the review.'"
---
 
# Review Walker Skill
 
Take a code review report as input, walk through each finding with the user, and produce factory-ready issues.
 
## Step 1: Ingest and Classify
 
Read the review report. For each finding, classify into:
 
- **Critical / Blocking** — standards violations, security issues, missing test coverage for ACs
- **High** — security gaps, significant coverage holes, architectural drift
- **Medium** — refactoring, consolidation, complexity reduction
- **Low** — style, comments, dead code, cosmetic
Present the classified list as a summary table to the user before diving in.
 
## Step 2: Check Provenance
 
Before discussing any critical or high finding, check whether it was **introduced by recent changes** or is **pre-existing**. This matters because:
 
- If the factory introduced it, coding standards or issue ACs may need tightening
- If it's pre-existing, it's backlog work — important but not a process failure
To check provenance:
1. Look at the git history for the file and line range mentioned in the finding
2. Identify the commit that introduced the pattern (factory PR, manual commit, or original code)
3. Report: "This is pre-existing from [commit/date]" or "This was introduced in PR #XX (issue #YY)"
If multiple findings need provenance checks, batch them efficiently. Present results to the user before proceeding.
 
## Step 3: Walk High-Stakes Findings
 
For each critical and high finding, walk through it with the user. The goal is to reach a decision on **what to do** and **how to scope it** before creating an issue.
 
### When to ask questions
 
Ask the user when a finding involves:
 
- **Multiple valid approaches**: "The Discogs cover URLs bypass the SSRF allowlist. Two options: (A) route through `validateCoverURL` and add Discogs hosts to the allowlist, or (B) add a separate validation function for Discogs URLs. Which approach?"
- **Scope decisions**: "The `itemType` validation is missing from `ListItems`. Should this be its own issue, or added to the existing security issue #64 as a follow-up?"
- **Breaking changes**: "Renaming this JSON field is a breaking API change. Should we coordinate frontend+backend in one issue, or version the API?"
- **Tradeoffs**: "Adding LimitReader to the XML decode path could reject legitimate large RSS feeds. Should we use the same 10 MiB limit as JSON, or a different one?"
- **Process implications**: "This gap was caused by an incomplete AC in issue #64 — the AC said 'response body reads are bounded' without specifying success vs error paths. Should we update the issue-writing skill to prevent this?"
### When NOT to ask questions
 
Don't ask when the finding is:
 
- **Straightforward with one obvious fix**: "Delete empty `internal/koha/interface.go`" — just note it, no discussion needed
- **Already decided by standards**: "CODING_STANDARDS.md says no hardcoded infra values" — the standard decides, not the user
- **A direct enumeration**: "These 5 files need LimitReader added" — no design decision, just work
### How to present options
 
For each option:
1. State what it does
2. State what it prioritizes (security, simplicity, backward compatibility)
3. State what it trades off
4. Let the user choose
Do not recommend unless asked. Present the options neutrally — the user knows their project's priorities better than the skill does.
 
## Step 4: Group into Issues
 
Once all findings have been discussed, group them into issues. Grouping rules:
 
- **Same concern, same files**: group together (e.g., "add LimitReader to 3 remaining paths in `internal/koha/`")
- **Same concern, different files**: group if the changes are independent and the PR is reviewable (e.g., "terminology fixes across 5 files")
- **Different concerns**: separate issues, even if they touch the same file
- **Coordinated changes**: backend + frontend in one issue when they must merge together
Present the proposed grouping to the user before creating issues: "I'd group these into N issues: [list]. Does that look right?"
 
## Step 5: Create Issues
 
For each agreed group, create an issue following the issue-writer skill:
 
- Use exhaustive enumeration for patterns that apply to multiple sites
- Include sweep ACs where applicable ("verify with grep that no instances remain")
- Reference the specific review finding and standards document
- Check provenance — if the finding was introduced by a factory PR, note which issue's ACs were incomplete
- End Notes with "Do not implement until labeled `ready-for-agent`"
After creating all issues, present the recommended execution order with rationale (blocking issues first, wide changes alone, test-only changes can parallelize).
 
## Step 6: Process Check
 
After the walk-through is complete, ask:
 
- "Were any findings caused by incomplete ACs in previous issues?" — if yes, discuss whether the issue-writing skill needs updating
- "Were any findings caused by gaps in CODING_STANDARDS.md?" — if yes, suggest additions
- "Should any of these be labeled `ready-for-agent` now, or are there dependencies?"
## Calibration Rules
 
These encode review judgment from past sessions:
 
- **Standards violations are always blocking.** A CODING_STANDARDS.md violation is not "medium" because the fix is small. The severity is about the violation, not the effort.
- **Pre-existing ≠ unimportant.** Pre-existing issues still need issues and fixes. They just don't indicate a process failure.
- **The factory doesn't generalize.** When grouping findings into issues, enumerate every site. "Fix the remaining LimitReader gaps" is not an AC — "Add LimitReader at `opac.go:94` (XML decode success path)" is.
- **Comments don't reach the factory.** Everything actionable goes in the issue body. If a finding needs discussion before it becomes an AC, have the discussion here, then write the AC.
- **Test existence ≠ test quality.** If a test exists but only asserts "no error," that's a coverage gap worth a finding, not a pass.
- **Continuous improvement.** Every review round should make the next one cleaner. If the same class of finding keeps appearing, the standards or skills need updating, not just the code.
