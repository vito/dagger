---
title: "@agent middlewares fold onto one LLM in alphabetical module:fn order, with base injected by the engine"
kind: fact
status: active
tags: ["agents", "llm", "modules"]
anchors: ["core/agents.go", "core/schema/agents.go"]
asserts: ['core/schema/agents.go ~ Compose all selected agent middlewares onto a base LLM, in alphabetical', 'core/schema/agents.go ~ func \(s agentsSchema\) compose', 'core/agents.go ~ func .*Compose\(']
tests: ["AgentsSuite/TestComposeToolset", "AgentsSuite/TestListAcrossModules", "AgentsSuite/TestComposeSeedIsWorkspaceBound"]
related: ["glossary/workspace", "designs/context-documents"]
verifiedAt: 5be6a6b6f70bf0269cf0b1a8593995a0f4543b61
---

A module contributes tools and system prompts to the local agent by marking one
function `@agent`. The contract is a **middleware**, not a producer:

```dang
agent(base: LLM!): LLM! @agent {
  base.withTools(currentNode).withSystemPrompt(systemPrompt)
}
```

`base` is injected by the engine — the running accumulator, not a user argument —
the same way a `Workspace!` argument is filled in. That is what lets the fold call
every marked function as a zero-argument leaf. The producer shape (`agent(): LLM!`,
engine merges the results) was rejected because merging independent LLMs has no
clean answer for whose history and whose system prompt survive.

`AgentMiddlewareGroup.compose` walks the selected leaves in alphabetical
`module:fn` order and threads one LLM through them, seeding a fresh
workspace-bound `llm()` when no base is supplied. Ordering falls out of the
mod-tree rollup, which sorts by path; it is not configurable, so a module must
not depend on running before or after another one. Every step is an ordinary
dagql selector, so the composed LLM's ID records the whole chain and replays
deterministically.

Consequences worth knowing:

- Enumeration covers the current module *and* every installed dependency,
  uniformly — the current module's own agents are included for free.
- A marked function must return `LLM!` and must not require any argument the
  engine cannot inject; both are validation errors at enumeration time, not
  runtime surprises.
- `@agent` functions are excluded from `withTools` toolsets, so the entrypoint
  never shows up as a tool to itself.
