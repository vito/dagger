package schema

import (
	"context"
	"fmt"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/dagql"
	"github.com/dagger/dagger/dagql/call"
)

// agentSchema installs the Agent type: the async evaluation-loop entity of
// hack/designs/async-agents.md. Agent instances are created by LLM.spawn
// (minted, or restored under a supplied handle) and addressed through the
// pure LLM.agent(handle:) lookup (core/schema/llm.go); this file installs the
// type's lifecycle fields, all of which operate on the session-scoped runtime
// registry (core.AgentRuntimes) rather than on the value.
type agentSchema struct{}

var _ SchemaResolvers = &agentSchema{}

func (s agentSchema) Install(srv *dagql.Server) {
	// Agents are v1+ API surface; installing the classes with a view gate also
	// gates their generated ID/load fields.
	srv.InstallObject(dagql.NewClass[*core.Agent](srv).View(AfterVersion("v1.0.0-0")))
	srv.InstallObject(dagql.NewClass[*core.AgentMessage](srv).View(AfterVersion("v1.0.0-0")))

	dagql.Fields[*core.Query]{
		dagql.NodeFunc("__agentReference", func(ctx context.Context, _ dagql.ObjectResult[*core.Query], args struct{ Handle string }) (dagql.ObjectResult[*core.Agent], error) {
			agents, err := agentRuntimes(ctx)
			if err != nil {
				return dagql.ObjectResult[*core.Agent]{}, err
			}
			return agents.Reference(ctx, args.Handle)
		}).View(AfterVersion("v1.0.0-0")),
	}.Install(srv)

	dagql.Fields[*core.Agent]{
		dagql.NodeFunc("parent", s.parent).
			DoNotCache("Projects lineage from the current session's runtime registry.").
			Experimental("Agent APIs are likely to change.").
			Doc("The agent that spawned this agent, or null for a root agent. Restored agents retain their recorded parent handle."),
		dagql.Func("name", s.name).
			Experimental("Agent APIs are likely to change.").
			Doc(`Display label for the agent; carries no identity.`),

		// The runtime handle is already public: it rides every loop span as
		// dagger.io/agent.id, which is how a client's roster keys its
		// entries (design §9). Reading it off a handle is what lets that
		// client tell whether a rostered agent is one it already drives —
		// the correlation focus needs — and it grants nothing extra: you
		// still need the handle to ask, so §3.3's capability model holds.
		dagql.Func("handle", s.handle).
			Experimental("Agent APIs are likely to change.").
			Doc(`The opaque runtime handle minted by the spawn that created this agent.`,
				`It is the same value the agent's loop span publishes as dagger.io/agent.id, so a client can correlate the agent with what it discovers in the trace. Two spawns of an identical composition have different handles; a display name is shared freely.`),

		dagql.NodeFunc("state", s.state).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Projects live runtime state.").
			Doc(`Computed lifecycle state; never stored.`,
				`An agent that was never started reports IDLE: its mailbox is empty and no turn is open.`),

		dagql.NodeFunc("error", s.loopError).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Projects live runtime state.").
			Doc(`Why the loop failed, for a FAILED agent; empty otherwise.`,
				`The snapshot holds the completed prefix — send or resume retries from it.`),

		// Public Agent IDs carry their fixed seed. Handle-only references read
		// it from the runtime, after initial composition has completed.
		dagql.NodeFunc("seed", s.seed).
			Experimental("Agent APIs are likely to change.").
			Doc(`The conversation the agent was spawned from: the tools, workspace and message history its loop started with.`,
				`Fixed by the spawn; stepping does not move it (see snapshot for the latest committed step). Its workspace is the baseline the agent's own changes are measured from.`,
				`For an agent restored under a handle, this is the conversation it was restored from.`),

		dagql.NodeFunc("snapshot", s.snapshot).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Reflects the loop's last committed step, which advances as the agent runs.").Reevaluate().
			Doc(`The conversation as of the last committed step: immutable, branchable, persistable.`,
				`The seed conversation if the agent never stepped.`,
				`Branching from it does not affect the agent.`),

		dagql.NodeFunc("send", s.send).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Every send enqueues a distinct message into live runtime state.").
			Doc(`Enqueue a message, on the record: it is consumed at a step boundary, appends to the agent's history, and steers the running turn or opens a new one.`,
				`Never blocks, never drops; concurrent sends queue in order.`,
				`The returned message is pinned through the message lookup field, so its handle is re-addressable from any request in the session: cancel a response request and request it again freely.`,
				`Sending to a never-started agent starts it (signal-with-start). Sending to a stopped agent restarts the same instance from its last committed snapshot. Sending to a paused or failed agent enqueues with QUEUED delivery, to be drained by a resume.`).
			Args(
				dagql.Arg("message").Doc(`The message text, appended to the agent's history as a prompt when a turn consumes it. When content is supplied, nonempty text precedes those blocks in the same user message.`),
				dagql.Arg("replyTo").Doc(`The ref of a message in the SENDER's own mailbox this send answers (e.g. "#3", from its attribution header). The recipient sees the two paired, and awaiters of the replied-to message resolve with this reply immediately instead of at the sender's turn end.`),
				dagql.Arg("content").Doc(`Ordered TEXT, IMAGE, AUDIO, or DOCUMENT user content blocks. File inputs are resolved and all content is validated before enqueueing. Pass an empty message for media-only sends.`),
			),

		// message is deliberately cached (no DoNotCache): the ref argument
		// pins the lookup to one immutable identity-only handle and
		// permanent runtime records are never deleted. Mutable/pending delivery
		// evidence is read by AgentMessage.delivery, not copied onto this value,
		// so the same chain always denotes the same handle without freezing its
		// current runtime state. The chain is per-session by construction (every
		// Agent descends from Query.llm's PerSessionInput), so the cached handle
		// can never leak across sessions. Caching is also what makes send's
		// re-exec pinning cheap: re-loading a pinned ID replays
		// …agent(handle:…)!message(ref:…) and lands on the same cached instance.
		dagql.NodeFunc("message", s.message).
			Experimental("Agent APIs are likely to change.").
			Doc(`Look up a previously sent message by its ref.`,
				`This is the lookup send pins its result's identity through: the returned handle's ID is an honest, replayable chain, addressable from any request in the session (the cancel-and-request-again contract).`,
				`Fails if the agent has no runtime entry in this session, or no record of the given ref.`).
			Args(
				dagql.Arg("ref").Doc(`The message's short ref within this agent's runtime, e.g. "#3": the token its attribution header shows and a reply's replyTo names. A bare ordinal ("3") is accepted too.`),
			),

		dagql.NodeFunc("pause", s.pause).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Imperatively mutates runtime state.").
			Doc(`Stop draining the mailbox once the in-flight step completes, or immediately with interrupt.`,
				`Pause takes priority over pending work: a mid-turn pause suspends the turn, which resume continues. Messages sent while paused enqueue with QUEUED delivery until a resume.`,
				`Pausing a never-started agent leaves it paused for its eventual resume; pausing a failed agent is allowed (resume decides the retry); pausing a stopped agent fails.`).
			Args(
				dagql.Arg("interrupt").Doc(`Preempt the in-flight step instead of letting it finish. All completed steps are kept and the interrupted turn stays open: messages it consumed remain pending, while unconsumed mailbox messages are discarded. Resume continues the turn from the last committed step. On an idle, never-started, or failed agent there is nothing to preempt, so this is a plain pause.`),
			),

		dagql.NodeFunc("resume", s.resume).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Imperatively mutates runtime state.").
			Doc(`Resume draining the mailbox: a suspended turn continues from the last committed step, and queued messages drain.`,
				`Resuming a never-started agent starts its evaluation loop, detached from the calling request: it steps the conversation while input is pending, then idles awaiting further lifecycle operations. Resuming a FAILED agent retries its pending step. Resuming a STOPPED agent relaunches the same instance from its last committed snapshot.`,
				`No-op on a running or idle agent.`),

		dagql.NodeFunc("wait", s.wait).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Blocks on live runtime state.").
			Doc(`Block until the agent settles: IDLE, FAILED, or STOPPED. Read which from state afterwards.`,
				`Unlike waiting for one exact state, this cannot hang merely because the agent settled in a different outcome.`),

		dagql.NodeFunc("notify", s.notify).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Imperatively mutates runtime state.").
			Doc(`Subscribe another agent to this agent's lifecycle: each transition into one of the given states enqueues an event message to the subscriber — steering its open turn, or waking it if idle, like any other message.`,
				`This is how a supervisor hears every completion and failure without polling or blocking: subscribe at spawn time, keep working, and events arrive as attributed messages.`,
				`Events never relaunch a stopped subscriber, and an already-reached state fires immediately at subscribe time, so a fast agent settling before the subscription lands is not missed.`,
				`A restored agent that nothing has sent to, started, or resumed yet is the exception: its state was reached in the session it was restored from, so subscribing to it announces nothing until it next transitions. This is how a restore reinstalls recorded subscriptions without waking their subscribers.`,
				`Idempotent per subscriber; re-subscribing replaces the state set.`).
			Args(
				dagql.Arg("subscriber").Doc(`The agent to deliver event messages to. You must hold its handle: subscriptions are capability-based like everything else.`),
				dagql.Arg("on").Doc(`The lifecycle states that fire an event. IDLE events carry the turn's final reply; FAILED events carry the loop error.`),
			),

		dagql.NodeFunc("stop", s.stop).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Imperatively mutates runtime state.").
			Doc(`Release the agent's runtime. The tombstone (state, snapshot) stays readable for the rest of the session.`).
			Args(
				dagql.Arg("kill").Doc(`Cancel the loop immediately instead of letting an in-flight step finish. Either way the completed steps are preserved in the snapshot.`),
			),

		dagql.NodeFunc("reseed", s.reseed).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Imperatively mutates runtime state.").
			Doc(`Replace this instance's committed conversation with the given one, keeping the entry: identity, mailbox, and lifecycle state are untouched. A paused suspended turn is abandoned and its consumed messages are resolved before replacement.`,
				`This is the continuity verb. Compaction, a workspace rebind, a model change, or rewinding an interrupted prompt produce a new conversation value for the SAME agent; reseed swaps it in place, where a stop-and-respawn would mint a successor instance and split the agent across two roster entries. It is the client-facing form of what a continuation tool already does mid-turn: the agent adopts a new conversation without changing who it is.`,
				`The next turn continues from the reseeded conversation, and queued messages drain onto it. A FAILED agent keeps its error — resume retries from the new conversation.`,
				`Fails if the instance has no runtime entry in this session (only a spawned or re-hydrated instance holds a conversation to replace), if a step is in flight, or if the agent is stopped.`).
			Args(
				dagql.Arg("conversation").Doc(`The conversation that becomes the agent's committed history, replacing the current one.`),
			),

		withAgentComposition(dagql.NodeFunc("compose", s.compose)).
			Experimental("Agent APIs are likely to change.").
			Doc(`Compose expertise into a conversation as this agent. Return the new conversation; the agent itself is unchanged. Apply the result by returning it from a tool, which continues the turn on it, or with reseed.`,
				`Run the given entries in list order and record them. An entry already recorded under the same artifact address, with dimension keys, is refreshed in place: its contributions are replaced and its compatible tool state is preserved. Other recorded entries are untouched.`,
				`Every Agent argument of an entry, required or optional, receives this agent. The conversation must be unowned or owned by this agent, and the result is owned by it.`,
				`While an entry runs, the entries' contributions belong to that outer entry and are not recorded separately: this is how an entry composes other expertise, e.g. me.compose(conversation: base, expertise: [...]). Rerunning the outer entry reproduces them.`).
			Args(
				dagql.Arg("expertise").Doc(`The expertise to run. Each reference retains its source workspace.`),
				dagql.Arg("conversation").Doc(`The conversation to compose into. Defaults to the agent's snapshot.`),
			),

		withAgentComposition(dagql.NodeFunc("recompose", s.recompose)).
			Experimental("Agent APIs are likely to change.").
			Doc(`Refresh every recorded entry of a conversation as this agent. Return the new conversation; the agent itself is unchanged. Apply the result by returning it from a tool, which continues the turn on it, or with reseed.`,
				`Find each recorded entry again by its artifact address, with dimension keys, in the conversation's bound workspace. If that workspace does not address an entry, rerun the recorded entry from its own source. A recorded module that fails to load there is an error.`,
				`Entries run in record order, replacing their contributions and preserving compatible tool state. Contributions made outside an entry are unowned and retained. A change to a tool binding's version resets its state. Removed bindings, changed identities, and incompatible state are errors.`,
				`Every Agent argument of an entry receives this agent. Fails if the conversation has no recorded expertise, is owned by another agent, or if called while an entry runs.`).
			Args(
				dagql.Arg("conversation").Doc(`The conversation to recompose. Defaults to the agent's snapshot.`),
			),
	}.Install(srv)

	dagql.Fields[*core.AgentMessage]{
		dagql.Func("delivery", s.messageDelivery).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Blocks on pending delivery evidence in live runtime state.").
			Doc(`How the message conclusively landed: opened a new turn (STARTED), was absorbed into the running turn at a step boundary (STEERED), or queued behind it (QUEUED).`,
				`Blocks until provider or native lifecycle evidence is conclusive. Once recorded, the result or cancellation error is immutable.`),

		dagql.Func("ref", s.messageRef).
			Experimental("Agent APIs are likely to change.").
			Doc(`The message's short ref within the receiving agent's runtime, e.g. "#3".`,
				`This is the deterministic token the recipient's attribution header shows and a reply's replyTo names — quote it when telling the recipient what to answer.`),

		dagql.Func("response", s.messageResponse).
			Experimental("Agent APIs are likely to change.").
			DoNotCache("Blocks on live runtime state.").
			Doc(`Block until this message is answered, and return the answer: an explicit reply (a send whose replyTo names this message), or the final reply of the turn that consumed it, whichever comes first.`,
				`Idempotent: cancel and request the response again freely; concurrent waiters share the result.`,
				`Fails if the agent stops before the message resolves. On a failed agent it projects the failure — but the message stays pending, so after a resume consumes it, requesting the response again returns the real reply.`,
				`Refused when called from inside an agent turn whose wait would deadlock: turns should not block on other agents — send without awaiting, and the reply arrives as a message.`),
	}.Install(srv)

	core.AgentStates.Install(srv, AfterVersion("v1.0.0-0"))
	core.AgentMessageDeliveries.Install(srv, AfterVersion("v1.0.0-0"))
}

