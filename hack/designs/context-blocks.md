# Context: a set of Blocks

Status: **design**. Prototype on `vito/dagger:context-documents-prototype`
(`modules/context`, `context/`, `hack/context-blocks-ui/`).

## 1. Summary

An agent session's context is, today, a conversation: an append-only log, ordered
by accident, unreviewable, and stale in ways nothing detects. Replace it with a
**Context** — a *set* of **Blocks**, where a Block is a text document plus
metadata. Sessions **inherit a subset** of the Context, add Blocks as they resolve
work, **refresh** to take in what has changed, and **promote** what should outlive
them.

Three properties follow, and they are the whole argument:

- **Addressable.** A decision is a thing with an identity you can link to,
  supersede, or drop — not a paragraph somewhere in a scroll.
- **Editable.** Context is a set you compose and prune. A log can only be appended
  to; a set can have things taken out of it.
- **Checkable.** A Block declares what it depends on, so the system can tell you
  when it stopped being true, instead of the model confidently repeating it.

This unifies two things that started as separate designs: the durable corpus of
project knowledge, and the in-flight results a session accumulates. They are the
same data structure at different scopes. There is one type — Block — and one
container — Context.

## 2. The model

### Block

A Block is **text plus metadata**.

The text is the payload the model reads: prose, markdown, whatever a person would
write. The metadata is what the *system* reads — for selection, verification and
provenance — and only a single compact header line of it ever reaches the model.
That split matters: metadata that has to be read by the model is just more prose
with a colon in it.

| Group | Field | Meaning |
|---|---|---|
| identity | `id` | Address within the Context. Path-shaped: `facts/agent-composition`. |
| | `kind` | `glossary` · `fact` · `decision` · `design` · `result` · `question` · `view` · `note` |
| | `status` | `draft` · `active` · `superseded` |
| selection | `tags` | Free labels; the primary selection axis. |
| | `related` | Blocks that should be composed alongside this one. |
| | `supersedes` | Blocks this one replaces. |
| | `pin` | Always present, in every session. |
| verification | `anchors` | Files this Block describes. Movement here is what makes it drift. |
| | `asserts` | Claims a machine can re-check. |
| | `tests` | Tests that demonstrate it, for a person to follow. |
| | `verifiedAt` | The commit this Block was last reconciled against. |
| provenance | `origin` | `authored` · `resolved` · `promoted` |
| | `resolvedFrom` | The sub-session this Block collapsed out of, and what it cost. |
| | `scope` | `session` · `durable` |

`kind` says what a Block *is*; `origin` says how it came to exist. Keeping those
apart is why a fact that a person typed and a fact a sub-agent resolved are the
same kind of thing, selected the same way, checked the same way.

### Context

A Context is a **set** of Blocks. Set, not list, and that word is load-bearing:

- **Membership is idempotent.** Adding the same Block twice is a no-op. Two
  sessions that establish the same fact converge on one Block instead of leaving
  two paragraphs that mostly agree.
- **There is no intrinsic order.** Ordering is therefore an *operation*
  (composition), performed against a purpose, not a property the data carries
  around from whenever it happened to be written.
- **Removal is expressible.** This is the one a log fundamentally cannot do. You
  cannot unsay a message; you can take a Block out of a set.

The durable Context is the repository: markdown with front matter under
`context/`, committed, reviewed in pull requests, checked in CI. That is the whole
persistence story — no separate store to keep in sync, and `git log context/` is a
history of what the project learned.

### Session

A session never holds "the Context." It holds a **selection**: an explicit subset
of the durable Context, plus the Blocks it has created itself. Everything the
session knows is in that set and nowhere else — there is no ambient history behind
it that might also be influencing the model.

## 3. Operations

Six, and they are the entire interface.

### compose(selection) → context

Resolve a selection into an ordered, budgeted, rendered context.

- **Selection** is by id, tag, kind, link-closure depth, and exclusions.
- **Ordering** runs `glossary → fact → decision → design → result`, ties broken by
  address, so definitions land before the things that use them.
- **Budget** is a line or token ceiling. Blocks that do not fit are *named in the
  manifest*, never silently dropped.
- **Manifest** opens every composed context: what went in, what was left out and
  why, and which included Blocks have drifted.

The manifest is how "you can see your context" stops being a slogan. Nothing is in
a model's context without a line saying why it is there and how far to trust it.

### resolve(intent) → Block

Spend a sub-session on an intent, keep what it established, drop the transcript.
The resulting Block carries `resolvedFrom`: a handle to the sub-session's trace and
what it cost, so the discarded work is *retrievable* without being *resident*.

This is the operation the whole paradigm rests on, and the one most likely to be
wrong — see §8.

### prune(id…)

Take Blocks out of the session's Context. Immediate, and it applies to every later
composition in the session.

