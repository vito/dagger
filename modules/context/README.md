# Context

Explicit context management for a Dagger coding agent: the project's knowledge as
documents in the repository, composed into the model's context on purpose,
and checked against the code it describes.

## Why

The default arrangement treats a conversation as context. Whatever the model still
has in its window is what the project knows — which makes knowledge accidental
(it depends on what happened to scroll past), unreviewable (nobody diffs a chat
log), unshareable (it belongs to one session, on one machine), and silently stale
(the model will confidently repeat something that stopped being true three commits
ago). Worst of all, the *reasoning* — the constraints weighed, the option rejected
and why — is exactly the part that gets thrown away when the session ends.

This module keeps that material instead. A session's job is not only to change the
code, but to leave the corpus better than it found it.

## The corpus

Markdown with YAML front matter, under `context/` by default:

```
context/
  glossary/…     what the words mean in this codebase
  facts/…        small, checkable claims about the code
  decisions/…    what was decided, and what was rejected, and why
  designs/…      where something is going
```

A document's id is its path under the corpus root without `.md`
(`facts/agent-middlewares-fold-alphabetically`). Everything under the root must be
a well-formed document — there is nowhere to keep a stray README.

```markdown
---
title: "@agent middlewares fold onto one LLM in alphabetical order"
kind: fact                # glossary | fact | decision | design | note
status: active            # draft | active | superseded
tags: ["agents", "llm"]
anchors: ["core/agents.go", "core/schema/agents.go"]
asserts: ['core/agents.go ~ func .*Compose\(']
tests: ["AgentsSuite/TestComposeToolset"]
related: ["glossary/workspace"]
verifiedAt: 5be6a6b6f70bf0269cf0b1a8593995a0f4543b61
pin: false
---

Prose.
```

## Tools

| Tool | Description |
| --- | --- |
| `docs` | List the corpus, grouped by kind, with each document's drift state. |
| `docRead` | Read documents in full, with their links and drift state. |
| `docSearch` | Regex search across prose and front matter. |
| `docCompose` | Assemble a bundle: a selection plus its link closure, ordered, deduplicated, budgeted, with a manifest. |
| `docWrite` | Create or update a document. Refuses to write one whose assertions do not hold. |
| `docStale` | Report which documents have drifted, and name the commits that moved them. |
| `docVerify` | Re-stamp a document as reconciled, after re-running its assertions. |

Tool names are deliberately prefixed: a collision with another module's `read`
would force *every* tool of *both* modules into namespaced form (see
`context/facts/colliding-tool-names-namespace-both-toolsets.md`).

## Checks

| Check | What it enforces |
| --- | --- |
| `context:lint` | Front matter parses; kind and status are known; links resolve; anchors exist; a `fact` carries evidence. |
| `context:assertions` | Every `asserts` entry still holds against the code. |
| `context:drifted` | No active document describes code that has moved since it was last reconciled. |

These run in the ordinary `dagger check`, which is what makes the corpus converge
with the implementation rather than drift away from it in comfort. They are
excluded from the agent toolset: a bare `lint` tool is the kind of name that
collides with another module and drags every tool of both into namespaced form.

## Evidence, and the convergence loop

Three fields tie a document to the code:

- **`anchors`** — the files it describes. They are what makes drift detectable.
- **`asserts`** — claims a machine can re-check: `"<path> ~ <regexp>"` to require a
  match, `"<path> !~ <regexp>"` to forbid one, a bare `"<path>"` to require the path
  to exist. They are verified *before* `docWrite` commits anything, so a fact that
  is already wrong cannot be recorded.
- **`tests`** — the tests a human should read to believe it.

`verifiedAt` records the newest commit touching the anchors at the moment the
document was last reconciled. Comparing that one sha against the newest such
commit today is the whole of drift detection: cheap, exact, and inspectable by
hand (`git log <verifiedAt>..HEAD -- <anchors>`). Uncommitted edits to anchored
files are reported but do not fail the check — your working tree is your business;
once a change is committed, the document is red until somebody reads it.

## Composition

`docCompose` is the deliberate alternative to letting a window fill up. It takes a
selection by id, tag or kind, follows `related` links out to a depth, drops
superseded documents unless named, orders by kind (glossary → fact → decision →
design → note, so definitions land before the things that use them), and clips to
a line budget. The bundle opens with a manifest naming what went in, what was left
out for budget, and which included documents have drifted — so nothing is in
context without the model knowing why it is there or how far to trust it.

The `@agent` middleware folds the corpus index, plus any `pin: true` document, into
the system prompt, so a session opens already knowing what the project knows.

## Usage

```sh
dagger agent                          # the tools are composed in automatically
dagger check context:lint context:assertions context:drifted
```

Configure the corpus root in `dagger.toml`:

```toml
[env.dev.modules.context]
source = "modules/context"

[env.dev.modules.context.settings]
root = "context"
requireEvidence = true
```

## Developing offline

This host has no container runtime, so the module cannot be loaded by an engine
here — but it can still be type-checked and unit-tested, because `dang` accepts a
GraphQL schema from a file and the branch already generates one for the docs site:

```sh
tmp=tmp/ctx-logic                      # tmp/ is gitignored
mkdir -p $tmp
cp modules/context/main.dang modules/context/dagger.json \
   modules/context/testdata/logic.dang $tmp/
printf '[imports.Dagger]\nschema = "%s/docs/docs-graphql/schema.graphqls"\n' \
   "$PWD" > $tmp/dang.toml
dang $tmp                              # type-checks, then runs the assertions
```

`dagger.json` has to be there — `dang` only wires up the Dagger import for a
directory it recognises as a Dagger module — and the `dang` binary must match the
branch (`go install github.com/vito/dang/v2/cmd/dang@v2.1.3`). It will try and fail
to start an engine, warn, and carry on with the file schema.

The `dang.toml` deliberately lives in the scratch copy rather than in the module:
its schema path would not resolve inside the module context the engine loads.

`testdata/logic.dang` covers everything that does not need a live workspace —
front-matter parsing, the emitter round-trip, id validation, assertion shapes and
the renderers. Drift detection, assertion evaluation and the tools themselves need
an engine.

## Status

Prototype. It type-checks against this branch's API schema and its pure-logic
tests pass, but it has never been loaded by an engine, so the parts that touch
`Workspace` and git are unexercised.
