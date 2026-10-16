package core

import (
	"testing"

	"github.com/dagger/dagger/dagql"
	"github.com/stretchr/testify/require"
)

// A spawn's seed may end in a prompt (llm.withPrompt("x").spawn()). Completing
// the composition must leave that input for the loop to step — spawn's and
// resume's contract — rather than holding it the way a reseed does.
func TestCompleteCompositionLeavesSeedInputUnheld(t *testing.T) {
	_, ctx := stateRecorderCtx(t)
	cache, err := dagql.NewCache(ctx, "", nil, nil)
	require.NoError(t, err)
	ctx = dagql.ContextWithCache(ctx, cache)
	srv := newCoreDagqlServerForTest(t, &Query{})
	srv.InstallObject(dagql.NewClass[*LLM](srv))
	prompt := &LLMMessage{Role: LLMMessageRoleUser, Content: []*LLMContentBlock{{Kind: LLMContentText, Text: "x"}}}
	frame := testResultCall("withPrompt", &LLM{}, testResultCall("llm", &LLM{}, nil))
	seed, err := dagql.NewObjectResultForCall(&LLM{Messages: []*LLMMessage{prompt}, OwnerAgentHandle: "test"}, srv, frame)
	require.NoError(t, err)
	require.True(t, seed.Self().HasPending())

	rt := testRuntime(t, ctx)
	rt.composing = true
	rt.held = true // a stale hold must not survive the composed seed's commit
	require.NoError(t, rt.CompleteComposition(ctx, rt.self, seed))
	require.False(t, rt.held, "the seed's pending input is the loop's to step on send or resume")
	require.False(t, rt.composing)
	require.Same(t, seed.Self(), rt.Snapshot().Self())
	require.Equal(t, AgentStateIdle, rt.State(), "a never-started agent is IDLE until a send or resume starts it")

	// Once a loop is live, the unheld pending seed projects as work about to
	// step, not as a completed turn.
	rt.testTransition(func() { rt.started = true })
	require.Equal(t, AgentStateRunning, rt.State())
}
