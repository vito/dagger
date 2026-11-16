package idtui

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vito/tuist"

	"github.com/dagger/dagger/dagql/dagui"
)

// TestSearchFindsCollapsedMatches checks that search finds matches beneath
// collapsed rows, whose trees the view doesn't build, in tree order, and
// navigates to one whose home is beneath a cause rather than its parent.
func TestSearchFindsCollapsedMatches(t *testing.T) {
	start := time.Unix(100, 0)
	at := func(s int) time.Time { return start.Add(time.Duration(s) * time.Second) }
	id := prettyTestSpanID
	span := func(n byte, parent byte, name string) dagui.SpanSnapshot {
		snap := dagui.SpanSnapshot{
			ID: id(n), TraceID: prettyTestTraceID(), Name: name,
			StartTime: at(int(n)), EndTime: at(int(n) + 1),
		}
		if parent != 0 {
			snap.ParentID = id(parent)
		}
		return snap
	}
	hosted := span(6, 2, "needle three")
	hosted.Links = []dagui.SpanLink{{
		SpanContext: dagui.SpanContext{SpanID: id(5)},
		Purpose:     "cause",
	}}
	db := dagui.NewDB()
	db.ImportSnapshots([]dagui.SpanSnapshot{
		span(1, 0, "root"),
		span(2, 1, "a"),
		span(3, 2, "needle one"),
		span(4, 1, "needle two"),
		span(5, 1, "c"),
		hosted,
	})
	db.SetPrimarySpan(id(1))
	fe := newWithTerminal(io.Discard, db, tuist.NewHeadlessTerminal(120, 40))
	fe.FrontendOpts.ZoomedSpan = id(1)
	fe.FrontendOpts.Verbosity = dagui.ShowCompletedVerbosity
	fe.setupTUI()
	fe.recalculateViewLocked()
	require.Nil(t, fe.rows.BySpan[id(3)], "a should be collapsed")
	require.Nil(t, fe.rows.BySpan[id(6)], "c should be collapsed")

	fe.confirmSearch("needle")
	var got []dagui.SpanID
	for _, m := range fe.searchMatches {
		got = append(got, m.spanID)
	}
	require.Equal(t, []dagui.SpanID{id(3), id(4), id(6)}, got)

	// The third match's home is beneath its cause, c.
	fe.searchIdx = 2
	fe.goToSearchMatch(2)
	require.Equal(t, id(6), fe.FocusedSpan)
	row := fe.rows.BySpan[id(6)]
	require.NotNil(t, row)
	require.Equal(t, id(5), row.Parent.Span.ID)
}

// TestSearchMatchSurvivesReflow checks that going to a match in a log laid
// out before its row showed it -- search lays it out at the full width, and
// its row's prefix then narrows it, which lays it out again -- keeps that
// match current and in view, though its row moved.
func TestSearchMatchSurvivesReflow(t *testing.T) {
	start := time.Unix(100, 0)
	id := prettyTestSpanID
	db := dagui.NewDB()
	db.ImportSnapshots([]dagui.SpanSnapshot{
		{ID: id(1), TraceID: prettyTestTraceID(), Name: "root", StartTime: start, EndTime: start.Add(time.Minute)},
		{ID: id(2), TraceID: prettyTestTraceID(), Name: "build", ParentID: id(1), StartTime: start, EndTime: start.Add(time.Minute)},
	})
	db.SetPrimarySpan(id(1))
	fe := newWithTerminal(io.Discard, db, tuist.NewHeadlessTerminal(120, 40))
	fe.FrontendOpts.ZoomedSpan = id(1)
	fe.FrontendOpts.Verbosity = dagui.ShowCompletedVerbosity
	fe.setupTUI()
	fe.tui.Step()
	logs := fe.logs.spanLogs(id(2))
	// Each line fits the screen, but not the row's gutter beside it.
	for i := range 20 {
		line := fmt.Sprintf("needle %02d ", i)
		_, _ = logs.Write([]byte(line + strings.Repeat("x", 119-len(line)) + "\n"))
	}
	fe.recalculateViewLocked()

	fe.confirmSearch("needle")
	require.Len(t, fe.searchMatches, 20)
	fe.searchIdx = 5
	fe.goToSearchMatch(5)
	fe.syncSearchState()
	step := func() string {
		return stripANSICodes(strings.Join(fe.tui.Step(), "\n"))
	}
	frame := step()
	require.Contains(t, frame, "needle 05", frame)

	// The next match is the next one, though the matches were gathered
	// before the render moved their rows.
	fe.searchNext()
	for range 3 {
		frame = step()
	}
	require.Equal(t, 6, fe.searchIdx)
	require.Equal(t, fe.searchMatches[6].logRow, logs.SearchCurrentRow)
	require.Contains(t, frame, "needle 06", frame)
}
