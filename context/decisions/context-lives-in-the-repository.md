---
title: "Context documents are files in the repository, not rows in a session store"
kind: decision
status: active
tags: ["context", "persistence"]
anchors: ["modules/context/main.dang"]
asserts: ['modules/context/main.dang ~ withNewFile', 'modules/context/main.dang ~ docWrite']
related: ["designs/context-documents", "glossary/workspace"]
---

The corpus is markdown with YAML front matter under `context/`, written through
`Workspace.withNewFile` like any other source edit. It is committed with the code
it describes and reviewed in the same pull request.

The alternatives that were considered and rejected:

**A session store keyed by conversation.** Reproduces the problem it was supposed
to solve: knowledge scoped to a session, invisible to everybody else, impossible
to review, and lost when the session is. The whole point is that what a session
learns should outlive it.

**A database or embedding index.** Buys retrieval quality and costs everything
else: another service to run, a corpus nobody can read or edit by hand, no diffs,
no review, no bisect. At project scale the index is small enough that a listing
plus grep is not the bottleneck — and when it becomes one, an index can be built
*from* the files without changing where they live.

**Documents as values inside a Dagger module.** Tempting, since checks and tools
are already module functions and it would make the corpus typed. But it makes
prose an edit to source code, which is exactly the friction that stops people
writing things down. Files stay hand-editable; the module supplies the semantics
over them.

What falls out of the choice, and would have to be rebuilt under any of the
alternatives:

- Writes land in the session's changeset, so they show up in `status` / `diff`
  and get committed alongside the change that motivated them.
- `git log context/` is a history of what the project learned, bisectable and
  attributable.
- A document can be edited with the ordinary file tools, or in an editor, by
  somebody who has never heard of this module.
- Review of a claim about the codebase happens where review already happens.
