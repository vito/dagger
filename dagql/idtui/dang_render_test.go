package idtui

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vito/tuist"

	"github.com/dagger/dagger/dagql/dagui"
)

func TestRenderDangSource(t *testing.T) {
	source := "let greeting = shout(\"hi\") # loud\n\tgreeting + \"!\"\n"
	assert.Equal(t, []string{
		`1 │ let greeting = shout("hi") # loud`,
		`2 │   greeting + "!"`,
	}, renderDangSource(termenv.Ascii, source, 0, 0))

	// Narrow: lines wrap under a blank line number, never past width.
	got := renderDangSource(termenv.Ascii, source, 16, 0)
	assert.Equal(t, []string{
		`1 │ let greeting`,
		`  │  = shout("hi`,
		`  │ ") # loud`,
		`2 │   greeting +`,
		`  │  "!"`,
	}, got)
	for _, line := range got {
		assert.LessOrEqual(t, ansi.StringWidth(line), 16, line)
	}

	// Too narrow to wrap usefully: lines keep a minimum width.
	assert.Equal(t, `1 │ let gree`, renderDangSource(termenv.Ascii, source, 3, 0)[0])

	// Long: the first lines, then how many are left; line numbers are as
	// wide as the shown ones need.
	var long []string
	for i := range 12 {
		long = append(long, fmt.Sprintf("print(%d)", i))
	}
	assert.Equal(t, []string{
		` 1 │ print(0)`,
		` 2 │ print(1)`,
		` 3 │ print(2)`,
		` 4 │ print(3)`,
		` 5 │ print(4)`,
		` 6 │ print(5)`,
		` 7 │ print(6)`,
		` 8 │ print(7)`,
		` 9 │ print(8)`,
		`10 │ print(9)`,
		`   ┆ … 2 more lines`,
	}, renderDangSource(termenv.Ascii, strings.Join(long, "\n"), 0, 10))
	assert.Len(t, renderDangSource(termenv.Ascii, strings.Join(long, "\n"), 0, 0), 12)
	assert.Equal(t, `  ┆ … 1 more line`, renderDangSource(termenv.Ascii, "1\n2", 0, 1)[1])
}

func TestRenderDangSourceHighlights(t *testing.T) {
	got := strings.Join(renderDangSource(termenv.ANSI, `let x = "hi" # c`, 0, 0), "\n")
	assert.Contains(t, got, "\x1b[35mlet\x1b[0m", "keyword: magenta")
	assert.Contains(t, got, "\x1b[32m\"hi\"\x1b[0m", "string: green")
	assert.Contains(t, got, "\x1b[90m# c\x1b[0m", "comment: bright black")
	assert.Contains(t, got, "\x1b[90;2m1 │ \x1b[0m", "gutter: faint")

	// No color: no escapes at all.
	plain := renderDangSource(termenv.Ascii, `let x = "hi" # c`, 0, 0)
	assert.NotContains(t, strings.Join(plain, "\n"), "\x1b")
}

// TestRenderDangSourceNeverChangesText is the highlighter's contract with
// the view: whatever it makes of the source -- unparseable, partial,
// multibyte, with control characters -- styling only adds escapes, and
// wrapping holds at any width.
func TestRenderDangSourceNeverChangesText(t *testing.T) {
	for _, source := range []string{
		`let x = `,
		`"unterminated`,
		"}}}{{{ ### \"\"\" @@@",
		"pub f(x: Int!): Int! {\n  x * 2\n}\n\nf(21)",
		"print(\"héllo wörld 日本語 🎉\")",
		"a\r\nb\x1b[31mc\x00\td",
		"",
	} {
		for _, width := range []int{0, 12, 40} {
			plain := renderDangSource(termenv.Ascii, source, width, 0)
			styled := renderDangSource(termenv.ANSI, source, width, 0)
			require.Len(t, styled, len(plain), "%q at %d", source, width)
			for i := range plain {
				assert.Equal(t, plain[i], ansi.Strip(styled[i]), "%q at %d", source, width)
				assert.NotContains(t, plain[i], "\x1b", "%q at %d", source, width)
				if width > 0 {
					assert.LessOrEqual(t, ansi.StringWidth(styled[i]), max(width, 12), "%q at %d", source, width)
				}
			}
		}
	}
}

