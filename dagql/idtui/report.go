package idtui

import (
	"io"
	"strings"
	"time"

	"github.com/muesli/termenv"
	"github.com/vito/tuist"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/dagger/dagger/dagql/dagui"
)

// TraceReport renders a trace's final pretty report offline — no TTY, no TUI
// event loop — for a consumer that reads the result as text (e.g. the LLM's
// ReadTrace tool). It wraps a reportOnly frontendPretty the same way the
// end-of-run report and 'dagger trace' do, so the reader sees what a user
// sees, but with plain output and agent-style headings forced on regardless
// of the process environment (the engine's env says nothing about who's
// reading).
//
// Hydrate it by feeding OTel data to SpanExporter/LogExporter (e.g. replayed
// from the session's clientdb), then call Render.
type TraceReport struct {
	fe *frontendPretty
}

func NewTraceReport() *TraceReport {
	db := dagui.NewDB()
	// The terminal is never started; final render ignores its dimensions
	// (contentWidth 0 = no truncation), it just has to exist.
	fe := newWithTerminal(io.Discard, db, tuist.NewHeadlessTerminal(consoleWidth, consoleHeight))
	fe.reportOnly = true
	// Force plain, agent-readable output. newWithTerminal sniffed the process
	// env (ColorProfile/RunningInAgent), which describes the engine, not the
	// reader. Swap the log store too: it mints per-span vterms with the
	// profile baked in, and nothing has been ingested yet.
	fe.profile = termenv.Ascii
	fe.logs = newPrettyLogs(termenv.Ascii, db)
	fe.agentStyle = true
	return &TraceReport{fe: fe}
}

// SpanExporter returns the sink for the trace's spans. The trace's root
// (parentless) span becomes the report's primary span as it's ingested.
func (tr *TraceReport) SpanExporter() sdktrace.SpanExporter {
	return tr.fe.SpanExporter()
}

// LogExporter returns the sink for the trace's logs. Feed spans first: log
// records route to their span's vterm, and unknown spans buffer as orphans.
func (tr *TraceReport) LogExporter() sdklog.Exporter {
	return tr.fe.LogExporter()
}

// Render produces the final report, zoomed to the given span when valid (the
// same subtree scoping as 'dagger trace --span'), at the given dagui
// verbosity (dagui.ShowCompletedVerbosity shows completed spans, collapsed;
// see dagql/dagui/opts.go for the levels).
func (tr *TraceReport) Render(zoom dagui.SpanID, verbosity int) string {
	fe := tr.fe
	fe.FrontendOpts.Verbosity = verbosity
	if fe.FrontendOpts.TooFastThreshold == 0 {
		fe.FrontendOpts.TooFastThreshold = 100 * time.Millisecond
	}
	if fe.FrontendOpts.GCThreshold == 0 {
		fe.FrontendOpts.GCThreshold = time.Second
	}
	if zoom.IsValid() {
		fe.ZoomToSpan(zoom)
	} else {
		// Whole trace; clear any zoom pinned by a prior Render.
		fe.pinnedZoom = dagui.SpanID{}
	}
	// Mirror the reportOnly leg of Run/FinalRender: hydration is done and
	// nothing else is dispatching, so drive the one-shot render directly.
	fe.setupFinalRenderLocked()
	return strings.Join(fe.tui.RenderLines(), "\n")
}
