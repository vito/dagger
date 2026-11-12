package daggercmd

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	telemetry "github.com/dagger/otel-go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/dagger/dagger/dagql/dagui"
	"github.com/dagger/dagger/dagql/idtui"
	"github.com/dagger/dagger/engine/archive"
	"github.com/dagger/dagger/util/cleanups"
)

// DIAGNOSTICS tests (live_logs_diag.go).

// TestLiveLogsDiagnosticsReplay: in runUnplacedReply's scenario, the reply's
// first rows render twice, and the diagnostics name the request that widened
// the follow, the tree it gave, the reply's rows imported unclaimed, and the
// catch-up that imported them again.
func TestLiveLogsDiagnosticsReplay(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	reply, check := liveTestSpan(7).String(), liveTestSpan(5).String()
	exported, _ := runUnplacedReply(t, func(l *liveLogs, fe idtui.LiveLogsDiagnostics) {
		l.diag = newLiveLogsDiag(func(msg string, args ...any) {
			mu.Lock()
			defer mu.Unlock()
			lines = append(lines, strings.TrimSpace(fmt.Sprintln(append([]any{msg}, args...)...)))
		})
		l.running = fe.LiveLogsRunning
		fe.SetLogRequestDiagnostics(l.diagRequest)
	})
	require.Equal(t, 2, strings.Count(strings.Join(exported, ""), "Hello there"), "the scenario no longer replays: %q", exported)
	mu.Lock()
	defer mu.Unlock()
	has := func(parts ...string) bool {
		return slices.ContainsFunc(lines, func(line string) bool {
			for _, part := range parts {
				if !strings.Contains(line, part) {
					return false
				}
			}
			return true
		})
	}
	require.True(t, has("live logs: descendants request", "span "+check, "check lint", "ZoomToSpan"), "%q", lines)
	require.True(t, has("live logs: tree", "join", check), "%q", lines)
	require.True(t, has("live logs: unclaimed", "span "+reply), "%q", lines)
	require.True(t, has("live logs: replay", "span "+reply, "unclaimedRecords 1", "unclaimedRead 0", "claimedRecords 1"), "%q", lines)
}