func (s agentSchema) parent(ctx context.Context, agent dagql.ObjectResult[*core.Agent], _ struct{}) (dagql.Nullable[dagql.ObjectResult[*core.Agent]], error) {
	var result dagql.Nullable[dagql.ObjectResult[*core.Agent]]
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return result, err
	}
	rt, found, err := agents.Get(ctx, agent)
	if err != nil {
		return result, err
	}
	if !found {
		return result, fmt.Errorf("agent %q has no runtime entry in this session", agent.Self().Handle)
	}
	if handle := rt.ParentHandle(); handle != "" {
		parent, err := core.AgentReference(ctx, handle)
		if err != nil {
			return result, err
		}
		return dagql.NonNull(parent), nil
	}
	return result, nil
}

func (s agentSchema) name(ctx context.Context, agent *core.Agent, _ struct{}) (string, error) {
	return agent.Name, nil
}

func (s agentSchema) handle(ctx context.Context, agent *core.Agent, _ struct{}) (string, error) {
	return agent.Handle, nil
}

func agentRuntimes(ctx context.Context) (*core.AgentRuntimes, error) {
	query, err := core.CurrentQuery(ctx)
	if err != nil {
		return nil, err
	}
	return query.Agents(ctx)
}

// agentSelfID returns the parent agent's own ID as the field's result — the
// sync-style contract of the imperative verbs (like Service.start/stop): they
// return ID! with @expectedType rather than the object, so lazy clients force
// the verb at the call site and re-hydrate the handle from the ID instead of
// re-evaluating a DoNotCache chain (which would re-run the side effect).
func agentSelfID(ctx context.Context, parent dagql.ObjectResult[*core.Agent]) (res dagql.Result[core.AgentID], _ error) {
	parentID, err := parent.ID()
	if err != nil {
		return res, fmt.Errorf("agent ID: %w", err)
	}
	return dagql.NewResultForCurrentCall(ctx, dagql.NewID[*core.Agent](parentID))
}

