---
title: "A tool-name collision namespaces every tool of every module involved, not just the clashing one"
kind: fact
status: active
tags: ["agents", "llm", "tools"]
anchors: ["core/llm_object_tools.go"]
asserts: ['core/llm_object_tools.go ~ func namespacedToolName', 'core/llm_object_tools.go ~ func namespacedTypes']
tests: ["LLMSuite"]
related: ["facts/agent-middlewares-fold-alphabetically"]
verifiedAt: 764259166e6f206396800839535dc4fb9398ec07
---

Two composed modules that both contribute a tool called `read` do not get
last-wins shadowing. `namespacedTypes` detects the collision and switches **all**
tools of **every** binding involved to qualified names — `editor_read`,
`history_read` — so each toolset stays uniform (either everything bare or
everything prefixed) and nothing is silently hidden. It runs to a fixpoint,
because a namespaced name can itself collide with another binding's bare name.

Bindings with no collision keep their bare names, so the common case stays terse.

The practical consequence for module authors: **one careless name change can
rename every tool the model knows**, mid-project, because the collision is
resolved across the whole composition rather than locally. Prefix the tool names
of a module that adds a general-purpose verb — `docRead` rather than `read` — and
the rest of the toolset keeps its short names.
