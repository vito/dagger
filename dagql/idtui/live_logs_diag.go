package idtui

import (
	"runtime"
	"strings"

	"github.com/dagger/dagger/dagql/dagui"
)

// DIAGNOSTICS: what a live session's reader needs to tell, from its trace
// alone, what widened its follow (a request with descendants) and what keeps
// it wide. Kept apart so it can be dropped on its own.

// LiveLogsDiagnostics is the optional interface of a LiveLogsFrontend that
// explains its requests.
type LiveLogsDiagnostics interface {
	// SetLogRequestDiagnostics registers fn, called on the event loop (it
	// must not block) for every request with descendants it makes, before
	// the request itself.
	SetLogRequestDiagnostics(fn func(LogRequest, LogRequestDiag))
	// LiveLogsRunning tells, for each span, the work that keeps it active
	// (LogActivity.Active): its running descendants. It returns nothing
	// once the frontend quit.
	LiveLogsRunning([]dagui.SpanID) []LogRunning
}

// LogRequestDiag is why a frontend requested a span's logs with its
// descendants.
type LogRequestDiag struct {
	Name, Tool string
	// Origin names the frontend functions that made the request, innermost
	// first.
	Origin string
	// Promoted and Reported tell whether the span is in the transcript the
	// live tree renders (promoted) and the one whose logs are requested own
	// only (reportConversation).
	Promoted, Reported bool
}

// LogRunning is the work that keeps a span active.
type LogRunning struct {
	// Running lists up to logRunningMax running descendants; Total counts
	// them all.
	Running []LogRunningSpan
	Total   int
	// Effects: the span's cause-linked effects run.
	Effects bool
}

// LogRunningSpan is a running descendant: Agent when it is an agent's loop,
// InAgent when it runs beneath one.
type LogRunningSpan struct {
	ID      dagui.SpanID
	Name    string
	Agent   bool
	InAgent bool
}

const logRunningMax = 10

var _ LiveLogsDiagnostics = (*frontendPretty)(nil)

func (fe *frontendPretty) SetLogRequestDiagnostics(fn func(LogRequest, LogRequestDiag)) {
	fe.dispatchWait(func() { fe.logRequestDiag = fn })
}

// diagnoseLogRequest reports a request with descendants to the registered
// diagnostics, if any. It runs within the request, on the event loop.
func (fe *frontendPretty) diagnoseLogRequest(req LogRequest) {
	if fe.logRequestDiag == nil || !req.Descendants {
		return
	}
	diag := LogRequestDiag{Origin: logRequestOrigin()}
	if span := fe.db.Spans.Map[req.Span]; span != nil {
		diag.Name, diag.Tool = span.Name, span.LLMTool
	}
	diag.Promoted = messageNodesHold(fe.promotedConversation, req.Span)
	diag.Reported = messageNodesHold(fe.reportConversation(), req.Span)
	fe.logRequestDiag(req, diag)
}

// logRequestOrigin names the frontend functions that asked for a span's
// logs: the callers of the request plumbing, innermost first.
func logRequestOrigin() string {
	pcs := make([]uintptr, 16)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	var names []string
	for {
		frame, more := frames.Next()
		name := frame.Function
		name = name[strings.LastIndex(name, "/")+1:]
		name = strings.TrimPrefix(name, "idtui.")
		switch {
		case !strings.Contains(frame.Function, "dagql/idtui"):
		case strings.Contains(name, "requestLogsWith"), strings.Contains(name, "requestLogsOnRender"),
			strings.HasSuffix(name, ".requestLogs"), strings.Contains(name, "SetLiveLogs"),
			strings.Contains(name, "diagnoseLogRequest"):
		default:
			names = append(names, name)
		}
		if !more || len(names) == 3 {
			break
		}
	}
	return strings.Join(names, " < ")
}

func messageNodesHold(nodes []*dagui.MessageNode, id dagui.SpanID) bool {
	for _, n := range nodes {
		if (n.Span != nil && n.Span.ID == id) || messageNodesHold(n.Children, id) {
			return true
		}
	}
	return false
}

func (fe *frontendPretty) LiveLogsRunning(spans []dagui.SpanID) []LogRunning {
	got := make(chan []LogRunning, 1)
	fe.dispatchWait(func() {
		running := make([]LogRunning, len(spans))
		for i, id := range spans {
			running[i] = fe.liveLogRunning(id)
		}
		got <- running
	})
	select {
	case running := <-got:
		return running
	default:
		return nil
	}
}

func (fe *frontendPretty) liveLogRunning(id dagui.SpanID) LogRunning {
	span, ok := fe.db.Spans.Map[id]
	if !ok {
		return LogRunning{}
	}
	r := LogRunning{Effects: span.Activity.IsRunning()}
	if span.RunningSpans == nil {
		return r
	}
	r.Total = span.RunningSpans.Len()
	for running := range span.RunningSpans.Iter() {
		if len(r.Running) == logRunningMax {
			break
		}
		rs := LogRunningSpan{ID: running.ID, Name: running.Name, Agent: running.Agent}
		for p := running.ParentSpan; p != nil && p != span; p = p.ParentSpan {
			if p.Agent {
				rs.InAgent = true
				break
			}
		}
		r.Running = append(r.Running, rs)
	}
	return r
}
