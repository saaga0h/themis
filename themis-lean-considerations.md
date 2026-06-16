# Themis Lean Pass — Considerations Brief

## How to use this

This is a map of what to check and what to avoid — not a plan, and not ground truth. Claude Code has repo access; verify every claim here against the actual files before acting on it. I read `README.md` and `commands/issue.md` in full; everything about `review.md`, `ship.md`, `factory.md`, the reviewer agents, and the current skills is inferred from those plus file sizing, and is flagged where it matters.

The brief deliberately does not restate the repo. Pointing at the source instead of copying it is the same discipline the lean pass is meant to enforce — so the document models the thing it's asking for.

## What's working — guardrails, don't break these

The agents are already single-responsibility and lean (none over ~165 lines). The weight is in the commands. Lean there, not in the agents — and resist collapsing agent logic up into a command to reduce file count, because that destroys the model-pinning cost benefit and the cross-command reuse that make the agents worth having.

The tiered context model — minimal always-loaded `CLAUDE.md`, on-demand `docs/` via the scope resolver — is sound. It's the principle the rest of the lean should conform *to*, not a target.

The RED→GREEN→refactor→review pipeline with explicit CHECKPOINT steps (`issue.md` 4d, 8e) is reliability scaffolding for headless runs, not bloat. See the run-script risk below before touching it.

The two-tier review gate — factory ships on a high blocking bar and notes the rest, human is the strict gate — is intentional and principled, not a drift to be tidied. It rests on a bounded-attempts-then-surface policy (detailed in finding 2): more autonomous rounds can't fix a wrong-approach problem, so the factory stops and hands the survivors to the human. Preserve the gate and the surfacing; treat any change to either as a behavioral decision.

The system is designed for failure: primitives drop-and-log and move on rather than retry in-loop, the factory branches from fresh main and is idempotent on re-run, and recurring failures are read as signal (a standard to clarify, a skill to tune) rather than ground to grind on. This works only because three supports hold together — units are atomic (a drop leaves done or not-done, never half-done), re-runs converge, and the failure trail is observed (`blocked` labels, issue comments, Review Notes, structured logs). The trail is observability, not noise. A lean pass must not quiet it: do not collapse `blocked`-label-plus-comment-plus-stop into a silent stop, and do not trim structured error logging in primitives as verbosity. Failing cleanly and *loudly* is the design; a quiet failure is silent data loss.

## Governing principles for the lean

**Single source.** Each contract, rule, or fact lives in exactly one place; everything else references it. Most of the bloat in this system is one principle re-derived in several files, not dead content — so the dominant move is *consolidate and reference*, not delete.

**Cost is per-successful-outcome, not per-token.** A shorter, more ambiguous prompt that triggers an extra fix cycle is more expensive, not less. The issue-writer enumeration rules are long precisely because they prevent incomplete-fix cycles. Do not trade clarity for character count.

**Descriptive vs prescriptive docs.** Prescriptive docs (`UBIQUITOUS_LANGUAGE.md`, the rule-parts of `CODING_STANDARDS.md`) are authoritative — code conforms to them, the doc wins on conflict, the factory reads them as input. Descriptive docs (`ARCHITECTURE.md`, `CONCEPTS.md`) describe what the code is — code wins on drift, and the factory or `/document` maintains them. This split decides, for every doc, who wins a conflict and who is responsible for keeping it true. It is already latent in `issue.md` Step 8; it just isn't named.

**Decision-point reinforcement is not redundancy.** In always-loaded context, restating a rule is waste. In an autonomous step-by-step run-script, restating a stop condition *at the point the decision is made* improves reliability for an agent deep in a long run that isn't re-reading the top of the file. These two artifact types must be leaned by different rules.

## Findings to act on (verify against the files first)

1. **The factory contract has a home: `commands/issue.md` (+ `factory.md`).** The literalism ("implement the minimum to pass each test… nothing not required by an AC"), the pipeline, and the cycle ceilings live here. The issue-writer and pr-review skills currently re-derive pieces of this. They should reference it instead. The commit-pipeline block specifically is duplicated across `issue.md` (twice), the pr-review skill, and was in the project instructions — single-source it here.

