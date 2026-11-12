package daggercmd

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	telemetry "github.com/dagger/otel-go"
	"go.opentelemetry.io/otel/trace"

	"github.com/dagger/dagger/dagql/dagui"
	"github.com/dagger/dagger/dagql/idtui"
	"github.com/dagger/dagger/engine/slog"
)

// DIAGNOSTICS for on-demand live logs: what widened the follow (a request
// with descendants), how its tree changed, which records were imported
// unclaimed, which of those a later read imported again (a replay: the text
// shows twice), and what keeps a roll-up followed for long. Kept apart so it
// can be dropped on its own. Every line starts with liveLogsDiagPrefix.
//
// The lines are Info records of their own internal span, "live logs
// diagnostics", so the session's trace keeps them (the CLI's telemetry goes
// to the engine's store, and Cloud): FindSpans finds the span, ReadLogs reads
// them. No frontend request selects that span -- it is internal, never
// rendered, and no request reads its parent with descendants -- so they never
// feed back into the follow, and the frontend drops the live trace's own
// display text, so they never reach the TUI either. Nor are they the
// primary span's output, printed at exit.
const liveLogsDiagPrefix = "live logs: "

const (
	// liveLogsDiagRate bounds the lines per minute; the rest are counted
	// and the count reported with the next line.
	liveLogsDiagRate = 20
	// liveLogsDiagUnclaimed bounds the pairs remembered as imported
	// unclaimed, oldest first out.
	liveLogsDiagUnclaimed = 4096
	// liveLogsDiagLongRollUp is how long a span followed with its
	// descendants may stay so before what keeps it active is reported.
	liveLogsDiagLongRollUp = 10 * time.Minute
)

// liveLogsDiag emits diagnostics, rate-limited, without ever blocking.
type liveLogsDiag struct {
	// log emits one line; it must not block.
	log func(msg string, args ...any)
	now func() time.Time
	// span is the diagnostics span, whose own records are not diagnosed.
	span trace.SpanID

	mu             sync.Mutex
	window         time.Time
	lines, dropped int
	closed         bool
	close          func()

	// unclaimed remembers the pairs imported unclaimed, FIFO-bounded.
	unclaimed      map[logOwner]liveLogsUnclaimed
	unclaimedOrder []logOwner
}

type liveLogsUnclaimed struct {
	records int
	read    uint64
}

func newLiveLogsDiag(log func(string, ...any)) *liveLogsDiag {
	return &liveLogsDiag{log: log, now: time.Now, unclaimed: map[logOwner]liveLogsUnclaimed{}}
}

// startLiveLogsDiag logs diagnostics as Info records of their own span: see
// liveLogsDiagPrefix. The lines are handed to a goroutine, so emitting never
// waits on telemetry; past its buffer they are dropped.
func startLiveLogsDiag(ctx context.Context) *liveLogsDiag {
	ctx, span := Tracer().Start(ctx, "live logs diagnostics", telemetry.Internal(), telemetry.Encapsulate())
	span.End()
	logger := slog.SpanLogger(ctx, InstrumentationLibrary)
	type line struct {
		msg  string
		args []any
	}
	lines := make(chan line, 256)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for l := range lines {
			logger.Info(l.msg, l.args...)
		}
	}()
	d := newLiveLogsDiag(func(msg string, args ...any) {
		select {
		case lines <- line{msg, args}:
		default:
		}
	})
	d.span = span.SpanContext().SpanID()
	d.close = func() {
		close(lines)
		<-done
	}
	return d
}

