package core

import "context"

type agentToolCallKey struct{}

// WithAgentToolCall marks ctx as executing an agent tool call. Only MCP tool
// dispatch (and tests exercising the gates it triggers) may set it.
func WithAgentToolCall(ctx context.Context) context.Context {
	return context.WithValue(ctx, agentToolCallKey{}, true)
}

// IsAgentToolCall distinguishes tools executing core APIs in the owner's
// context from direct user API calls. It is internal context, not API input.
func IsAgentToolCall(ctx context.Context) bool {
	marked, _ := ctx.Value(agentToolCallKey{}).(bool)
	return marked
}