2. **Keystone: "blocking" names two different concepts, and they must not be merged into one.** `issue.md` Step 7 sets a deliberately high bar (security, uncovered AC, compile failure, abstraction-boundary violation, data loss); everything else is non-blocking and noted. The pr-review skill sets a strict bar (any `CODING_STANDARDS.md` violation blocks, never downgrade). It looks like one definition that drifted — a hardcoded infrastructure value is non-blocking to the factory but blocking to the human — but it is not. The factory's bar is *what an agent can reliably resolve in-loop*; the human's bar is *what must be true before this merges*. The gap between them is routed to Review Notes on purpose, because part of it is wrong-approach findings that no autonomous round can close (see the cycle-limit rationale below). So the safe move is to **name both concepts and make the handoff between them explicit — not collapse them into one list.** A literal reading of "reconcile blocking to one definition" would force the factory to grind on exactly the findings this design deliberately surfaces, hit cycle limits, and block more often. The single-source discipline applies only one level down: *what counts as a `CODING_STANDARDS.md` violation* is owned by that repo's doc, and both bars reference it — they just apply it at different gates. This is a human design decision, not a mechanical merge.

   **Why the cycle limits and surfacing exist (preserve this):** review rounds have ~zero marginal return after two, with one exception — a *bonus round gated on a context change* (security focus, the API diff, a numerical reference, a freshly landed dependency), never a bare retry. The reason is the epistemics of failure: bugs converge in one or two rounds, so a finding still standing after two is usually evidence the *approach* was wrong, and a wrong approach is not solvable by more attempts — extra rounds have negative expected value (Opus cost plus the risk of a flailing agent introducing new defects). Surfacing dominates because the fix lives upstream in the issue or architecture, where only a human can reach it. The surviving findings are the diagnostic. New information can move a stuck problem; more effort on the same information cannot — which is exactly why round 3 requires a context change, not just another try.

3. **"Review Notes" is a literal artifact.** `issue.md` Step 10 mandates a Review Notes section in the PR listing every non-blocking finding for the human, and says the human decides what becomes a follow-up. That is the handoff between the factory and the pr-review skill. The skill should name and consume that section directly rather than treating "the PR's review notes" as something generic.

4. **Name the descriptive/prescriptive split.** It is already in the factory's behavior (Step 8 syncs `ARCHITECTURE.md` per-issue, defers `CONCEPTS.md` to the human via `/document`, reads `UBIQUITOUS_LANGUAGE.md`/`CODING_STANDARDS.md` as authoritative). Stating it as a principle lets the reviewers and skills apply it consistently.

5. **`issue.md` footer recaps are cuttable; inline reinforcements are not.** The "Expected commit history" and "Failure modes" footers largely re-list the inline steps. The inline stop-conditions and checkpoints stay (reliability).

## Risk register — what to look for while leaning

**Stripping reinforcement from run-scripts.** The biggest trap. Treating `issue.md` like a context doc and removing restated checkpoints and stop-conditions will raise failure rates in long headless runs. Test before cutting: would an agent 200 turns deep, not re-reading the top of the file, still do the right thing here? If the restatement is the only thing ensuring it, keep it.

**Deferring across a boundary the consumer doesn't load.** Single-sourcing a rule into a doc only works if the consumer reliably loads that doc at the moment it needs the rule. The factory does (`issue.md` Step 2 reads the standards). Conversational webUI work may not. Before moving a rule out of always-loaded context, confirm the consumer loads the source at the point of need — otherwise the result is silent rule violations, not leanness.

**Behavioral change disguised as a refactor.** The blocking-definition reconciliation is the prime case. Tightening the factory's bar to match the human's produces more fix cycles, more cycle-limit hits, more `blocked` labels, slower throughput. Loosening the human's bar to match the factory's turns standards violations into follow-ups instead of merge blockers. Neither is a dedupe; both are policy choices with throughput-vs-quality tradeoffs. Decide deliberately and test on real issues.

