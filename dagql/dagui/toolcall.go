package dagui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	telemetry "github.com/dagger/otel-go"
)

// This file extends toolargs.go for the tool calls whose arguments, not just
// a header summary, shape how they're shown: the Dang builtins, whose
// snippet is the call, and Timeout, which presents as the call it wraps.

// Tool names whose calls render from their full arguments.
const (
	dangEvalTool  = "DangEval"
	dangCheckTool = "DangCheck"
	timeoutTool   = "Timeout"
)

// RendersToolArgs reports whether a tool's calls render from their full
// arguments (see DB.ToolCallView), so the DB keeps the JSON arguments its
// display span streams.
func RendersToolArgs(tool string) bool {
	return ToolNameIs(tool, dangEvalTool) ||
		ToolNameIs(tool, dangCheckTool) ||
		ToolNameIs(tool, timeoutTool)
}

// maxToolArgsJSON bounds the streamed JSON arguments kept per tool call.
// Arguments past it are dropped, and the call falls back to its span
// attributes.
const maxToolArgsJSON = 256 << 10

// ingestToolArgs keeps the JSON arguments a tool-call display span streams,
// for tools that render from them. A span's attributes only reach a live
// client when it starts and ends, so the arguments the tool recorded on its
// span arrive only once the call is over; its streamed arguments are how a
// running call shows what it runs.
func (db *DB) ingestToolArgs(record sdklog.Record, spanID SpanID, body string) {
	if !isJSONRecord(record) {
		return
	}
	span := db.Spans.Map[spanID]
	if span == nil {
		return
	}
	if span.Received && (span.LLMRole == "" || !RendersToolArgs(span.LLMTool)) {
		// Not a tool call that renders from its arguments. A span not
		// received yet is a placeholder whose tool isn't known: keep its
		// arguments in case it is one.
		return
	}
	if db.toolArgsJSON == nil {
		db.toolArgsJSON = map[SpanID]*toolArgsJSON{}
	}
	args := db.toolArgsJSON[spanID]
	if args == nil {
		args = &toolArgsJSON{}
		db.toolArgsJSON[spanID] = args
	}
	if args.overflowed {
		return
	}
	if args.buf.Len()+len(body) > maxToolArgsJSON {
		// Too large to keep whole: a truncated document wouldn't parse, so
		// drop it and let the attributes speak.
		args.buf.Reset()
		args.overflowed = true
		return
	}
	args.buf.WriteString(body)
	db.update(span)
}

// toolArgsJSON accumulates the JSON arguments a tool-call display span
// streams.
type toolArgsJSON struct {
	buf        strings.Builder
	overflowed bool

	// parsed is buf decoded as of parsedLen bytes; nil while it doesn't
	// parse (still streaming).
	parsed    map[string]any
	parsedLen int
}

// args returns the arguments, once the streamed document is complete.
func (t *toolArgsJSON) args() map[string]any {
	if t.overflowed {
		return nil
	}
	if t.buf.Len() != t.parsedLen {
		t.parsedLen = t.buf.Len()
		t.parsed = nil
		var args map[string]any
		if err := json.Unmarshal([]byte(t.buf.String()), &args); err == nil {
			t.parsed = args
		}
	}
	return t.parsed
}

func isJSONRecord(record sdklog.Record) bool {
	var isJSON bool
	record.WalkAttributes(func(kv otellog.KeyValue) bool {
		if kv.Key == telemetry.ContentTypeAttr {
			ct, _ := LogValueString(kv.Value)
			isJSON = ct == "application/json"
			return false
		}
		return true
	})
	return isJSON
}

// ToolCallArgs returns a tool call's arguments: the JSON its display span
// streamed once it is complete, else the string values its span attributes
// recorded (see SpanSnapshot.LLMToolArgNames). Nil when neither is known.
func (db *DB) ToolCallArgs(span *Span) map[string]any {
	if span == nil {
		return nil
	}
	if db != nil {
		if streamed := db.toolArgsJSON[span.ID]; streamed != nil {
			if args := streamed.args(); args != nil {
				return args
			}
		}
	}
	return span.toolArgsFromAttrs()
}

