# Context Blocks — a UI prototype

Open `index.html` in a browser. Press **Step** (or `→`) to work a thread; press
**Play** to watch it run.

This is a paper prototype for an interaction paradigm, not working software. The
thread is scripted; the token counts are plausible rather than measured.

## The paradigm

A conversation is a log: it only grows, everything that ever happened is still in
it, and the model's context is whatever survived the scroll. This proposes the
opposite shape.

A **thread** is an ordered set of **blocks**. A block has an intent, spends a
sub-conversation working on it, and **resolves to a value** — a fact, a decision,
a design, a result. When it resolves, the sub-conversation is discarded and only
the resolved value stays. The thread's context at any moment is the set of
resolved values, not the union of everything said.

That gives four operations, and they are the whole interface:

- **Resolve** — a block runs, spends whatever it spends, and collapses to what it
  established. The transcript is retrievable (press *peek*) but it is not context.
- **Prune** — context is a set you edit. Untick a resolved value and it leaves,
  immediately, for every block after it. Nothing is load-bearing by accident.
- **Compose** — a block opens on a selection, not on history. The demo's
  implementation block opens on 6 items and 2,255 tokens: the design, the decision,
  three facts, and the glossary — and not one message of the thread that produced
  them.
- **Promote** — what should outlive the thread is written to the corpus on disk,
  where it is reviewed, committed, and checked against the code it describes.

The last one is the join with `modules/context`. Blocks are working memory for one
thread; the corpus is long-term memory across all of them. A thread seeds from the
corpus and promotes back into it, and the checks (`context:lint`,
`context:assertions`, `context:drifted`) are what stop the promoted material from
quietly going stale.

## What the demo is arguing

The measured claim is the one on the chart: the same work, with the same
sub-conversations actually happening, ends with **~2.9k tokens of context instead
of ~148k** — because the 145k of exploration and tool traffic resolved into ~2k of
established truth and was then dropped.

But token count is the shallow win, and it gets less interesting every time a
context window grows. The two that don't:

- **You can see your context, and change it.** Not "what did the model still
  remember" but a list, with sizes, that you edit.
- **Truth is addressable.** A decision is a thing with an identity you can link
  to, supersede, promote, or drop — not a paragraph somewhere in a scroll.

## What it fakes

Everything. There is no model, no engine, no corpus read. The block contents are
written by hand to be plausible for this repository, and the discarded transcripts
are three or four representative lines standing in for twenty.

## The hard parts it doesn't answer

- **When is a block resolved, and who decides?** Collapsing a sub-conversation into
  its result is the entire mechanism, and the demo shows it as a smooth animation
  because it is the part that is hardest to actually do. If resolution needs
  babysitting, this is plan-mode with more ceremony.
- **Re-opening.** A later block contradicts an earlier one. Does the earlier block
  reopen, get superseded in place, or fork the thread? The corpus has an answer for
  documents (`supersedes`); a live thread needs one too.
- **Granularity.** Seven blocks for one design question feels right; seventy would
  not. What is the unit — a question, a phase, a turn?
- **The discarded transcript is still the audit trail.** "Peek" implies it is kept
  somewhere. Where, for how long, and does it come back when a decision is
  challenged six months later?
