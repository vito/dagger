---
title: "Workspace, changeset, staged commit and mount — the four things an agent's edits pass through"
kind: glossary
status: active
tags: ["workspace", "vocabulary"]
anchors: ["core/workspace.go", "core/schema/workspace.go"]
asserts: ['core/schema/workspace.go ~ dagql\.NodeFunc\("withMountedDirectory"', 'core/schema/workspace.go ~ dagql\.NodeFunc\("withCommit"', 'core/schema/workspace.go ~ dagql\.NodeFunc\("withoutMount"']
verifiedAt: 355c84d15e1ab6c05d77ad1292f834c329691e4f
---

Four words that this codebase uses in its own way. Getting them wrong is the
usual cause of an agent "losing" an edit or committing something it did not
mean to.

**Workspace** — the project as a value. It is immutable: every `with…` method
returns a *new* Workspace rather than changing the one you had, which is why
tools that edit files return `Workspace!` and why the value they return is the
one that matters. Relative paths resolve from the workspace's cwd, absolute
paths from its root.

**Changeset** — a set of pending edits relative to a base. `Workspace.git.uncommitted`
is the changeset holding everything edited but not yet committed; it renders as
per-file stats (`diffStats`) or as a real unified patch (`asPatch`). This is what
`status` and `diff` read, and it is the thing that reaches the user's checkout
when a session is saved.

**Staged commit** — a commit made engine-side with `Workspace.withCommit`, on top
of the workspace's git HEAD. It exists in the workspace value only: the user's
checkout is untouched until the session is saved. After it, `Workspace.git.head`
resolves to the staged tip, so history walks see staged and pre-existing commits
as one namespace. Commits are deterministic — the same workspace state and
arguments produce the same hash — and message-idempotent, so a retried commit
with an identical message is a no-op rather than a duplicate.

**Mount** — content grafted into the workspace with `withMountedDirectory` /
`withMountedFile` / `withMountedCache`, readable through the ordinary file tools
but deliberately *outside* the changeset: it never appears in status or diff,
cannot be edited, and is never committed or exported. That is what makes reading
another repository side-effect free (see `facts/workspace-mounts-are-read-only-and-invisible-to-git`),
and it is why "I mounted it, then my edit vanished" is expected rather than a bug.
`withoutMount` takes one back off.