func (s agentSchema) state(ctx context.Context, parent dagql.ObjectResult[*core.Agent], _ struct{}) (core.AgentState, error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return "", err
	}
	rt, found, err := agents.Get(ctx, parent)
	if err != nil {
		return "", err
	}
	if !found {
		// No runtime entry (reads never create one), which today means a
		// handle built by the bare agent(handle:) lookup rather than by a
		// spawn, which creates the entry it names. The projected state of
		// that absence is IDLE: mailbox empty, no turn open. Runtime verbs
		// still reject the handle: only spawn may create the entry.
		return core.AgentStateIdle, nil
	}
	return rt.State(), nil
}

// loopError projects the FAILED tombstone's error. Like state, absence of a
// runtime is an honest empty answer, never a creation.
func (s agentSchema) loopError(ctx context.Context, parent dagql.ObjectResult[*core.Agent], _ struct{}) (string, error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return "", err
	}
	rt, found, err := agents.Get(ctx, parent)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return rt.LoopError(), nil
}

func (s agentSchema) seed(ctx context.Context, parent dagql.ObjectResult[*core.Agent], _ struct{}) (dagql.ObjectResult[*core.LLM], error) {
	if parent.Self().Seed.Self() != nil {
		return parent.Self().Seed, nil
	}
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return dagql.ObjectResult[*core.LLM]{}, err
	}
	rt, found, err := agents.Get(ctx, parent)
	if err != nil {
		return dagql.ObjectResult[*core.LLM]{}, err
	}
	if found {
		return rt.Seed()
	}
	return dagql.ObjectResult[*core.LLM]{}, fmt.Errorf("agent %q has no runtime entry in this session", parent.Self().Handle)
}

