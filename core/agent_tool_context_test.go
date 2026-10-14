package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/dagger/dagger/dagql"
	"github.com/dagger/dagger/dagql/call"
	"github.com/dagger/dagger/engine"
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

func TestComposingAgentCallCacheAndReplay(t *testing.T) {
	var newLease func(engine.ClientLeaseKind, string) *engine.ClientLifecycleLease
	newLease = func(kind engine.ClientLeaseKind, owner string) *engine.ClientLifecycleLease {
		return engine.NewClientLifecycleLease(kind, owner, nil, func(kind engine.ClientLeaseKind, owner string) (*engine.ClientLifecycleLease, error) {
			return newLease(kind, owner), nil
		})
	}
	lease := newLease(engine.ClientLeaseRequest, "test")
	t.Cleanup(lease.Release)
	scope, err := engine.NewClientScope(&engine.ClientMetadata{ClientID: "client", SessionID: "session"}, lease)
	require.NoError(t, err)
	ctx, err := engine.ContextWithClientScope(t.Context(), scope)
	require.NoError(t, err)
	cache, err := dagql.NewCache(ctx, "", nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cache.Close(context.Background())) })
	ctx = dagql.ContextWithCache(ctx, cache)
	srv, err := dagql.NewServer(ctx, &Query{})
	require.NoError(t, err)
	srv.InstallObject(dagql.NewClass[*Agent](srv))
	srv.InstallObject(dagql.NewClass[*Module](srv))
	mod, err := dagql.NewObjectResultForCall(&Module{}, srv, testResultCall("module", &Module{}, nil))
	require.NoError(t, err)
	fn := &ModuleFunction{mod: mod, metadata: &Function{}}
	registry := NewAgentRuntimes()
	t.Cleanup(func() { require.NoError(t, registry.KillAll(context.Background(), nil)) })
	for _, handle := range []string{"parent-A", "parent-B"} {
		parentCtx := testAgentContext(t, ctx, handle, handle)
		parent, _ := AgentFromContext(parentCtx)
		_, err := registry.Create(parentCtx, parent, AgentStateIdle, "", false, "")
		require.NoError(t, err)
	}
	calls := 0
	// No public arguments: only the engine's recorded scopes distinguish the
	// same spawning helper invoked by the same entry for two different parents.
	field := dagql.NodeFunc("spawnHelper", func(ctx context.Context, _ dagql.ObjectResult[*Query], args struct {
		Owner     dagql.Optional[dagql.String] `name:"_expertiseOwner" internal:"true"`
		Composing dagql.Optional[dagql.String] `name:"_composingAgent" internal:"true"`
	}) (dagql.ObjectResult[*Agent], error) {
		opts := &CallOpts{useRecordedExpertiseOwner: true, Inputs: []CallInput{
			{Name: expertiseOwnerArg, Value: args.Owner},
			{Name: composingAgentArg, Value: args.Composing},
		}}
		ctx = composingAgentCallContext(expertiseCallContext(ctx, opts), opts)
		calls++
		child, err := dagql.NewObjectResultForCurrentCall(ctx, srv, &Agent{Handle: fmt.Sprintf("child-%d", calls), Name: "child"})
		if err != nil {
			return child, err
		}
		_, err = registry.Create(ctx, child, AgentStateIdle, "", false, "")
		return child, err
	})
	field.Spec.GetDynamicInput = fn.DynamicInputsForCall
	dagql.Fields[*Query]{field}.Install(srv)

	for _, handle := range []string{"parent-A", "parent-B", ""} {
		callCtx := ctx
		if handle != "" {
			callCtx = WithComposingAgent(WithExpertiseOwner(ctx, "same-entry-uri"), handle)
		}
		before := calls
		var child dagql.ObjectResult[*Agent]
		for range 2 {
			require.NoError(t, srv.Select(callCtx, srv.Root(), &child, dagql.Selector{Field: "spawnHelper"}))
			rt, err := registry.Require(ctx, child)
			require.NoError(t, err)
			require.Equal(t, handle, rt.ParentHandle())
		}
		require.Equal(t, before+1, calls)
		recipe, err := child.RecipeID(callCtx)
		require.NoError(t, err)
		if handle == "" {
			require.Nil(t, recipe.Arg(composingAgentArg))
			legacy := call.New().Append((&Agent{}).Type(), "spawnHelper")
			require.Equal(t, legacy.Digest(), recipe.Digest(), "ordinary module call IDs stay unchanged")
		} else {
			require.Equal(t, handle, recipe.Arg(composingAgentArg).Value().ToInput())
		}
		cold, err := dagql.NewCache(ctx, "", nil, nil)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, cold.Close(context.Background())) })
		replayCtx := WithComposingAgent(dagql.ContextWithCache(ctx, cold), "different-replay-parent")
		replayed, err := srv.Load(replayCtx, recipe)
		require.NoError(t, err)
		rt, err := registry.Require(replayCtx, replayed.(dagql.ObjectResult[*Agent]))
		require.NoError(t, err)
		require.Equal(t, handle, rt.ParentHandle(), "cold replay restores present and absent lineage scopes")
	}
	inputs, err := fn.setCallInputs(ctx, &CallOpts{Inputs: []CallInput{{Name: composingAgentArg, Value: dagql.Opt(dagql.String("parent-A"))}}})
	require.NoError(t, err)
	require.Empty(t, inputs, "the scope must not become a module SDK argument")
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