// runUnplacedReply drives an agent conversation through the running headless
// frontend and an engine-like store. The user opens a check the agent's tool
// call runs, which rolls up its exec's output: it is requested with its
// descendants, so while it runs, the follow's query has a tree, and the store
// also returns rows of spans it has no snapshot of. The reply's first rows
// are stored, and read, before its span reaches the store and the frontend;
// once it arrives, the conversation requests it, own logs only. setup
// configures the reader before it starts. It returns what the reader
// exported and the final render.
func runUnplacedReply(t *testing.T, setup func(*liveLogs, idtui.LiveLogsDiagnostics)) (exported []string, rendered string) {
	t.Helper()
	t.Setenv("DAGGER_TUI_CONSOLE", "127.0.0.1:0")
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	live := dagui.TraceID{TraceID: trace.TraceID{15: 1}}
	const (
		rootN uint16 = iota + 1
		loopN
		promptN
		toolN
		checkN
		execN
		replyN
	)
	root, loop, prompt, tool, check, exec, reply := liveTestSpan(rootN), liveTestSpan(loopN), liveTestSpan(promptN),
		liveTestSpan(toolN), liveTestSpan(checkN), liveTestSpan(execN), liveTestSpan(replyN)
	fe := idtui.NewWithDB(io.Discard, dagui.NewDB())
	feCtx, stopFE := context.WithCancel(context.Background())
	feDone, ready := make(chan struct{}), make(chan struct{})
	var feErr error
	go func() {
		defer close(feDone)
		feErr = fe.Run(feCtx, dagui.FrontendOpts{}, func(ctx context.Context) (cleanups.CleanupF, error) {
			close(ready)
			<-ctx.Done()
			return nil, nil
		})
	}()
	<-ready
	lCtx, stopLogs := context.WithCancel(ctx)
	logsDone := make(chan struct{})
	t.Cleanup(func() {
		stopLogs()
		<-logsDone
		stopFE()
		<-feDone
	})

	store := &fakeLogStore{placing: true}
	store.snapshot(rootN, 0)
	store.snapshot(loopN, rootN)
	store.snapshot(promptN, loopN)
	store.snapshot(toolN, loopN)
	store.snapshot(checkN, toolN)
	store.snapshot(execN, checkN)
	store.add(
		liveLogRow{span: promptN, body: "Lint the repo\n"},
		liveLogRow{span: execN, body: "linting...\n"},
	)
	out := &bodyCollector{}
	l := newLiveLogs(store, live.String(), nil, true)
	unknown := make(chan struct{})
	var unknownOnce sync.Once
	l.coverage = func(targets []idtui.LogTarget) []idtui.LogRoute {
		routes := fe.LiveLogsCoverage(targets)
		for i, target := range targets {
			if target.Span == reply && i < len(routes) && routes[i].Coverage == idtui.LogUnknown {
				unknownOnce.Do(func() { close(unknown) })
			}
		}
		return routes
	}
	l.activity = fe.LiveLogsActivity
	if setup != nil {
		setup(l, fe)
	}
	export := fe.SetLiveLogs(live, l.subscribe)
	l.export = exporterFunc(func(ctx context.Context, records []sdklog.Record) error {
		if err := out.Export(ctx, records); err != nil {
			return err
		}
		return export.Export(ctx, records)
	})
	go func() { defer close(logsDone); l.run(lCtx) }()

	start := time.Now()
	fe.ImportSnapshots([]dagui.SpanSnapshot{
		{ID: root, TraceID: live, Name: "agent", StartTime: start},
		{ID: loop, TraceID: live, ParentID: root, Name: "loop", StartTime: start, Agent: true, AgentID: "agent-1", AgentName: "agent"},
		{ID: prompt, TraceID: live, ParentID: loop, Name: "prompt", LLMRole: "user", Message: "sent", StartTime: start.Add(time.Millisecond), EndTime: start.Add(2 * time.Millisecond), HasLogs: true},
		{ID: tool, TraceID: live, ParentID: loop, Name: "Check", LLMRole: "assistant", LLMTool: "Check", Boundary: true, RollUpLogs: true, RollUpSpans: true, StartTime: start.Add(3 * time.Millisecond), HasLogs: true},
		{ID: check, TraceID: live, ParentID: tool, Name: "check lint", CheckName: "lint", RollUpLogs: true, StartTime: start.Add(4 * time.Millisecond)},
		{ID: exec, TraceID: live, ParentID: check, Name: "exec lint", StartTime: start.Add(5 * time.Millisecond), HasLogs: true},
	})
	fe.SetPrimary(root)
	fe.RequestSurfacedLogs()
	// The user opens the running check, and goes back to the conversation.
	fe.ZoomToSpan(check)
	fe.ZoomToSpan(root)
	fe.RequestSurfacedLogs()
	require.Eventually(t, func() bool {
		seen := store.seen()
		if len(seen) == 0 || !seen[len(seen)-1].Follow {
			return false
		}
		return slices.ContainsFunc(seen[len(seen)-1].Spans, func(s archive.LogSpan) bool {
			return s.SpanID == check.String() && s.Descendants
		})
	}, 5*time.Second, 10*time.Millisecond, "the opened check is not followed with its descendants")

	// The reply streams its first rows before its span reaches the store,
	// or the frontend: the follow reads them as rows of an unplaced span.
	store.add(liveLogRow{span: replyN, body: "Hello there, "})
	select {
	case <-unknown:
	case <-ctx.Done():
		t.Fatal("the follow never read the unplaced reply's rows")
	}
	// Its span arrives, and the conversation requests its own logs.
	store.snapshot(replyN, loopN)
	fe.ImportSnapshots([]dagui.SpanSnapshot{
		{ID: reply, TraceID: live, ParentID: loop, Name: "reply", LLMRole: "assistant", Message: "received", StartTime: start.Add(10 * time.Millisecond), HasLogs: true},
	})
	fe.RequestSurfacedLogs()
	store.add(liveLogRow{span: replyN, body: "general Kenobi\n"})
	require.Eventually(t, func() bool {
		return strings.Contains(strings.Join(out.got(), ""), "general Kenobi")
	}, 10*time.Second, 10*time.Millisecond, "the reply's later rows never arrived")
	require.NoError(t, fe.WaitForEventLoop(ctx))
	stopLogs()
	<-logsDone
	stopFE()
	<-feDone
	require.NoError(t, feErr)

	var final strings.Builder
	require.NoError(t, fe.FinalRender(&final))
	return out.got(), final.String()
}

