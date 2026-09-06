---
name: themis-bug-report
description: "Compose a compact, human-readable Themis bug report from a run's telemetry — ONLY for a genuine Themis defect (not a bad-issue / config / git failure, which is diagnose-themis-run's job). Writes a markdown file the user reviews and chooses whether/where to share. Use when diagnose-themis-run concludes the factory itself behaved wrong."
---

# Compose a Themis bug report

Use this only after `diagnose-themis-run` concludes the failure is a **Themis
defect** — the factory itself behaved wrong. Bad issues, misconfig, and git-state
problems are the diagnose skill's job and are **out of scope here** (we don't want
"the user wrote a bad issue" reports).

## Treat everything in the report as PUBLIC

This is the stance, and it must be stated to the user plainly — do not soften it:

- **Anything you put in this report is public by definition.** Once shared, assume
  it can't be unshared. There is no "we keep it safe" — we don't store it, we don't
  control where it goes, and neither do you once it's out.
- So: **be careful what you collect, and more careful what you send.**
- **Redacting the *known*-sensitive is only half the story.** We can strip the
  obvious (emails, tokens, account IDs — and yes, Themis telemetry contains these).
  But there will be things neither of us knows are sensitive — internal hostnames,
  paths, snippets, data in error output. We cannot reliably classify those.
- **If you're not sure, don't.** Leave it out, or don't share the report.
- **Never auto-send.** This skill writes a file; the user decides whether and where
  it goes.

## What to produce

A single compact markdown file (as little as needed, but enough to be useful),
written to a path the user chooses. Keep it **Themis-scoped**: the factory's
behavior, not the user's code. Sections:

1. **What Themis did wrong** — the observed factory behavior vs. expected, in one or
   two sentences (from the diagnosis).
2. **Run signature** — the relevant `themis/factory` telemetry: the failing `stage`,
   `outcome`, `green_gate`, a trimmed `verify_output` excerpt, `commits`,
   `completed`. Themis version if known. Trim user-code specifics to the minimum
   that demonstrates the defect.
3. **Minimal repro** — the smallest shape that triggers it (issue shape, footprint,
   run path), abstracted away from the user's actual code where possible.

Apply a **best-effort** scrub of the known-sensitive (emails, tokens, account/org
IDs, obvious secrets in `verify_output`). **Never describe the result as "safe" or
"redacted enough."** State, in the file and to the user: *review the whole file and
remove anything you wouldn't post publicly before sharing.*

## Sharing

The user chooses the channel — email, an issue tracker, a chat, whatever; this skill
does not pick and does not send. The only channel it names is the **Themis GitHub
issues** (github.com/saaga0h/themis/issues) as the project's report destination. No
email address, no other channel. End by reminding the user: **read it, redact what
you must, and only then share it — anywhere you send it is public.**