**Invisible cross-artifact contracts.** The commit pipeline, the label lifecycle (`ready-for-agent` → `needs-review` → `blocked`), the Review Notes format, the AC→test mapping, and the `Closes #N` convention are each shared across `issue.md`, `ship.md`, `review.md`, the skills, and the tracker. Changing the shape in one place silently breaks consumers elsewhere. Map every consumer of a contract before editing it.

**No compiler.** Prompt changes throw no build error. The only regression net is running the factory on real issues before and after and diffing the behavior — commit history, Review Notes contents, blocking decisions, checkpoint adherence. Keep a small set of representative closed issues to re-run in the sandbox as a regression suite, and run it after each contract is consolidated rather than once at the end.

**Leaning the wrong axis.** File length is not the cost function. Some of the longest sections (enumeration rules) are the cheapest overall because they prevent expensive cycles. Optimize successful-outcome cost.

## Agent vs orchestrator — the split question

The seam to preserve: agents are one responsibility, make no orchestration decisions, are pinned to the cheapest adequate model, and are callable by more than one command. Commands compose agents, make the decisions, choose models, and own the workflow.

Test for "should this be an agent": is it a single, reusable, mechanically-statable responsibility that more than one command needs, and would pinning it to a cheaper model save real tokens? If yes, it's an agent. If it requires judgment about what to do next, or only one command needs it inline, leave it in the command.

Anti-patterns to watch for while leaning:

- Orchestration logic leaking into an agent — the agent starts deciding sequence or branching, and stops being a primitive.
- Agent logic inlined into a command to cut file count — the command balloons and loses both model-pinning and reuse.
- The orchestrator's (expensive) model doing mechanical work that a haiku agent should do — a missing agent showing up as a cost leak. Scan the commands for haiku-appropriate work being done inline.

## Skills, commands, and agents together

Folding the Desktop skills into the repo is the right move — it makes the human-side and autonomous-side halves of the loop visible at once, which is what an overhaul needs.

Working split:

- Skills are human-side, intent-triggered, used in webUI/Desktop — authoring and review aids that run *before* and *after* the autonomous pipeline (issue-writer, pr-review, intent-doc).
- Commands are explicitly invoked pipeline steps, often autonomous, that run *during* (`/issue`, `/factory`, `/review`, `/ship`).
- Agents are the primitives commands compose.

The full loop: author an issue (skill) → factory implements (command) → ship a PR with Review Notes (command) → review the PR and promote findings to follow-ups (skill) → author the follow-up issue (skill).

Two things to decide, given the README's pending "skills versions of commands" item:

- Where workflow logic lives when a skill and a command both touch it. Don't duplicate — pick the home (execution logic in the command/agent, the human-facing wrapper in the skill) and reference across.
- Whether some skills should become commands or vice versa once co-located. Regardless of which side owns them, the shared contracts (blocking definition, commit pipeline, Review Notes format) should be referenced by both sides from one source.

## Method and order of operations

Work per-contract, not per-file. For each shared contract — commit pipeline, blocking definition, Review Notes format, label lifecycle, AC→test mapping:

1. Pick its single home (usually the most authoritative place it already lives).
2. Find every consumer.
3. Make the consumers reference the home.
4. Delete the duplicates.
5. Only then lean within each file.

Establish the home before deleting anything; deleting first loses information. Regression-run after each contract is consolidated, not once at the end.

## What I have not verified

Read in full: `README.md`, `commands/issue.md`. Inferred from those plus sizing: `review.md`, `ship.md`, `factory.md`, the reviewer and other agents, and the current state of the issue-writer and pr-review skills. Before acting, have Claude Code read at least `commands/review.md` and `commands/ship.md` (to confirm the Review Notes and findings contract and the label/PR mechanics), the reviewer agents (to see where severity is *assigned* versus where blocking is *decided* — these may be different places that interact), and the two skills as they currently stand. The blocking-definition finding in particular depends on confirming how `review.md` and the reviewer agents phrase severity, since that feeds the threshold `issue.md` applies.