func (snapshot *SpanSnapshot) toolArgsFromAttrs() map[string]any {
	names, values := snapshot.LLMToolArgNames, snapshot.LLMToolArgValues
	n := min(len(names), len(values))
	if n == 0 {
		return nil
	}
	args := make(map[string]any, n)
	for i := range n {
		args[names[i]] = values[i]
	}
	return args
}

// ToolCallView is how a tool call presents: the tool it runs and the
// arguments it runs it with. A Timeout call presents as the call it wraps,
// with its duration noted.
type ToolCallView struct {
	// Tool is the tool the call runs: the wrapped tool, for Timeout.
	Tool string
	// Args are the arguments it runs it with.
	Args map[string]any
	// Timeout is the duration a Timeout wrapper runs it under, or "".
	Timeout string
}

// ToolCallView returns how a tool-call span presents (see ToolCallView).
func (db *DB) ToolCallView(span *Span) ToolCallView {
	if span == nil {
		return ToolCallView{}
	}
	return NewToolCallView(span.LLMTool, db.ToolCallArgs(span))
}

// NewToolCallView returns how a call of tool with args presents, unwrapping
// a Timeout whose wrapped tool is known. A Timeout whose arguments haven't
// arrived yet presents as itself.
func NewToolCallView(tool string, args map[string]any) ToolCallView {
	view := ToolCallView{Tool: tool, Args: args}
	if !ToolNameIs(tool, timeoutTool) {
		return view
	}
	inner, _ := args["tool"].(string)
	if strings.TrimSpace(inner) == "" {
		return view
	}
	duration, _ := args["duration"].(string)
	if strings.TrimSpace(duration) == "" {
		duration = "?"
	}
	var innerArgs map[string]any
	switch v := args["arguments"].(type) {
	case map[string]any:
		innerArgs = v
	case string:
		// Span attributes carry it as JSON.
		_ = json.Unmarshal([]byte(v), &innerArgs)
	}
	return ToolCallView{Tool: inner, Args: innerArgs, Timeout: duration}
}

// Wrapped reports whether the call runs under a Timeout wrapper.
func (view ToolCallView) Wrapped() bool {
	return view.Timeout != ""
}

// ArgsSummary is SpanSnapshot.ToolArgsSummary for the view's tool and
// arguments.
func (view ToolCallView) ArgsSummary() []ToolArgPart {
	names, values := view.headerArgs()
	return toolArgsSummary(view.Tool, names, values)
}

// ArgsFallback is SpanSnapshot.ToolArgsFallback for the view's tool and
// arguments: the first line of its first argument, by name.
func (view ToolCallView) ArgsFallback() string {
	_, values := view.headerArgs()
	if len(values) == 0 {
		return ""
	}
	return SanitizeInline(firstLine(values[0]))
}

// headerArgs flattens the view's arguments into header values, in name
// order, as the engine records them on a span: strings as they are, other
// scalars formatted, lists joined by spaces, objects as JSON.
func (view ToolCallView) headerArgs() (names, values []string) {
	for _, name := range sortedKeys(view.Args) {
		val, ok := headerArgValue(view.Args[name])
		if !ok {
			continue
		}
		names = append(names, name)
		values = append(values, val)
	}
	return names, values
}

func headerArgValue(v any) (string, bool) {
	switch v := v.(type) {
	case nil:
		return "", false
	case string:
		return v, true
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, " "), true
	case map[string]any:
		enc, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(enc), true
	default:
		return fmt.Sprint(v), true
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// DangSnippet is a Dang builtin's call: DangEval's or DangCheck's
// arguments.
type DangSnippet struct {
	// Source is the Dang the call runs or checks; "" for a DangCheck of a
	// scope alone.
	Source string
	// At is the scope: a module, file, or path:line:col; "" for the
	// workspace scope.
	At string
	// Search is DangCheck's search, if any.
	Search string
}

// DangSnippet returns the call's Dang snippet, if it is a call of a Dang
// builtin (DangEval or DangCheck).
func (view ToolCallView) DangSnippet() (DangSnippet, bool) {
	if !ToolNameIs(view.Tool, dangEvalTool) && !ToolNameIs(view.Tool, dangCheckTool) {
		return DangSnippet{}, false
	}
	str := func(name string) string {
		s, _ := view.Args[name].(string)
		return s
	}
	return DangSnippet{Source: str("source"), At: str("at"), Search: str("search")}, true
}