// TestLiveLogsDiagnosticsRecords: the diagnostics are Info records of their
// own internal span, emitted through the command's logger provider -- the
// one whose processors carry the CLI's telemetry to the engine's store and
// Cloud -- not records of the primary span, whose output is printed at exit.
func TestLiveLogsDiagnosticsRecords(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	var records recordCollector
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(&records)))
	ctx, root := tp.Tracer("test").Start(telemetry.WithLoggerProvider(t.Context(), lp), "command")
	defer root.End()

	d := startLiveLogsDiag(ctx)
	d.emit("started", "follow", true)
	d.finish()
	d.emit("after finish")

	require.Len(t, records.records, 1)
	rec := records.records[0]
	require.Contains(t, rec.Body().AsString(), "live logs: started")
	require.Equal(t, d.span, rec.SpanID(), "not a record of the diagnostics span")
	require.NotEqual(t, root.SpanContext().SpanID(), rec.SpanID())
	rec.WalkAttributes(func(kv log.KeyValue) bool {
		require.NotEqual(t, telemetry.StdioStreamAttr, kv.Key, "diagnostics are not stdio")
		return true
	})
	ended := spans.Ended()
	i := slices.IndexFunc(ended, func(s sdktrace.ReadOnlySpan) bool { return s.SpanContext().SpanID() == d.span })
	require.GreaterOrEqual(t, i, 0)
	require.Equal(t, "live logs diagnostics", ended[i].Name())
	require.Equal(t, root.SpanContext().SpanID(), ended[i].Parent().SpanID())
	require.True(t, slices.ContainsFunc(ended[i].Attributes(), func(kv attribute.KeyValue) bool {
		return kv.Key == telemetry.UIInternalAttr && kv.Value.AsBool()
	}), "the diagnostics span is not internal: the TUI would render it")
}

// TestLiveLogsDiagnosticsRateLimit: a burst of diagnostics is capped per
// minute, and the next minute's first line says how many were dropped.
func TestLiveLogsDiagnosticsRateLimit(t *testing.T) {
	var lines [][]any
	d := newLiveLogsDiag(func(msg string, args ...any) { lines = append(lines, append([]any{msg}, args...)) })
	clock := &fakeClock{t: time.Unix(100, 0)}
	d.now = clock.now
	for range 3 * liveLogsDiagRate {
		d.emit("tree")
	}
	require.Len(t, lines, liveLogsDiagRate)
	clock.advance(time.Minute)
	d.emit("tree")
	require.Equal(t, []any{"live logs: tree", "dropped", 2 * liveLogsDiagRate}, lines[len(lines)-1])
}

// TestLiveLogsDiagnosticsLongRollUp: a span followed with its descendants for
// over liveLogsDiagLongRollUp is reported once, with the work keeping it
// active.
func TestLiveLogsDiagnosticsLongRollUp(t *testing.T) {
	var lines []string
	l := newLiveLogs(nil, "trace", nil, true)
	clock := &fakeClock{t: time.Unix(100, 0)}
	l.now = clock.now
	l.diag = newLiveLogsDiag(func(msg string, args ...any) {
		lines = append(lines, strings.TrimSpace(fmt.Sprintln(append([]any{msg}, args...)...)))
	})
	l.diag.now = clock.now
	spawn, worker := liveTestSpan(5), liveTestSpan(8)
	var asked [][]dagui.SpanID
	l.running = func(ids []dagui.SpanID) []idtui.LogRunning {
		asked = append(asked, ids)
		return []idtui.LogRunning{{Total: 1, Running: []idtui.LogRunningSpan{{ID: worker, Name: "agent: treesitter", Agent: true}}}}
	}
	l.mu.Lock()
	l.spans[spawn.String()] = archive.LogSpan{SpanID: spawn.String(), Descendants: true}
	l.spans[liveTestSpan(3).String()] = archive.LogSpan{SpanID: liveTestSpan(3).String()}
	l.noteTreeLocked("join")
	l.mu.Unlock()
	require.Equal(t, []string{"live logs: tree why join spans " + spawn.String() + " count 1"}, lines)
	l.reportLongRollUps()
	require.Empty(t, asked, "asked about a span followed briefly")
	clock.advance(liveLogsDiagLongRollUp + time.Second)
	l.reportLongRollUps()
	l.reportLongRollUps()
	require.Equal(t, [][]dagui.SpanID{{spawn}}, asked, "reported more than once")
	require.Len(t, lines, 2)
	require.Contains(t, lines[1], "live logs: long-lived roll-up span "+spawn.String())
	require.Contains(t, lines[1], "agent true")
	require.Contains(t, lines[1], worker.String()+` "agent: treesitter" [agent]`)
}
