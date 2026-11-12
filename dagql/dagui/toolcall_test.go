package dagui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"

	telemetry "github.com/dagger/otel-go"
)

func TestNewToolCallView(t *testing.T) {
	// An ordinary call presents as itself.
	view := NewToolCallView("Read", map[string]any{"path": "main.go"})
	assert.Equal(t, "Read", view.Tool)
	assert.False(t, view.Wrapped())

	// A Timeout presents as the call it wraps, from JSON arguments...
	view = NewToolCallView("Timeout", map[string]any{
		"duration":  "2m",
		"tool":      "DangEval",
		"arguments": map[string]any{"source": "1 + 1", "at": "notes"},
	})
	assert.Equal(t, ToolCallView{
		Tool:    "DangEval",
		Args:    map[string]any{"source": "1 + 1", "at": "notes"},
		Timeout: "2m",
	}, view)
	assert.True(t, view.Wrapped())

	// ...or from span attributes, which carry them as a JSON string.
	view = NewToolCallView("Timeout", map[string]any{
		"duration":  "30s",
		"tool":      "read",
		"arguments": `{"path":"main.go","offset":20}`,
	})
	assert.Equal(t, "read", view.Tool)
	assert.Equal(t, "30s", view.Timeout)
	assert.Equal(t, []ToolArgPart{
		{Text: "main.go", Style: ToolArgPath},
		{Text: "offset=20", Style: ToolArgDescription},
	}, view.ArgsSummary())

	// A Timeout whose wrapped tool isn't known yet presents as itself.
	view = NewToolCallView("Timeout", map[string]any{"duration": "2m"})
	assert.Equal(t, "Timeout", view.Tool)
	assert.False(t, view.Wrapped())

	// A wrapped tool with no duration is still wrapped.
	view = NewToolCallView("Timeout", map[string]any{"tool": "frobnicate"})
	assert.Equal(t, "?", view.Timeout)
	assert.Nil(t, view.Args)
}

func TestToolCallViewArgsFallback(t *testing.T) {
	view := NewToolCallView("frobnicate", map[string]any{
		"name":  "first\nsecond",
		"count": 3.0,
		"opts":  map[string]any{"x": true},
	})
	// Unrecognized: no summary; the fallback is the first argument by name.
	assert.Empty(t, view.ArgsSummary())
	assert.Equal(t, "3", view.ArgsFallback())
	assert.Equal(t, "", NewToolCallView("frobnicate", nil).ArgsFallback())
}

func TestDangSnippet(t *testing.T) {
	snippet, ok := NewToolCallView("DangEval", map[string]any{"source": "1", "at": "notes"}).DangSnippet()
	require.True(t, ok)
	assert.Equal(t, DangSnippet{Source: "1", At: "notes"}, snippet)

	snippet, ok = NewToolCallView("DangCheck", map[string]any{"search": "sort"}).DangSnippet()
	require.True(t, ok)
	assert.Equal(t, DangSnippet{Search: "sort"}, snippet)

	_, ok = NewToolCallView("Read", map[string]any{"source": "x"}).DangSnippet()
	assert.False(t, ok)

	assert.True(t, RendersToolArgs("DangEval"))
	assert.True(t, RendersToolArgs("dangcheck"))
	assert.True(t, RendersToolArgs("Timeout"))
	assert.False(t, RendersToolArgs("Read"))
}

// TestToolCallArgsFromStreamedJSON covers a live tool call: its arguments
// stream as JSON logs on its display span before its attributes arrive
// (which only happens once it ends), and are what it renders from once the
// document is complete.
func TestToolCallArgsFromStreamedJSON(t *testing.T) {
	traceID := trace.TraceID{1}
	toolID := SpanID{SpanID: trace.SpanID{2}}
	plainID := SpanID{SpanID: trace.SpanID{3}}
	start := time.Unix(100, 0)
	db := NewDB()
	db.ImportSnapshots([]SpanSnapshot{
		{
			ID: toolID, TraceID: TraceID{traceID}, Name: "Timeout",
			LLMRole: "assistant", LLMTool: "Timeout", StartTime: start,
		},
		{
			ID: plainID, TraceID: TraceID{traceID}, Name: "Read",
			LLMRole: "assistant", LLMTool: "Read", StartTime: start,
		},
	})
	jsonLog := func(id SpanID, body string) {
		db.IngestLogs([]sdklog.Record{newTestLogRecord(traceID, id.SpanID, body,
			otellog.String(telemetry.ContentTypeAttr, "application/json"))})
	}
	tool := db.Spans.Map[toolID]

	// Nothing yet.
	assert.Nil(t, db.ToolCallArgs(tool))

	// Incomplete: not yet.
	jsonLog(toolID, `{"duration":"2m","tool":"DangEval",`)
	assert.Nil(t, db.ToolCallArgs(tool))
	assert.False(t, db.ToolCallView(tool).Wrapped())

	jsonLog(toolID, `"arguments":{"source":"1 + 1"}}`+"\n")
	view := db.ToolCallView(tool)
	assert.Equal(t, "DangEval", view.Tool)
	assert.Equal(t, "2m", view.Timeout)
	assert.Equal(t, map[string]any{"source": "1 + 1"}, view.Args)

	// Tools that don't render from their arguments keep nothing.
	jsonLog(plainID, `{"path":"main.go"}`)
	assert.Nil(t, db.ToolCallArgs(db.Spans.Map[plainID]))
}

func TestToolCallArgsFallsBackToAttributes(t *testing.T) {
	traceID := trace.TraceID{1}
	toolID := SpanID{SpanID: trace.SpanID{2}}
	db := NewDB()
	db.ImportSnapshots([]SpanSnapshot{{
		ID: toolID, TraceID: TraceID{traceID}, Name: "DangEval",
		LLMRole: "assistant", LLMTool: "DangEval",
		LLMToolArgNames:  []string{"at", "source"},
		LLMToolArgValues: []string{"notes", "shout(\"x\")"},
	}})
	tool := db.Spans.Map[toolID]
	assert.Equal(t, map[string]any{"at": "notes", "source": "shout(\"x\")"}, db.ToolCallArgs(tool))

	// Arguments too large to keep fall back to the attributes too.
	db.IngestLogs([]sdklog.Record{newTestLogRecord(traceID, toolID.SpanID,
		`{"source":"`+strings.Repeat("x", maxToolArgsJSON)+`"}`,
		otellog.String(telemetry.ContentTypeAttr, "application/json"))})
	assert.Equal(t, "notes", db.ToolCallArgs(tool)["at"])

	// No DB: attributes alone.
	var nilDB *DB
	assert.Equal(t, "notes", nilDB.ToolCallArgs(tool)["at"])
	assert.Nil(t, nilDB.ToolCallArgs(nil))
}
