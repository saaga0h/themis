# 3 · A green baseline

The factory builds **features, vertically**. A project's bare skeleton — directories, build files, a hello-world — is **horizontal** work, and horizontal work is *not* the factory's job. So you get the project to a starting floor yourself, once.

## What "a green baseline" means

The factory's green gate needs a floor: a project that **builds and has at least one passing test**. That's it. From there, the factory adds behavior one vertical slice at a time.

## How to get there

1. **Shape it with `grill-me`.** Use the `grill-me` skill to turn a fuzzy idea into something tangible: what is this project, what's the minimum, what stack and frameworks, what does the first thin skeleton look like. `grill-me` is a thinking tool — it asks the questions; you decide.
2. **Scaffold with your own tools.** Once you know the shape, create the bare-bones skeleton with your stack's normal tooling (`go mod init`, `cargo new`, `npm create`, a framework generator, …). Themis deliberately does **not** try to be a project generator — that space is unbounded and every stack differs.
3. **Get to green.** Make it build and add one real passing test. Now the green gate has a floor.

With a green baseline, a filled-in [`.themis/workflow.yaml`](02-configure.md), and your [contract docs](04-contracts.md), the factory is ready to build features.

→ Next: [Contract docs](04-contracts.md)