func (s agentSchema) snapshot(ctx context.Context, parent dagql.ObjectResult[*core.Agent], _ struct{}) (res dagql.ObjectResult[*core.LLM], _ error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return res, err
	}
	rt, found, err := agents.Get(ctx, parent)
	if err != nil {
		return res, err
	}
	if !found {
		// Never started: the last committed conversation is the seed.
		return parent.Self().Seed, nil
	}
	return rt.Snapshot(), nil
}

func (s agentSchema) send(ctx context.Context, parent dagql.ObjectResult[*core.Agent], args struct {
	Message string
	ReplyTo string                                         `name:"replyTo" default:""`
	Content []dagql.InputObject[core.LLMContentBlockInput] `default:"[]"`
}) (res dagql.Result[core.AgentMessageID], _ error) {
	blocks, err := resolveLLMContent(ctx, args.Content)
	if err != nil {
		return res, err
	}
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return res, err
	}
	// Signal-with-start plus delivery-evidence computation both live in the
	// registry's central enqueue path.
	msg, err := agents.Send(ctx, parent, args.Message, args.ReplyTo, blocks...)
	if err != nil {
		return res, err
	}
	// send is DoNotCache — every send enqueues a distinct message — and
	// DoNotCache results are detached in dagql: they have no addressable
	// ID, which would break cancel-and-request-again across
	// requests. Pin the handle's identity by re-exec (design §9, the same
	// trick step() uses to materialize state as selectors): a real Select
	// through the message lookup field, on the agent receiver instance,
	// yields an instance whose ID is the honest, replayable chain
	// `…agent(handle:…)!message(ref:"#3")` — addressable from any request in
	// the session. That pinned ID is the result: send is imperative, so like
	// the other verbs it returns ID! rather than the object, forcing lazy
	// clients to execute the (unrepeatable) enqueue exactly once and
	// re-hydrate the handle from the ID — which replays the lookup, not
	// the send.
	srv, err := core.CurrentDagqlServer(ctx)
	if err != nil {
		return res, err
	}
	var pinned dagql.ObjectResult[*core.AgentMessage]
	if err := srv.Select(ctx, parent, &pinned, dagql.Selector{
		Field: "message",
		Args: []dagql.NamedInput{
			{
				Name:  "ref",
				Value: dagql.NewString(msg.Ref),
			},
		},
	}); err != nil {
		return res, err
	}
	pinnedID, err := pinned.ID()
	if err != nil {
		return res, fmt.Errorf("message ID: %w", err)
	}
	return dagql.NewResultForCurrentCall(ctx, dagql.NewID[*core.AgentMessage](pinnedID))
}

