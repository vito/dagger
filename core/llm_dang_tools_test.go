package core

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dagger/dagger/dagql"
)

// A DangEval snippet's writes are all or nothing: when one fails to commit,
// the ones before it are undone.
func TestCommitDangWritesAllOrNothing(t *testing.T) {
	t.Parallel()
	workspace := func(address string) dagql.ObjectResult[*Workspace] {
		res, err := dagql.NewResultForCall(&Workspace{Address: address}, &dagql.ResultCall{})
		require.NoError(t, err)
		return dagql.ObjectResult[*Workspace]{Result: res}
	}
	before, after := workspace("before"), workspace("after")

	m := &MCP{mu: &sync.Mutex{}, workspace: before, boundTools: []boundTool{{Owner: "tool"}}}
	writeWS := dangCommit{name: "ws", apply: func() (string, error) {
		m.workspace = after
		m.markStateChanged()
		return "ws written (1 file)\ndiff --git a/x b/x", nil
	}}
	writeTool := dangCommit{name: "tool", apply: func() (string, error) {
		m.mu.Lock()
		m.boundTools[0].Owner = "rebound"
		m.stateChanged = true
		m.mu.Unlock()
		return "tool state updated", nil
	}}
	failing := dangCommit{name: "broken", apply: func() (string, error) {
		return "", errors.New("boom")
	}}

	_, err := m.commitDangWrites([]dangCommit{writeWS, writeTool, failing})
	require.ErrorContains(t, err, "write broken: boom (nothing was written)")
	require.Equal(t, "before", m.workspace.Self().Address, "the workspace write is undone")
	require.Equal(t, "tool", m.boundTools[0].Owner, "the tool rebind is undone")
	require.False(t, m.stateChanged)

	out, err := m.commitDangWrites([]dangCommit{writeWS, writeTool})
	require.NoError(t, err)
	require.Equal(t, "ws written (1 file) · tool state updated\ndiff --git a/x b/x", out)
	require.Equal(t, "after", m.workspace.Self().Address)
	require.Equal(t, "rebound", m.boundTools[0].Owner)
	require.True(t, m.stateChanged)
}