Pruning is not deletion. The Block still exists in the Context; it is out of *this
session's* selection. Deletion is an edit to the durable set, and goes through
review like any other.

### refresh()

Re-materialize the session's Context from the durable set:

1. re-read the durable Blocks
2. re-resolve the selection — Blocks that now match a tag join, deleted ones leave
3. re-evaluate verification: assertions, drift
4. recompose
5. **report the change**: `+added  −removed  ~updated  !drifted`

Step 5 is not optional. A context that changes underneath a session without saying
so is worse than one that never changes at all.

Refresh is what makes a Context a live view rather than a snapshot. You edit a
Block in your editor; another session promotes a fact; a commit lands that moves an
anchor. Refresh takes in all three and names them. Because the Context is a set,
refresh can *remove* and *replace* — operations a conversation cannot perform on
itself, and the reason "just re-read the file" is not the same thing.

The mechanism already exists on the prototype branch in a narrower form: `install`
and `reload` recompose an agent against an updated workspace mid-session, with the
conversation intact. Refresh generalises that from modules to Blocks.

### promote(id…)

Flip a session Block to `durable` and write it into the repository. Assertions run
first: **a Block that does not check out cannot be promoted.** The write lands in
the workspace changeset, so it shows up in `status`/`diff` and is committed
alongside the code that motivated it.

### verify(id…)

Re-run a Block's assertions and re-stamp `verifiedAt` against its anchors. It is a
claim — *I have read this against what changed* — so it refuses to stamp a Block
whose assertions fail.

## 4. Verification, and what makes it converge

Three metadata fields tie a Block to the code:

- **`anchors`** — the files it describes. Movement here is what makes it drift.
- **`asserts`** — claims a machine can re-check: `"<path> ~ <regexp>"` to require a
  match, `"<path> !~ <regexp>"` to forbid one, a bare `"<path>"` to require
  existence. Checked *before* a write goes through, so a Block that is already
  wrong cannot be recorded.
- **`tests`** — the tests a person should read to believe it.

`verifiedAt` records the newest commit touching the anchors at the moment the Block
was last reconciled. Drift is then one sha comparison: cheap, exact, and
inspectable by hand (`git log <verifiedAt>..HEAD -- <anchors>`). Uncommitted edits
are reported but do not fail — your working tree is your business; once a change is
committed, the Block is red until somebody reads it.

Three checks run in the ordinary `dagger check`, beside the tests:

| Check | Enforces |
|---|---|
| `context:lint` | Front matter parses; kind and status known; links resolve; anchors exist; a `fact` carries evidence. |
| `context:assertions` | Every recorded claim still holds. |
| `context:drifted` | No active Block describes code that has moved since it was last reconciled. |

Structurally this is a crude **justification-based truth maintenance system**: a
Block is a belief, its anchors are its justification, and invalidation is a check
going red. That machinery is forty years old and well understood; what is new is
pointing it at prose about a codebase rather than at logical propositions.

## 5. The life of a Block

```text
  intent
    │  resolve
    ▼
  Block  scope: session ──── prune ────▶ out of this session's selection
    │                                     (still in the Context)
    │  promote  (assertions must pass)
    ▼
  Block  scope: durable ──▶ committed · reviewed · checked in CI
    │
    │  anchors move
    ▼
  drifted ──── verify ────▶ active
    │     └─── rewrite ───▶ active
    │
    │  a later Block supersedes it
    ▼
  status: superseded        (kept, never deleted — the reasoning
                             outlives the conclusion)
```

## 6. What exists, and what this adds

Built and running on the prototype branch (`modules/context`, a Dang module with an
`@agent` middleware):

- Blocks as markdown + YAML front matter under a configurable root.
- `docCompose` — selection, link closure, kind ordering, budget, manifest.
- `docWrite` — assertion-checked writes that land in the workspace changeset.
- `docStale` / `docVerify` — drift reporting and re-stamping.
- The three checks.
- Session start: the middleware folds the Block index, plus every `pin`ned Block,
  into the system prompt — so a session opens knowing what the project knows,
  without a single message of history.

New in this design, not yet built:

- **One type.** Corpus documents and in-session results unify into Block, separated
  only by `scope`.
- **Session scope and promotion** as an explicit boundary, rather than every write
  being immediately durable.
- **`refresh`** as a first-class operation with a change report.
- **`resolvedFrom`** — provenance for the sub-session a Block collapsed out of.
- **Views** — a saved selection. Probably itself a Block (`kind: view`), so views
  are versioned, reviewable and linkable like everything else; see §9.

## 7. The interface

`hack/context-blocks-ui/` is a working paper prototype. A thread is a column of
Blocks; each shows what it kept and, struck through beneath it, what it discarded
(peek to read the sub-conversation, rendered explicitly *outside* the context). The
right panel is the Context: every Block with its size and a checkbox. Untick one
and it leaves — the Block dims, the composed size drops, and the preview of what
the next Block will actually receive updates.

