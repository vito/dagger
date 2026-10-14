package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComposingAgentContext(t *testing.T) {
	ctx := WithComposingAgent(t.Context(), "family")
	for range 3 {
		require.Equal(t, "family", composingAgentHandle(ctx))
		_, caller := CallerAgent(ctx)
		require.False(t, caller, "composition lineage must not create caller provenance")
		fnCall := newFunctionCall(FunctionCall{composingAgentHandle: composingAgentHandle(ctx)})
		ctx = ContextWithQuery(t.Context(), NewRoot(&mockServer{functionCall: fnCall}))
	}
	require.Equal(t, "family", composingAgentHandle(ctx))
	require.Equal(t, "child", composingAgentHandle(WithComposingAgent(ctx, "child")))
	require.Empty(t, composingAgentHandle(WithComposingAgent(ctx, "")))
}

func TestAgentToolCallContext(t *testing.T) {
	ctx := t.Context()
	require.False(t, IsAgentToolCall(ctx))
	m := &MCP{}
	called := false
	_, failed := m.Call(ctx, []LLMTool{{
		Name: "push",
		Call: func(ctx context.Context, _ any) (any, error) {
			called = true
			require.True(t, IsAgentToolCall(ctx))
			return "ok", nil
		},
	}}, &LLMToolCall{Name: "push"})
	require.True(t, called)
	require.False(t, failed)
	require.False(t, IsAgentToolCall(ctx), "tool scope must not leak back into caller")
}