func (s agentSchema) message(ctx context.Context, parent dagql.ObjectResult[*core.Agent], args struct {
	Ref string
}) (*core.AgentMessage, error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return nil, err
	}
	return agents.LookupMessage(ctx, parent, args.Ref)
}

func (s agentSchema) pause(ctx context.Context, parent dagql.ObjectResult[*core.Agent], args struct {
	Interrupt bool `default:"false"`
}) (res dagql.Result[core.AgentID], _ error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return res, err
	}
	rt, err := agents.Require(ctx, parent)
	if err != nil {
		return res, err
	}
	if args.Interrupt {
		err = rt.Interrupt()
	} else {
		err = rt.Pause()
	}
	if err != nil {
		return res, err
	}
	return agentSelfID(ctx, parent)
}

func (s agentSchema) resume(ctx context.Context, parent dagql.ObjectResult[*core.Agent], _ struct{}) (res dagql.Result[core.AgentID], _ error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return res, err
	}
	rt, err := agents.Require(ctx, parent)
	if err != nil {
		return res, err
	}
	if err := rt.Resume(ctx); err != nil {
		return res, err
	}
	return agentSelfID(ctx, parent)
}

func (s agentSchema) messageDelivery(ctx context.Context, msg *core.AgentMessage, _ struct{}) (core.AgentMessageDelivery, error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return "", err
	}
	return agents.MessageDelivery(ctx, msg)
}

