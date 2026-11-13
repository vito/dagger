package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

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

// dangToolCallCtx is a context in a tool call's span, as a Dang tool's call
// runs in.
func dangToolCallCtx(t *testing.T, spanID string) context.Context {
	t.Helper()
	traceID, err := trace.TraceIDFromHex("000102030405060708090a0b0c0d0e0f")
	require.NoError(t, err)
	span, err := trace.SpanIDFromHex(spanID)
	require.NoError(t, err)
	return trace.ContextWithSpanContext(t.Context(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  span,
	}))
}

// A Dang tool's report is bounded as a module tool's printed output is: the
// whole report is the tool call's logs, and a long one gives way in the
// middle to a pointer to ReadLogs on the call's span.
func TestDangToolResultBounds(t *testing.T) {
	t.Parallel()
	const spanID = "00000000000000d1"
	ctx := dangToolCallCtx(t, spanID)
	m := newMCP()

	t.Run("a report within bounds is shown whole", func(t *testing.T) {
		report := "output:\nhi\nString! = \"x\"\nok · 1ms · 0 engine calls · workspace scope\n"
		require.Equal(t, strings.TrimSuffix(report, "\n"), m.dangToolResult(ctx, DangReport{Text: report}))
	})

	t.Run("a long report points at ReadLogs", func(t *testing.T) {
		var report strings.Builder
		report.WriteString("String! = \"\"\"\n")
		for i := range 2000 {
			fmt.Fprintf(&report, "line %04d of a long file\n", i)
		}
		report.WriteString("\"\"\"\nok · 1ms · 1 engine call · workspace scope\n")
		got := m.dangToolResult(ctx, DangReport{Text: report.String()})
		require.LessOrEqual(t, len(got), llmToolLogsMaxBytes+128)
		require.Regexp(t, `\n\.\.\. \d+ lines omitted \(use ReadLogs\(span: `+spanID+`\) to read more\) \.\.\.\n`, got)
		// Both ends survive: the value's head, and the footer.
		require.True(t, strings.HasPrefix(got, "String! = \"\"\"\nline 0000 of a long file\n"), got)
		require.True(t, strings.HasSuffix(got, "line 1999 of a long file\n\"\"\"\nok · 1ms · 1 engine call · workspace scope"), got)
	})

	t.Run("a long line is clamped", func(t *testing.T) {
		got := m.dangToolResult(ctx, DangReport{Text: "String! = \"" + strings.Repeat("x", 3*llmLogsMaxLineLen) + "\"\n"})
		require.Contains(t, got, "chars truncated]")
		require.Less(t, len(got), 2*llmLogsMaxLineLen)
	})

	t.Run("without a span, the report is left to the result guard", func(t *testing.T) {
		report := strings.Repeat("a line of the report\n", 2000)
		require.Equal(t, report, m.dangToolResult(t.Context(), DangReport{Text: report}))
	})
}