// emit logs a line, unless the minute's budget is spent.
func (d *liveLogsDiag) emit(msg string, args ...any) {
	if d == nil || d.log == nil {
		return
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	if now := d.now(); now.Sub(d.window) >= time.Minute {
		if d.dropped > 0 {
			args = append(args, "dropped", d.dropped)
		}
		d.window, d.lines, d.dropped = now, 0, 0
	}
	if d.lines >= liveLogsDiagRate {
		d.dropped++
		d.mu.Unlock()
		return
	}
	d.lines++
	// Under mu, so a line is never sent once closed.
	d.log(liveLogsDiagPrefix+msg, args...)
	d.mu.Unlock()
}

// finish flushes the lines emitted so far, and emits no more.
func (d *liveLogsDiag) finish() {
	if d == nil {
		return
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.closed = true
	d.mu.Unlock()
	if d.close != nil {
		d.close()
	}
}

// own reports whether a record's span is the diagnostics span.
func (d *liveLogsDiag) own(span [8]byte) bool {
	return d != nil && d.span.IsValid() && trace.SpanID(span) == d.span
}

// imported notes what a read of number read imported of each pair: unclaimed
// (idtui.LogUnknown), then claimed. A pair imported unclaimed, then claimed
// by a read that reads from the start -- a catch-up, a gap read, finish's
// read of a pending span -- is a replay: the records show twice.
func (d *liveLogsDiag) imported(read uint64, unclaimed, claimed map[logOwner]int) {
	if d == nil || d.log == nil {
		return
	}
	type replay struct {
		key        logOwner
		was        liveLogsUnclaimed
		claimedNow int
	}
	var replays []replay
	var fresh []logOwner
	d.mu.Lock()
	for key, n := range unclaimed {
		u, seen := d.unclaimed[key]
		if !seen {
			if len(d.unclaimedOrder) >= liveLogsDiagUnclaimed {
				delete(d.unclaimed, d.unclaimedOrder[0])
				d.unclaimedOrder = d.unclaimedOrder[1:]
			}
			d.unclaimedOrder = append(d.unclaimedOrder, key)
			u.read = read
			fresh = append(fresh, key)
		}
		u.records += n
		d.unclaimed[key] = u
	}
	for key, n := range claimed {
		u, seen := d.unclaimed[key]
		if !seen {
			continue
		}
		delete(d.unclaimed, key)
		d.unclaimedOrder = slices.DeleteFunc(d.unclaimedOrder, func(k logOwner) bool { return k == key })
		if read != 0 {
			replays = append(replays, replay{key, u, n})
		}
	}
	d.mu.Unlock()
	// Once per pair: the replay line counts every record imported unclaimed.
	for _, key := range fresh {
		d.emit("unclaimed", "span", trace.SpanID(key.span).String(), "digest", key.digest,
			"records", unclaimed[key], "read", read)
	}
	for _, r := range replays {
		d.emit("replay", "span", trace.SpanID(r.key.span).String(), "digest", r.key.digest,
			"unclaimedRecords", r.was.records, "unclaimedRead", r.was.read,
			"claimedRecords", r.claimedNow, "claimedRead", read)
	}
}

// diagRequest reports a frontend request with descendants
// (idtui.LiveLogsDiagnostics). It runs on the frontend's event loop.
func (l *liveLogs) diagRequest(req idtui.LogRequest, why idtui.LogRequestDiag) {
	l.mu.Lock()
	_, followed := l.spans[req.Span.String()]
	l.mu.Unlock()
	l.diag.emit("descendants request", "span", req.Span.String(), "name", why.Name, "tool", why.Tool,
		"origin", why.Origin, "promoted", why.Promoted, "reported", why.Reported, "followed", followed)
}

// noteTreeLocked reports the followed spans read with their descendants
// when they change, and remembers since when each is. It runs with mu held.
func (l *liveLogs) noteTreeLocked(why string) {
	if l.diag == nil {
		return
	}
	var tree []string
	for id, span := range l.spans {
		if span.Descendants {
			tree = append(tree, id)
		}
	}
	slices.Sort(tree)
	key := strings.Join(tree, ",")
	if key == l.treeKey {
		return
	}
	l.treeKey = key
	if l.treeSince == nil {
		l.treeSince = map[string]time.Time{}
	}
	now := l.now()
	for id := range l.treeSince {
		if !slices.Contains(tree, id) {
			delete(l.treeSince, id)
			delete(l.treeReported, id)
		}
	}
	for _, id := range tree {
		if _, ok := l.treeSince[id]; !ok {
			l.treeSince[id] = now
		}
	}
	l.diag.emit("tree", "why", why, "spans", key, "count", len(tree))
}

// reportLongRollUps reports, once each, the spans followed with their
// descendants for longer than liveLogsDiagLongRollUp, and the work that keeps
// them active. It must not run with mu held: the frontend answers on its
// event loop.
func (l *liveLogs) reportLongRollUps() {
	if l.diag == nil || l.running == nil {
		return
	}
	now := l.now()
	l.mu.Lock()
	var ids []dagui.SpanID
	var since []time.Time
	for id, t := range l.treeSince {
		if l.treeReported[id] || now.Sub(t) < liveLogsDiagLongRollUp {
			continue
		}
		if l.treeReported == nil {
			l.treeReported = map[string]bool{}
		}
		l.treeReported[id] = true
		var sid dagui.SpanID
		sid.SpanID, _ = trace.SpanIDFromHex(id)
		ids = append(ids, sid)
		since = append(since, t)
	}
	l.mu.Unlock()
	if len(ids) == 0 {
		return
	}
	running := l.running(ids)
	for i, id := range ids[:min(len(ids), len(running))] {
		r := running[i]
		var spans []string
		agent := false
		for _, s := range r.Running {
			kind := ""
			switch {
			case s.Agent:
				kind, agent = " [agent]", true
			case s.InAgent:
				kind, agent = " [in agent]", true
			}
			spans = append(spans, fmt.Sprintf("%s %q%s", s.ID, s.Name, kind))
		}
		l.diag.emit("long-lived roll-up", "span", id.String(), "followedFor", now.Sub(since[i]).Round(time.Second).String(),
			"running", r.Total, "agent", agent, "effects", r.Effects, "runningSpans", strings.Join(spans, "; "))
	}
}
