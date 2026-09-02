---
title: "Mounted content is readable by the normal file tools but never enters the changeset"
kind: fact
status: active
tags: ["workspace", "tools"]
anchors: ["core/schema/workspace.go", "modules/librarian/main.dang"]
asserts: ['core/schema/workspace.go ~ dagql\.NodeFunc\("withMountedDirectory"', 'core/schema/workspace.go ~ dagql\.NodeFunc\("withoutMount"', 'modules/librarian/main.dang ~ withMountedDirectory']
related: ["glossary/workspace"]
verifiedAt: 355c84d15e1ab6c05d77ad1292f834c329691e4f
---

`Workspace.withMountedDirectory` grafts content into the workspace where the
ordinary file tools — read, grep, find — see it like any other directory, while
keeping it out of the pending changeset entirely: it never shows up in status or
diff, cannot be edited, and is never committed or exported.

That single property is what makes reading a foreign repository safe rather than
dangerous. `modules/librarian` is one function on top of it: clone a git ref,
mount its tree read-only under `mnt/<name>`, and the existing tools do the rest.
An earlier version of that module built a whole parallel tool suite —
`repoGrep`, `repoFind`, `repoRead` — before noticing the engine already had the
clean primitive.

The generalisation is worth keeping in mind when adding a capability to an agent:
prefer making new content visible through the tools that already exist over
adding a second set of tools that differ only in where they look.
