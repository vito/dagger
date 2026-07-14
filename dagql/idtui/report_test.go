package idtui

import (
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/dagger/dagger/dagql/dagui"
)

// reportTestTrace hydrates a fresh TraceReport with a small failed run: a
// root command with a completed setup span and a failed leaf. Returns the
// report and the span IDs for zoom/verbosity assertions.
func reportTestTrace(t *testing.T) (*TraceReport, dagui.SpanID, dagui.SpanID) {
	t.Helper()
	rootID := prettyTestSpanID(1)
	connectID := prettyTestSpanID(2)
	failedID := prettyTestSpanID(3)
	start := time.Unix(100, 0)
	tr := NewTraceReport()
	tr.fe.db.ImportSnapshots([]dagui.SpanSnapshot{
		{
			ID:        rootID,
			TraceID:   prettyTestTraceID(),
			Name:      "dagger check",
			StartTime: start,
			EndTime:   start.Add(10 * time.Second),
			Status:    sdktrace.Status{Code: codes.Error},
			Final:     true,
		},
		{
			ID:        connectID,
			TraceID:   prettyTestTraceID(),
			ParentID:  rootID,
			Name:      "connect",
			StartTime: start,
			EndTime:   start.Add(2 * time.Second),
			Final:     true,
		},
		{
			ID:        failedID,
			TraceID:   prettyTestTraceID(),
			ParentID:  rootID,
			Name:      "test:unit",
			StartTime: start.Add(2 * time.Second),
			EndTime:   start.Add(8 * time.Second),
			Status:    sdktrace.Status{Code: codes.Error},
			Final:     true,
		},
	})
	return tr, connectID, failedID
}

// TestTraceReportPlainAgentOutput verifies the offline report forces the
// agent-readable style regardless of the process environment: no ANSI escape
// codes (the engine's env says nothing about the reader) and the flat
// "== TRACE ==" heading style, with the verdict and both child spans present.
func TestTraceReportPlainAgentOutput(t *testing.T) {
	tr, _, _ := reportTestTrace(t)
	out := tr.Render(dagui.SpanID{}, dagui.ShowCompletedVerbosity)

	if strings.Contains(out, "\x1b[") {
		t.Errorf("report contains ANSI escapes:\n%s", out)
	}
	if !strings.Contains(out, "== TRACE ==") {
		t.Errorf("report missing agent-style == TRACE == heading:\n%s", out)
	}
	if !strings.Contains(out, "FAILED") {
		t.Errorf("report missing FAILED verdict:\n%s", out)
	}
	for _, want := range []string{"connect", "test:unit"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing span %q:\n%s", want, out)
		}
	}
}

// TestTraceReportZoom verifies zooming scopes the report to the target span's
// subtree, like 'dagger trace --span': the zoomed span renders, its sibling
// does not, and the whole-trace verdict header is replaced by the zoom title.
func TestTraceReportZoom(t *testing.T) {
	tr, _, failedID := reportTestTrace(t)
	out := tr.Render(failedID, dagui.ShowCompletedVerbosity)

	if !strings.Contains(out, "test:unit") {
		t.Errorf("zoomed report missing target span:\n%s", out)
	}
	if strings.Contains(out, "connect") {
		t.Errorf("zoomed report leaked sibling span:\n%s", out)
	}
	if strings.Contains(out, "== TRACE ==") {
		t.Errorf("zoomed report should not render the whole-trace header:\n%s", out)
	}
}

// TestTraceReportVerbosity verifies the verbosity knob: at
// HideCompletedVerbosity only the failure (and its ancestry) shows; at
// ShowCompletedVerbosity the completed setup span appears too.
func TestTraceReportVerbosity(t *testing.T) {
	quiet, _, _ := reportTestTrace(t)
	q := quiet.Render(dagui.SpanID{}, dagui.HideCompletedVerbosity)
	if strings.Contains(q, "connect") {
		t.Errorf("verbosity %d should hide the completed span:\n%s", dagui.HideCompletedVerbosity, q)
	}
	if !strings.Contains(q, "test:unit") {
		t.Errorf("verbosity %d should still show the failed span:\n%s", dagui.HideCompletedVerbosity, q)
	}

	loud, _, _ := reportTestTrace(t)
	l := loud.Render(dagui.SpanID{}, dagui.ShowCompletedVerbosity)
	if !strings.Contains(l, "connect") {
		t.Errorf("verbosity %d should show the completed span:\n%s", dagui.ShowCompletedVerbosity, l)
	}
}