// dangTranscript renders a shell transcript of the given tool calls and
// returns its lines, ANSI-stripped and right-trimmed, from the first tool
// call on.
func dangTranscript(t *testing.T, width int, profile termenv.Profile, calls ...dagui.SpanSnapshot) ([]string, string) {
	t.Helper()
	db := dagui.NewDB()
	rootID := prettyTestSpanID(1)
	start := time.Unix(100, 0)
	snapshots := []dagui.SpanSnapshot{{
		ID: rootID, TraceID: prettyTestTraceID(), Name: "shell",
		StartTime: start, EndTime: start.Add(time.Minute), Final: true,
	}}
	for i, call := range calls {
		call.ID = prettyTestSpanID(byte(10 + i))
		call.TraceID = prettyTestTraceID()
		call.ParentID = rootID
		call.LLMRole = "assistant"
		call.Name = call.LLMTool
		call.StartTime = start.Add(time.Duration(2*i+1) * time.Second)
		call.EndTime = call.StartTime.Add(time.Second)
		call.Final = true
		snapshots = append(snapshots, call)
	}
	db.ImportSnapshots(snapshots)
	db.SetPrimarySpan(rootID)

	term := tuist.NewHeadlessTerminal(width, 80)
	fe := newWithTerminal(io.Discard, db, term)
	fe.profile = profile
	fe.logs.Profile = profile
	fe.shell = stubShellHandler{}
	fe.FrontendOpts.Verbosity = dagui.ShowCompletedVerbosity
	fe.autoFocus = false
	fe.recalculateViewLocked()

	frame := strings.Join(fe.tui.Frame(), "\n")
	var lines []string
	for _, line := range strings.Split(stripANSICodes(frame), "\n") {
		line = strings.TrimRight(line, " ")
		if len(lines) == 0 && !strings.Contains(line, "•") {
			continue
		}
		lines = append(lines, line)
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, frame
}

func toolCallSnapshot(tool string, args ...string) dagui.SpanSnapshot {
	snapshot := dagui.SpanSnapshot{LLMTool: tool}
	for i := 0; i+1 < len(args); i += 2 {
		snapshot.LLMToolArgNames = append(snapshot.LLMToolArgNames, args[i])
		snapshot.LLMToolArgValues = append(snapshot.LLMToolArgValues, args[i+1])
	}
	return snapshot
}

func TestDangToolCallTranscript(t *testing.T) {
	lines, _ := dangTranscript(t, 100, termenv.Ascii,
		toolCallSnapshot("DangEval", "source", "let greeting = shout(\"hi\")\ngreeting + \"!\""),
		toolCallSnapshot("DangEval", "at", "notes", "source", `shout("dang")`),
		toolCallSnapshot("DangCheck", "source", `["a"].reduce`),
		toolCallSnapshot("DangCheck", "at", "notes"),
		toolCallSnapshot("DangCheck", "search", "sort"),
		toolCallSnapshot("DangCheck", "at", ".dagger/modules/notes/main.dang:12:5", "search", "escape html", "source", "prefix + name"),
		// Not streamed or recorded yet: just the tool.
		toolCallSnapshot("DangEval"),
	)
	assert.Equal(t, []string{
		`  • DangEval 1.0s`,
		`  1 │ let greeting = shout("hi")`,
		`  2 │ greeting + "!"`,
		`  • DangEval notes 1.0s`,
		`  1 │ shout("dang")`,
		`  • DangCheck 1.0s`,
		`  1 │ ["a"].reduce`,
		`  • DangCheck notes 1.0s`,
		`  • DangCheck search "sort" 1.0s`,
		`  • DangCheck .dagger/modules/notes/main.dang:12:5 search "escape html" 1.0s`,
		`  1 │ prefix + name`,
		`  • DangEval 1.0s`,
	}, lines)
}

func TestTimeoutToolCallTranscript(t *testing.T) {
	lines, _ := dangTranscript(t, 100, termenv.Ascii,
		// Span attributes carry the wrapped call's arguments as JSON.
		toolCallSnapshot("Timeout", "arguments", `{"at":"notes","source":"container.from(\"alpine\").sync"}`, "duration", "2m", "tool", "DangEval"),
		// A wrapped tool with a summary of its own keeps it.
		toolCallSnapshot("Timeout", "arguments", `{"path":"main.go","offset":20}`, "duration", "30s", "tool", "read"),
		// An unknown wrapped tool falls back to the generic header.
		toolCallSnapshot("Timeout", "arguments", `{"name":"world"}`, "duration", "1h", "tool", "frobnicate"),
		// The wrapped tool isn't known (an older engine): the Timeout itself.
		toolCallSnapshot("Timeout", "duration", "5s"),
	)
	assert.Equal(t, []string{
		`  • DangEval notes timeout=2m 1.0s`,
		`  1 │ container.from("alpine").sync`,
		`  • Read main.go offset=20 timeout=30s 1.0s`,
		`  • Frobnicate(world) timeout=1h 1.0s`,
		`  • Timeout(5s) 1.0s`,
	}, lines)
}

func TestDangToolCallTranscriptNarrowAndColor(t *testing.T) {
	calls := []dagui.SpanSnapshot{
		toolCallSnapshot("Timeout", "arguments", `{"source":"let greeting = shout(\"hello\")\ngreeting"}`, "duration", "2m", "tool", "DangEval"),
	}
	lines, _ := dangTranscript(t, 24, termenv.Ascii, calls...)
	assert.Equal(t, []string{
		`  • DangEval timeout=2m`,
		`  1 │ let greeting = sho`,
		`    │ ut("hello")`,
		`  2 │ greeting`,
	}, lines)

	// In color, the snippet is highlighted, and lays out the same.
	colorLines, frame := dangTranscript(t, 24, termenv.ANSI, calls...)
	assert.Equal(t, lines, colorLines)
	assert.True(t, containsStyledLine(frame, "let greeting", "\x1b[35mlet"), visibleEscapes(frame))
}

func TestDangToolCallTranscriptCollapsesLongSnippets(t *testing.T) {
	var long []string
	for i := range dangSourceCollapsedLines + 4 {
		long = append(long, fmt.Sprintf("print(%d)", i))
	}
	lines, _ := dangTranscript(t, 100, termenv.Ascii,
		toolCallSnapshot("DangEval", "source", strings.Join(long, "\n")))
	require.Len(t, lines, 1+dangSourceCollapsedLines+1)
	assert.Equal(t, `  16 │ print(15)`, lines[dangSourceCollapsedLines])
	assert.Equal(t, `     ┆ … 4 more lines`, lines[dangSourceCollapsedLines+1])
}

// timeoutWithNestedCall builds a conversation of Timeout calls as the engine
// runs them: each opens the display span of the call it wraps beneath its
// own, under the tool's bare name and server, and carries the wrapped call's
// result as its own logs.
func timeoutWithNestedCall(t *testing.T, profile termenv.Profile, calls ...[2]dagui.SpanSnapshot) *frontendPretty {
	t.Helper()
	db := dagui.NewDB()
	rootID := prettyTestSpanID(1)
	start := time.Unix(100, 0)
	snapshots := []dagui.SpanSnapshot{{
		ID: rootID, TraceID: prettyTestTraceID(), Name: "shell",
		StartTime: start, EndTime: start.Add(time.Minute), Final: true,
	}}
	results := map[dagui.SpanID]string{}
	for i, call := range calls {
		timeout, nested := call[0], call[1]
		timeout.ID = prettyTestSpanID(byte(10 + 2*i))
		nested.ID = prettyTestSpanID(byte(11 + 2*i))
		timeout.ParentID, nested.ParentID = rootID, timeout.ID
		for _, s := range []*dagui.SpanSnapshot{&timeout, &nested} {
			s.TraceID = prettyTestTraceID()
			s.LLMRole = "assistant"
			s.Name = s.LLMTool
			s.Boundary, s.RollUpSpans, s.RollUpLogs = true, true, true
			s.StartTime = start.Add(time.Duration(2*i+1) * time.Second)
			s.EndTime = s.StartTime.Add(time.Second)
			s.Final = true
		}
		results[timeout.ID] = "RESULT OF " + nested.LLMTool
		snapshots = append(snapshots, timeout, nested)
	}
	db.ImportSnapshots(snapshots)
	db.SetPrimarySpan(rootID)

	term := tuist.NewHeadlessTerminal(100, 80)
	fe := newWithTerminal(io.Discard, db, term)
	fe.profile = profile
	fe.logs.Profile = profile
	fe.shell = stubShellHandler{}
	fe.FrontendOpts.Verbosity = dagui.ShowCompletedVerbosity
	fe.autoFocus = false
	for id, result := range results {
		fe.logs.Logs[id] = testVterm(t, result)
	}
	fe.recalculateViewLocked()
	return fe
}

// TestTimeoutPresentsAsItsNestedCall covers what a Timeout borrows from the
// call it runs, once that call's own span is there: its tool's server and
// bare name (the model names it with its server, "editor_read"), and whether
// its output stays collapsed -- a Timeout running Read is as quiet as Read.
// The final report shows the call once, not again for the nested span.
func TestTimeoutPresentsAsItsNestedCall(t *testing.T) {
	nestedRead := toolCallSnapshot("read", "path", "main.go")
	nestedRead.LLMToolServer = "Editor"
	fe := timeoutWithNestedCall(t, termenv.Ascii,
		[2]dagui.SpanSnapshot{
			toolCallSnapshot("Timeout", "arguments", `{"path":"main.go"}`, "duration", "30s", "tool", "editor_read"),
			nestedRead,
		},
		[2]dagui.SpanSnapshot{
			toolCallSnapshot("Timeout", "arguments", `{"source":"1 + 1"}`, "duration", "2m", "tool", "DangEval"),
			toolCallSnapshot("DangEval", "source", "1 + 1"),
		},
	)

	live := stripANSICodes(strings.Join(fe.tui.Frame(), "\n"))
	assert.Contains(t, live, "• editor Read main.go timeout=30s")
	assert.NotContains(t, live, "RESULT OF read", "a wrapped Read stays collapsed")
	assert.Contains(t, live, "• DangEval timeout=2m")
	assert.Contains(t, live, "RESULT OF DangEval")

	r := newRenderer(fe.db, 0, fe.FrontendOpts, true)
	report := strings.Join(fe.conversationReport(tuist.Context{Width: 100}, r, false), "\n")
	assert.Equal(t, 1, strings.Count(report, "Read main.go"), report)
	assert.NotContains(t, report, "RESULT OF read", report)
	assert.Equal(t, 1, strings.Count(report, "1 │ 1 + 1"), report)
	assert.Contains(t, strings.Split(report, "\n"), "  1 │ 1 + 1", "lined up under the call")
	assert.Equal(t, 1, strings.Count(report, "RESULT OF DangEval"), report)
}

// TestConversationReportRendersDangSource covers the final report: a Dang
// call shows its whole snippet as code, in place of its JSON arguments.
func TestConversationReportRendersDangSource(t *testing.T) {
	db := dagui.NewDB()
	toolID := prettyTestSpanID(2)
	start := time.Unix(100, 0)
	var long []string
	for i := range dangSourceCollapsedLines + 4 {
		long = append(long, fmt.Sprintf("print(%d)", i))
	}
	db.ImportSnapshots([]dagui.SpanSnapshot{{
		ID: toolID, TraceID: prettyTestTraceID(), Name: "DangEval",
		LLMRole: "assistant", LLMTool: "DangEval",
		LLMToolArgNames:  []string{"source"},
		LLMToolArgValues: []string{strings.Join(long, "\n")},
		StartTime:        start, EndTime: start.Add(time.Second), Final: true,
	}})
	fe := NewWithDB(io.Discard, db)
	fe.profile = termenv.Ascii
	fe.FrontendOpts.Verbosity = toolArgsVerbosity
	args := NewVterm(termenv.Ascii)
	_, _ = args.Write([]byte(`{"source":"print(0)"}` + "\n"))
	fe.logs.ToolArgs[toolID] = args

	var buf strings.Builder
	fe.renderMessageLogs(NewOutput(&buf, termenv.WithProfile(termenv.Ascii)), db.Spans.Map[toolID], 0)
	got := buf.String()
	assert.Contains(t, got, " 1 │ print(0)\n")
	assert.Contains(t, got, "20 │ print(19)\n")
	assert.NotContains(t, got, "more lines")
	assert.NotContains(t, got, `{"source"`)
}