The demo scripts one real question through seven Blocks and ends carrying **2,885
tokens of context where the same work left inline would carry 147,955** — because
145k of exploration and tool traffic resolved into ~2k of established truth and was
then dropped.

Token count is the shallow win, and it decays every time a context window grows.
The two that do not: you can see and edit your context, and resolved truth is
addressable.

One design constraint from the literature (§8): people use a canvas or block view
for *orientation, navigation and reflection*, and keep a linear conversation for
*generation and task progression*. Blocks are the orientation layer over a
conversational surface, not a replacement for one. The current demo has no
conversational surface at all, which is probably its biggest omission.

## 8. Prior art, and the two traps

**The graveyard: design rationale capture.** IBIS (Rittel, 1970), gIBIS,
Compendium — Questions, Ideas and Arguments as linked nodes, capturing reasoning
rather than transcript. This is the Block model, thirty years early, and it mostly
failed. Grudin's diagnosis is precise: there cannot be a disparity between who
invests the effort and who benefits; nobody enters quality rationale for an unknown
person at an unknown future time. And it is not a capture problem but a *useful*
capture problem — videotaping every meeting is not an archive.

Two things make this not obviously a re-run. The model does the capture, so the
effort cost approaches zero. And the capturer is the immediate beneficiary:
resolving a Block makes *this session* cheaper, right now. That inverts exactly the
asymmetry that killed IBIS. Worth noting too that IBIS did work in the hands of a
skilled facilitator; it failed as something individuals do to themselves.

**The live disagreement.** Cognition's *Don't Build Multi-Agents* argues the
opposite of this design, explicitly: share full agent traces, not just individual
messages, because "actions carry implicit decisions" that a summary drops. Their
worked example is a block-collapse failure — two sub-agents each resolved locally
and correctly, and the results did not compose. Anthropic's multi-agent research
system does the reverse and reports large gains. Both are probably right about
different task shapes: parallel and independent favours isolation, sequential and
interdependent favours the shared trace. **This design is sequential**, which puts
it on the contested side of that line.

**The other trap: ceremony.** Living documentation and Gherkin were sold with this
exact convergence pitch — put requirement, test and doc in one place and the drift
stops. In practice it produced enormous duplication (one analysis found more than
four in five Gherkin steps byte-identical to another, a single step repeated over
twenty thousand times) plus incompleteness, delay and inconsistency in authoring.
The lesson for `asserts`: an assertion vocabulary must never become a DSL people
write ritually. `"<path> ~ <regexp>"` is cheap enough to write without thinking,
which is precisely how you end up with twenty thousand of them.

**Nearest live neighbours.** MemGPT/Letta — context as RAM, external store as disk,
self-editing memory paged by the agent; strong reported gains, at the cost of making
the model meta-reason about memory operations. Ink & Switch's Patchwork —
version-controlled documents where AI proposes changes on branches you merge like a
coauthor's. A deRSE25 design-thinking workshop proposed almost exactly the inspector
panel here — an explicit, user-curated context window with add/remove, reorder, and
saved arrangements — but as a concept, never implemented or user-tested.

Every individual piece has prior art. The combination — capture performed by the
model, justification-based invalidation against a live codebase, and resolved values
as the unit of composition for the next session — appears to be open.

## 9. Open questions

1. **Does a resolved Block carry the implicit decisions its transcript held?** This
   is Cognition's objection and the load-bearing assumption of the entire design.
   It is testable, and should be tested first: resolve a real thread, hand only the
   resolved Context to a fresh session, and see whether it makes a choice that
   contradicts something only the transcript knew.
2. **Who decides a Block is resolved, and what its synthesis is?** The demo animates
   this in 750ms because it is the part that is hardest to actually do. If it needs
   babysitting, this is plan mode with more ceremony.
3. **Re-opening.** A later Block contradicts an earlier one. Supersede in place,
   reopen, or fork the thread? The durable set has an answer (`supersedes`); a live
   session needs one.
4. **Granularity.** Seven Blocks for one design question feels right; seventy would
   not. What is the unit — a question, a phase, a turn?
5. **Are views Blocks?** Self-hosting is elegant — versioned, reviewable, linkable
   selections — but a `kind` whose body is not really prose strains the model. Lean:
   yes, and see whether it reads badly in practice.
6. **Where does `resolvedFrom` live, and for how long?** "Peek" implies the discarded
   transcript is kept. Where, at what cost, and does it come back when a decision is
   challenged a year later?
7. **Does `refresh` interrupt, or wait for a turn boundary?** Swapping context
   mid-turn is a different thing from swapping it between turns, and only one of them
   is safe.
8. **Does the Context need a size discipline of its own?** A set that only grows is a
   log with extra steps. `superseded` handles contradiction; nothing yet handles
   accumulation.
