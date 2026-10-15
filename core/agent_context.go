package core

import (
	"context"

	"github.com/dagger/dagger/dagql"
)

type agentContextKey struct{}
type composingAgentContextKey struct{}

const composingAgentArg = "_composingAgent"

func composingAgentInputSpec() dagql.InputSpec {
	return dagql.InputSpec{Name: composingAgentArg, Type: dagql.Optional[dagql.String]{}, Internal: true}
}

func stampComposingAgent(ctx context.Context, req *dagql.CallRequest) error {
	if req.Replay {
		return nil
	}
	if arg := req.Arg(composingAgentArg); arg != nil && arg.Value != nil && arg.Value.Kind != dagql.ResultCallLiteralKindNull {
		return nil
	}
	if handle := composingAgentHandle(ctx); handle != "" {
		return req.SetArgInput(ctx, composingAgentArg, dagql.Opt(dagql.String(handle)), false)
	}
	return nil
}

func composingAgentCallContext(ctx context.Context, opts *CallOpts) context.Context {
	if opts.useRecordedExpertiseOwner {
		// An absent recorded scope must not inherit the replay caller's agent.
		ctx = WithComposingAgent(ctx, "")
	}
	for _, input := range opts.Inputs {
		if input.Name != composingAgentArg {
			continue
		}
		if handle, ok := input.Value.(dagql.Optional[dagql.String]); ok && handle.Valid {
			return WithComposingAgent(ctx, string(handle.Value))
		}
	}
	return ctx
}

// WithComposingAgent identifies whose initial conversation spawn is composing.
// This affects only the lineage of agents spawned during that composition;
// CallerAgent remains unchanged for messages and deadlock detection.
func WithComposingAgent(ctx context.Context, handle string) context.Context {
	return context.WithValue(ctx, composingAgentContextKey{}, handle)
}

func composingAgentHandle(ctx context.Context) string {
	if handle, ok := ctx.Value(composingAgentContextKey{}).(string); ok {
		return handle
	}
	query, err := CurrentQuery(ctx)
	if err != nil {
		return ""
	}
	fnCall, err := query.CurrentFunctionCall(ctx)
	if err != nil || fnCall == nil {
		return ""
	}
	return fnCall.composingAgentHandle
}

// AgentToContext binds a running agent's handle into the context so that a
// tool invoked from its evaluation loop can receive it: a module function
// declaring an `Agent!` argument has it auto-filled from here, exactly as an
// `LLM!` argument is auto-filled from [LLMToContext]. This is the
// child->parent channel of hack/designs/async-agents.md §3.1: a spawned
// worker's tool can message (steer) the agent that called it.
//
// It is only ever set by the agent loop (AgentRuntime.loop), on the context
// every Step — and therefore every tool dispatch within it — descends from.
// A synchronous LLM.loop or a direct function call never carries an agent, so
// tools requiring one are not offered there (see implicitToolArgs).
func AgentToContext(ctx context.Context, agent dagql.ObjectResult[*Agent]) context.Context {
	return context.WithValue(ctx, agentContextKey{}, agent)
}

// AgentFromContext returns the agent handle bound into the context by
// [AgentToContext], if any.
func AgentFromContext(ctx context.Context) (dagql.ObjectResult[*Agent], bool) {
	if agent, ok := ctx.Value(agentContextKey{}).(dagql.ObjectResult[*Agent]); ok && agent.Self() != nil {
		return agent, true
	}
	return dagql.ObjectResult[*Agent]{}, false
}

// CallerAgent resolves the agent on whose behalf the current execution runs:
// the ambient loop context (a tool dispatched in-process by an agent's step),
// or — for module functions and every nested API call they make — the calling
// client's function-call record, which modfunc stamps with the ambient agent
// at dispatch time. This is what lets the central enqueue path resolve
// message provenance (hack/designs/agent-messaging.md §4.1) without any
// forgeable "from" argument: a module cannot claim an agent it was not
// actually called from.
func CallerAgent(ctx context.Context) (dagql.ObjectResult[*Agent], bool) {
	if agent, ok := AgentFromContext(ctx); ok {
		return agent, true
	}
	query, err := CurrentQuery(ctx)
	if err != nil {
		return dagql.ObjectResult[*Agent]{}, false
	}
	fnCall, err := query.CurrentFunctionCall(ctx)
	if err != nil || fnCall == nil {
		return dagql.ObjectResult[*Agent]{}, false
	}
	return fnCall.CallerAgent()
}