func (s agentSchema) messageRef(ctx context.Context, msg *core.AgentMessage, _ struct{}) (string, error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return "", err
	}
	return agents.MessageRef(ctx, msg)
}

func (s agentSchema) messageResponse(ctx context.Context, msg *core.AgentMessage, _ struct{}) (string, error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return "", err
	}
	return agents.MessageResponse(ctx, msg)
}

func (s agentSchema) wait(ctx context.Context, parent dagql.ObjectResult[*core.Agent], _ struct{}) (res dagql.Result[core.AgentID], _ error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return res, err
	}
	rt, err := agents.Require(ctx, parent)
	if err != nil {
		return res, err
	}
	if _, err := rt.WaitSettled(ctx); err != nil {
		return res, err
	}
	return agentSelfID(ctx, parent)
}

func (s agentSchema) notify(ctx context.Context, parent dagql.ObjectResult[*core.Agent], args struct {
	Subscriber core.AgentID
	On         []core.AgentState `default:"[\"IDLE\",\"FAILED\"]"`
}) (res dagql.Result[core.AgentID], _ error) {
	srv, err := core.CurrentDagqlServer(ctx)
	if err != nil {
		return res, err
	}
	subscriber, err := args.Subscriber.Load(ctx, srv)
	if err != nil {
		return res, err
	}
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return res, err
	}
	if err := agents.Notify(ctx, parent, subscriber, args.On); err != nil {
		return res, err
	}
	return agentSelfID(ctx, parent)
}

func (s agentSchema) stop(ctx context.Context, parent dagql.ObjectResult[*core.Agent], args struct {
	Kill bool `default:"false"`
}) (res dagql.Result[core.AgentID], _ error) {
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return res, err
	}
	rt, err := agents.Require(ctx, parent)
	if err != nil {
		return res, err
	}
	// Somebody asked for this stop, which is what distinguishes it from the
	// one session teardown performs on every surviving entry — the terminal
	// telemetry record carries the difference (core.AgentStopReason).
	if err := rt.Stop(ctx, args.Kill, nil, core.AgentStopExplicit); err != nil {
		return res, err
	}
	return agentSelfID(ctx, parent)
}

// reseed swaps the receiver instance's committed conversation for the one
// given as an argument. Argument-shaped deliberately, unlike spawn: a
// runtime verb's receiver NAMES an instance and never redefines one — the
// invariant that makes telemetry-rebuilt handles safe to hold (design §10.2:
// the seed is read only at entry creation). The creation verb (LLM.spawn)
// reads the receiver as the seed because creation is the one moment a chain
// defines an entry; everything an already-created entry is told to do arrives
// as an argument.
func (s agentSchema) reseed(ctx context.Context, parent dagql.ObjectResult[*core.Agent], args struct {
	Conversation core.LLMID
}) (res dagql.Result[core.AgentID], _ error) {
	srv, err := core.CurrentDagqlServer(ctx)
	if err != nil {
		return res, err
	}
	conversation, err := args.Conversation.Load(ctx, srv)
	if err != nil {
		return res, err
	}
	agents, err := agentRuntimes(ctx)
	if err != nil {
		return res, err
	}
	conversation, err = core.WithLLMOwnerAgent(ctx, conversation, parent.Self().Handle)
	if err != nil {
		return res, err
	}
	if err := agents.Reseed(ctx, parent, conversation); err != nil {
		return res, err
	}
	return agentSelfID(ctx, parent)
}

