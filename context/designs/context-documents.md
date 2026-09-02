---
title: "Context is generated, refined and composed as documents — the conversation is the byproduct, not the record"
kind: design
status: active
pin: true
tags: ["context", "agents", "workflow"]
anchors: ["modules/context/main.dang", "modules/context/README.md"]
asserts: ['modules/context/main.dang ~ @agent', 'modules/context/main.dang ~ docCompose']
tests: ["context:lint", "context:assertions", "context:drifted"]
related: ["decisions/context-lives-in-the-repository", "facts/agent-middlewares-fold-alphabetically"]
verifiedAt: 2605ac7ce790eac46fc80758e7a1295a0080d802
---

The working assumption of most agent tooling is that a conversation *is* the
context: you talk, the window fills, and whatever the model still remembers is
what the project knows. That makes knowledge accidental, unreviewable, and
silently stale.

Here the context is built on purpose, out of documents:

- **Generate.** A session that establishes something — how a subsystem behaves,
  why an option was rejected, what a word means here — writes it down with
  `docWrite` before it ends. One claim per document.
- **Refine.** Documents are corrected in place and linked to each other. A
  replaced document is marked `superseded` rather than deleted, so the reasoning
  survives its conclusion.
- **Compose.** `docCompose` assembles a bundle from a selection plus the closure
  of its links, ordered so definitions land before the things that use them, with
  a manifest naming what went in, what was dropped for budget, and what has
  drifted. Sessions start by composing what the task needs.
- **Converge.** Each document names the files it describes (`anchors`), claims a
  machine can re-check (`asserts`), and the tests that demonstrate it (`tests`),
  plus the commit it was last reconciled against (`verifiedAt`). Changing anchored
  code makes the document drift, and `context:drifted` stays red until somebody
  reads it against the change. Documents cannot quietly fall behind the code,
  because the code moving is what makes them visibly wrong.

The three checks are the whole enforcement surface: `context:lint` (well-formed,
links resolve, facts carry evidence), `context:assertions` (the claims still hold),
`context:drifted` (nothing describes code that has moved). They run in the same
`dagger check` as the tests, which is what "checked continuously" means here.
