package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

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

// dangToolCallCtx is a context in a conversation's tool call, with its span,
// as a Dang tool's call runs in.
func dangToolCallCtx(t *testing.T, spanID string) context.Context {
	t.Helper()
	traceID, err := trace.TraceIDFromHex("000102030405060708090a0b0c0d0e0f")
	require.NoError(t, err)
	span, err := trace.SpanIDFromHex(spanID)
	require.NoError(t, err)
	ctx := context.WithValue(t.Context(), agentToolCallKey{}, true)
	return trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
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
		require.Regexp(t, `\n\.\.\. \d+ lines omitted \(use ReadLogs\(span: `+spanID+`, scope: "own"\) to read more\) \.\.\.\n`, got)
		// Both ends survive: the value's head, and the footer.
		require.True(t, strings.HasPrefix(got, "String! = \"\"\"\nline 0000 of a long file\n"), got)
		require.True(t, strings.HasSuffix(got, "line 1999 of a long file\n\"\"\"\nok · 1ms · 1 engine call · workspace scope"), got)
	})

	t.Run("a long line is clamped once, on a rune boundary", func(t *testing.T) {
		got := m.dangToolResult(ctx, DangReport{Text: "String! = \"" + strings.Repeat("日", llmLogsMaxLineLen) + "\"\n"})
		require.Contains(t, got, "bytes truncated]")
		require.LessOrEqual(t, len(got), llmLogsMaxLineLen)
		require.True(t, utf8.ValidString(got))
		require.Equal(t, got, guardToolResult(got), "the result guard leaves it be")
	})

	t.Run("outside a conversation's tool call, the report is only guarded", func(t *testing.T) {
		// dagger mcp: no span of the call's own to point at.
		report := strings.Repeat("a line of the report\n", 2000)
		require.Equal(t, report, m.dangToolResult(t.Context(), DangReport{Text: report}))
		long := strings.Repeat("a line of the report\n", 5000)
		got := m.dangToolResult(trace.ContextWithSpanContext(t.Context(), trace.SpanContextFromContext(ctx)), DangReport{Text: long})
		require.Equal(t, guardToolResult(long), got)
		require.NotContains(t, got, "ReadLogs")
	})
}

// A Dang tool's source that fails to parse or type-check points at the
// dang-language skill, until the conversation reads it.
func TestDangToolResultSkillNudge(t *testing.T) {
	t.Parallel()
	ctx := dangToolCallCtx(t, "00000000000000d2")
	typeError := DangReport{Text: "<check>:1:1: error: \"nope\" not found\n1 error · workspace scope · nothing ran\n", Rejected: true}
	parseError := DangReport{Text: "<check>:1:5: error: unexpected end of input\n1 error · workspace scope · nothing ran\n", Rejected: true}
	runtimeError := DangReport{Text: "error: boom\nfailed · 1ms · 0 engine calls · workspace scope\n"}
	success := DangReport{Text: "Int! = 2\nok · 1ms · 0 engine calls · workspace scope\n"}

	m := newMCP()
	for _, rep := range []DangReport{typeError, parseError} {
		got := m.dangToolResult(ctx, rep)
		require.True(t, strings.HasSuffix(got, "nothing ran\n"+dangSkillNudge), got)
	}
	for _, rep := range []DangReport{runtimeError, success} {
		require.NotContains(t, m.dangToolResult(ctx, rep), "skill")
	}

	// Reading the skill, with ReadSkill as the model calls it, stops it.
	tools := NewLLMToolSet()
	m.loadSkillTools(&dagql.Server{}, tools)
	readSkillTool, ok := tools.Map["ReadSkill"]
	require.True(t, ok)
	_, err := readSkillTool.Call(ctx, map[string]any{"name": "nope"})
	require.Error(t, err)
	require.Contains(t, m.dangToolResult(ctx, typeError), dangSkillNudge, "a failed read reads nothing")
	_, err = readSkillTool.Call(ctx, map[string]any{"name": dangSkillName})
	require.NoError(t, err)
	require.NotContains(t, m.dangToolResult(ctx, typeError), "skill")
	require.NotContains(t, m.dangToolResult(ctx, parseError), "skill")
	// A step's reads are its own: later steps read the conversation's
	// record, which a bare clone doesn't have.
	require.Contains(t, m.Clone().dangToolResult(ctx, typeError), dangSkillNudge)
}

// A conversation has read a skill when its record holds a ReadSkill call
// naming it that succeeded.
func TestMessagesReadSkill(t *testing.T) {
	t.Parallel()
	call := func(id, tool, args string) *LLMMessage {
		return &LLMMessage{Role: LLMMessageRoleAssistant, Content: []*LLMContentBlock{
			{Kind: LLMContentToolCall, CallID: id, ToolName: tool, Arguments: JSON(args)},
		}}
	}
	result := func(id string, errored bool) *LLMMessage {
		return &LLMMessage{Role: LLMMessageRoleUser, Content: []*LLMContentBlock{
			{Kind: LLMContentToolResult, CallID: id, Text: "…", Errored: errored},
		}}
	}
	for _, tc := range []struct {
		name     string
		messages []*LLMMessage
		read     bool
	}{
		{"none", nil, false},
		{"read", []*LLMMessage{call("1", "ReadSkill", `{"name":"dang-language"}`), result("1", false)}, true},
		{"a reference file", []*LLMMessage{call("1", "ReadSkill", `{"name":"dang-language","file":"reference/objects.md"}`), result("1", false)}, true},
		{"another skill", []*LLMMessage{call("1", "ReadSkill", `{"name":"other"}`), result("1", false)}, false},
		{"failed", []*LLMMessage{call("1", "ReadSkill", `{"name":"dang-language"}`), result("1", true)}, false},
		{"not answered yet", []*LLMMessage{call("1", "ReadSkill", `{"name":"dang-language"}`)}, false},
		{"another tool", []*LLMMessage{call("1", "DangEval", `{"name":"dang-language"}`), result("1", false)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.read, messagesReadSkill(tc.messages, dangSkillName))
		})
	}
}