// withAgentComposition records the composition scope and the conversation
// before cache lookup. An omitted conversation is pinned to the agent's
// current snapshot, so the result's recipe and cache key name the
// conversation it was composed from rather than whatever the snapshot is later.
func withAgentComposition(field dagql.Field[*core.Agent]) dagql.Field[*core.Agent] {
	field.Spec.GetDynamicInput = func(ctx context.Context, self dagql.AnyResult, _ map[string]dagql.Input, _ call.View, req *dagql.CallRequest) error {
		if err := core.StampExpertiseOwner(ctx, req); err != nil {
			return err
		}
		if req.Replay {
			return nil
		}
		if arg := req.Arg("conversation"); arg != nil && arg.Value != nil && arg.Value.Kind != dagql.ResultCallLiteralKindNull {
			return nil
		}
		agent, ok := self.(dagql.AnyObjectResult)
		if !ok {
			return fmt.Errorf("agent composition: unexpected receiver %T", self)
		}
		srv, err := core.CurrentDagqlServer(ctx)
		if err != nil {
			return err
		}
		var snapshot dagql.ObjectResult[*core.LLM]
		if err := srv.Select(ctx, agent, &snapshot, dagql.Selector{Field: "snapshot"}); err != nil {
			return fmt.Errorf("agent snapshot: %w", err)
		}
		id, err := snapshot.ID()
		if err != nil {
			return err
		}
		return req.SetArgInput(ctx, "conversation", dagql.Opt(dagql.NewID[*core.LLM](id)), false)
	}
	return field
}

type agentComposeArgs struct {
	Expertise    []dagql.ID[*core.Expertise]
	Conversation dagql.Optional[core.LLMID]
	Owner        dagql.Optional[dagql.String] `name:"_expertiseOwner" internal:"true"`
}

type agentRecomposeArgs struct {
	Conversation dagql.Optional[core.LLMID]
	Owner        dagql.Optional[dagql.String] `name:"_expertiseOwner" internal:"true"`
}

func (s agentSchema) compose(ctx context.Context, agent dagql.ObjectResult[*core.Agent], args agentComposeArgs) (dagql.ObjectResult[*core.LLM], error) {
	ctx = core.RestoreExpertiseOwner(ctx, string(args.Owner.Value))
	conversation, err := agentConversation(ctx, agent, args.Conversation)
	if err != nil {
		return conversation, err
	}
	expertise, err := (expertiseArgs{Expertise: args.Expertise}).load(ctx)
	if err != nil {
		return conversation, err
	}
	return core.ComposeExpertise(ctx, conversation, expertise)
}

func (s agentSchema) recompose(ctx context.Context, agent dagql.ObjectResult[*core.Agent], args agentRecomposeArgs) (dagql.ObjectResult[*core.LLM], error) {
	ctx = core.RestoreExpertiseOwner(ctx, string(args.Owner.Value))
	conversation, err := agentConversation(ctx, agent, args.Conversation)
	if err != nil {
		return conversation, err
	}
	return core.RecomposeExpertise(ctx, conversation)
}

// agentConversation loads the conversation an agent composes, owned by that
// agent: the agent is what every entry's Agent arguments receive.
func agentConversation(ctx context.Context, agent dagql.ObjectResult[*core.Agent], arg dagql.Optional[core.LLMID]) (dagql.ObjectResult[*core.LLM], error) {
	var llm dagql.ObjectResult[*core.LLM]
	srv, err := core.CurrentDagqlServer(ctx)
	if err != nil {
		return llm, err
	}
	if arg.Valid {
		llm, err = arg.Value.Load(ctx, srv)
	} else {
		err = srv.Select(ctx, agent, &llm, dagql.Selector{Field: "snapshot"})
	}
	if err != nil {
		return llm, err
	}
	handle := agent.Self().Handle
	if owner := llm.Self().OwnerAgentHandle; owner != "" && owner != handle {
		return llm, fmt.Errorf("the conversation belongs to agent handle %q, not agent %q (%s); compose it as its own agent", owner, agent.Self().Name, handle)
	}
	return core.WithLLMOwnerAgent(ctx, llm, handle)
}
