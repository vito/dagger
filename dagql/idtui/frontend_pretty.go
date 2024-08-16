package idtui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/pkg/browser"
	"go.opentelemetry.io/otel/codes"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/term"

	"github.com/dagger/dagger/engine/slog"
)

type frontendPretty struct {
	FrontendOpts

	// updated by Run
	program     *tea.Program
	run         func(context.Context) error
	runCtx      context.Context
	interrupt   func()
	interrupted bool
	quitting    bool
	done        bool
	err         error

	// updated as events are written
	db           *DB
	logs         *prettyLogs
	eof          bool
	backgrounded bool
	autoFocus    bool
	debugged     trace.SpanID
	focusedIdx   int
	rowsView     *RowsView
	rows         *Rows
	pressedKey   string
	pressedKeyAt time.Time

	// set when authenticated to Cloud
	cloudURL string

	// TUI state/config
	restore    func()  // restore terminal
	fps        float64 // frames per second
	profile    termenv.Profile
	window     tea.WindowSizeMsg // set by BubbleTea
	view       *strings.Builder  // rendered async
	viewOut    *termenv.Output
	browserBuf *strings.Builder // logs if browser fails

	// held to synchronize tea.Model with updates
	mu sync.Mutex

	// messages to print before the final render
	msgPreFinalRender strings.Builder
}

func New() Frontend {
	db := NewDB()
	profile := ColorProfile()
	view := new(strings.Builder)
	return &frontendPretty{
		db:        db,
		logs:      newPrettyLogs(),
		autoFocus: true,

		// set empty initial row state to avoid nil checks
		rowsView: &RowsView{},
		rows:     &Rows{BySpan: map[trace.SpanID]*TraceRow{}},

		// initial TUI state
		window:     tea.WindowSizeMsg{Width: -1, Height: -1}, // be clear that it's not set
		fps:        30,                                       // sane default, fine-tune if needed
		profile:    profile,
		view:       view,
		viewOut:    NewOutput(view, termenv.WithProfile(profile)),
		browserBuf: new(strings.Builder),
	}
}

func (fe *frontendPretty) ConnectedToEngine(ctx context.Context, name string, version string, clientID string) {
	// noisy, so suppress this for now
}

func (fe *frontendPretty) SetCloudURL(ctx context.Context, url string, msg string, logged bool) {
	if fe.OpenWeb {
		if err := browser.OpenURL(url); err != nil {
			slog.Warn("failed to open URL", "url", url, "err", err)
		}
	}
	fe.mu.Lock()
	fe.cloudURL = url
	if msg != "" {
		slog.Warn(msg)
	}

	if logged {
		fe.msgPreFinalRender.WriteString(traceMessage(fe.profile, url, msg))
	} else if !skipLoggedOutTraceMsg() {
		fe.msgPreFinalRender.WriteString(fmt.Sprintf(loggedOutTraceMsg, url))
	}
	fe.mu.Unlock()
}

func traceMessage(profile termenv.Profile, url string, msg string) string {
	buffer := &bytes.Buffer{}
	out := NewOutput(buffer, termenv.WithProfile(profile))

	fmt.Fprint(buffer, out.String("Full trace at ").Bold().String())
	fmt.Fprint(buffer, url)
	if msg != "" {
		fmt.Fprintf(buffer, " (%s)", msg)
	}

	return buffer.String()
}

// Run starts the TUI, calls the run function, stops the TUI, and finally
// prints the primary output to the appropriate stdout/stderr streams.
func (fe *frontendPretty) Run(ctx context.Context, opts FrontendOpts, run func(context.Context) error) error {
	if opts.TooFastThreshold == 0 {
		opts.TooFastThreshold = 100 * time.Millisecond
	}
	if opts.GCThreshold == 0 {
		opts.GCThreshold = 1 * time.Second
	}
	fe.FrontendOpts = opts

	// find a TTY anywhere in stdio. stdout might be redirected, in which case we
	// can show the TUI on stderr.
	ttyIn, ttyOut := findTTYs()

	// run the function wrapped in the TUI
	runErr := fe.runWithTUI(ctx, ttyIn, ttyOut, run)

	// print the final output display to stderr
	if renderErr := fe.finalRender(); renderErr != nil {
		return renderErr
	}

	// return original err
	return runErr
}

func (fe *frontendPretty) SetPrimary(spanID trace.SpanID) {
	fe.mu.Lock()
	fe.db.SetPrimarySpan(spanID)
	fe.ZoomedSpan = spanID
	fe.FocusedSpan = spanID
	fe.recalculateViewLocked()
	fe.mu.Unlock()
}

func (fe *frontendPretty) RevealAllSpans() {
	fe.mu.Lock()
	fe.ZoomedSpan = trace.SpanID{}
	fe.mu.Unlock()
}

func (fe *frontendPretty) runWithTUI(ctx context.Context, ttyIn *os.File, ttyOut *os.File, run func(context.Context) error) error {
	var stdin io.Reader
	if ttyIn != nil {
		stdin = ttyIn

		// Bubbletea will just receive an `io.Reader` for its input rather than the
		// raw TTY *os.File, so we need to set up the TTY ourselves.
		ttyFd := int(ttyIn.Fd())
		oldState, err := term.MakeRaw(ttyFd)
		if err != nil {
			return err
		}
		fe.restore = func() { _ = term.Restore(ttyFd, oldState) }
		defer fe.restore()
	}

	// wire up the run so we can call it asynchronously with the TUI running
	fe.run = run
	// set up ctx cancellation so the TUI can interrupt via keypresses
	fe.runCtx, fe.interrupt = context.WithCancel(ctx)

	// keep program state so we can send messages to it
	fe.program = tea.NewProgram(fe,
		tea.WithInput(stdin),
		tea.WithOutput(ttyOut),
		// We set up the TTY ourselves, so Bubbletea's panic handler becomes
		// counter-productive.
		tea.WithoutCatchPanics(),
		tea.WithMouseCellMotion(),
	)

	// prevent browser.OpenURL from breaking the TUI if it fails
	browser.Stdout = fe.browserBuf
	browser.Stderr = fe.browserBuf

	// run the program, which starts the callback async
	if _, err := fe.program.Run(); err != nil {
		return err
	}

	// if the ctx was canceled, we don't need to return whatever random garbage
	// error string we got back; just return the ctx err.
	if fe.runCtx.Err() != nil {
		return fe.runCtx.Err()
	}

	// return the run err result
	return fe.err
}

func (fe *frontendPretty) renderErrorLogs(out *termenv.Output, r *renderer) error {
	if fe.rowsView == nil {
		return nil
	}
	errTree := fe.db.CollectErrors(fe.rowsView)
	var anyHasLogs bool
	WalkTree(errTree, func(row *TraceTree, _ int) bool {
		logs := fe.logs.Logs[row.Span.ID]
		if logs != nil && logs.UsedHeight() > 0 {
			anyHasLogs = true
			return true
		}
		return false
	})
	if anyHasLogs {
		fmt.Fprintln(out)
		fmt.Fprintln(out, out.String("Error logs:").Bold())
	}
	WalkTree(errTree, func(row *TraceTree, _ int) bool {
		logs := fe.logs.Logs[row.Span.ID]
		if logs != nil && logs.UsedHeight() > 0 {
			fmt.Fprintln(out)
			fe.renderStep(out, r, row.Span, row.Chained, 0, "")
			fe.renderLogs(out, r, logs, -1, logs.UsedHeight(), "")
		}
		return false
	})
	return nil
}

// finalRender is called after the program has finished running and prints the
// final output after the TUI has exited.
func (fe *frontendPretty) finalRender() error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	r := newRenderer(fe.db, fe.window.Width, fe.FrontendOpts)

	// Render the full trace.
	fe.ZoomedSpan = fe.db.PrimarySpan
	fe.FocusedSpan = trace.SpanID{}
	fe.focusedIdx = -1
	fe.recalculateViewLocked()

	// Render to stderr so stdout stays clean.
	out := NewOutput(os.Stderr, termenv.WithProfile(fe.profile))

	var renderedProgress bool
	if fe.Debug || fe.Verbosity >= ShowCompletedVerbosity || fe.err != nil {
		if fe.msgPreFinalRender.Len() > 0 {
			fmt.Fprintf(os.Stderr, fe.msgPreFinalRender.String()+"\n\n")
		}
		renderedProgress = fe.renderProgress(out, r, true, fe.window.Height, "")
	}

	// If there are errors, show log output.
	if fe.err != nil && renderedProgress {
		// Counter-intuitively, we don't want to render the primary output
		// when there's an error, because the error is better represented by
		// the progress output and error summary.
		return fe.renderErrorLogs(out, r)
	}

	if renderedProgress {
		// Print a blank line after progress if there is any primary output to
		// show.
		logs := fe.logs.Logs[fe.db.PrimarySpan]
		if logs != nil && logs.UsedHeight() > 0 {
			fmt.Fprintln(os.Stderr)
		}
	}

	// Replay the primary output log to stdout/stderr.
	return renderPrimaryOutput(fe.db)
}

func (fe *frontendPretty) SpanExporter() sdktrace.SpanExporter {
	return FrontendSpanExporter{fe}
}

type FrontendSpanExporter struct {
	*frontendPretty
}

func (fe FrontendSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	defer fe.recalculateViewLocked() // recalculate view *after* updating the db
	slog.Debug("frontend exporting spans", "spans", len(spans))
	return fe.db.ExportSpans(ctx, spans)
}

func (fe *frontendPretty) Shutdown(ctx context.Context) error {
	if err := fe.db.Shutdown(ctx); err != nil {
		return err
	}
	return fe.Close()
}

func (fe *frontendPretty) LogExporter() sdklog.Exporter {
	return prettyLogExporter{fe}
}

type prettyLogExporter struct {
	*frontendPretty
}

func (fe prettyLogExporter) Export(ctx context.Context, logs []sdklog.Record) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	if err := fe.db.LogExporter().Export(ctx, logs); err != nil {
		return err
	}
	return fe.logs.Export(ctx, logs)
}

type eofMsg struct{}

func (fe *frontendPretty) ForceFlush(context.Context) error {
	return nil
}

func (fe *frontendPretty) Close() error {
	if fe.program != nil {
		fe.program.Send(eofMsg{})
	}
	return nil
}

type backgroundMsg struct {
	cmd  tea.ExecCommand
	errs chan<- error
}

func (fe *frontendPretty) Background(cmd tea.ExecCommand) error {
	errs := make(chan error, 1)
	fe.program.Send(backgroundMsg{
		cmd:  cmd,
		errs: errs,
	})
	return <-errs
}

var KeymapStyle = lipgloss.NewStyle().
	Foreground(lipgloss.ANSIColor(termenv.ANSIBrightBlack))

func (fe *frontendPretty) renderKeymap(out *termenv.Output, style lipgloss.Style) int {
	w := new(strings.Builder)
	type keyHelp struct {
		label string
		keys  []string
		show  bool
	}
	var quitMsg string
	if fe.interrupted {
		quitMsg = "quit!"
	} else {
		quitMsg = "quit"
	}

	var showedKey bool
	// Blank line prior to keymap
	for _, key := range []keyHelp{
		{out.Hyperlink(fe.cloudURL, "web"), []string{"w"}, fe.cloudURL != ""},
		{"move", []string{"←↑↓→", "up", "down", "left", "right", "h", "j", "k", "l"}, true},
		{"first", []string{"home"}, true},
		{"last", []string{"end", " "}, true},
		{"zoom", []string{"enter"}, true},
		{"unzoom", []string{"esc"}, fe.ZoomedSpan.IsValid() &&
			fe.ZoomedSpan != fe.db.PrimarySpan},
		{fmt.Sprintf("verbosity=%d", fe.Verbosity), []string{"+/-", "+", "-"}, true},
		{quitMsg, []string{"q", "ctrl+c"}, true},
	} {
		if !key.show {
			continue
		}
		mainKey := key.keys[0]
		if showedKey {
			fmt.Fprint(w, style.Render("  "))
		}
		keyStyle := style
		if time.Since(fe.pressedKeyAt) < 500*time.Millisecond {
			for _, k := range key.keys {
				if k == fe.pressedKey {
					keyStyle = keyStyle.Foreground(nil)
					// Reverse(true)
				}
			}
		}
		fmt.Fprint(w, keyStyle.Bold(true).Render(mainKey))
		fmt.Fprint(w, keyStyle.Render(": "+key.label))
		showedKey = true
	}
	res := w.String()
	fmt.Fprint(out, res)
	return lipgloss.Width(res)
}

func (fe *frontendPretty) Render(out *termenv.Output) error {
	progHeight := fe.window.Height

	r := newRenderer(fe.db, fe.window.Width, fe.FrontendOpts)

	var progPrefix string
	if fe.rowsView != nil && fe.rowsView.Zoomed != nil && fe.rowsView.Zoomed.ID != fe.db.PrimarySpan {
		fe.renderStep(out, r, fe.rowsView.Zoomed, false, 0, "")
		progHeight -= 1
		progPrefix = "  "
	}

	below := new(strings.Builder)
	countOut := NewOutput(below, termenv.WithProfile(fe.profile))

	fmt.Fprint(countOut, KeymapStyle.Render(strings.Repeat(HorizBar, 1)))
	fmt.Fprint(countOut, KeymapStyle.Render(" "))
	fe.renderKeymap(countOut, KeymapStyle)
	fmt.Fprint(countOut, KeymapStyle.Render(" "))
	if rest := fe.window.Width - lipgloss.Width(below.String()); rest > 0 {
		fmt.Fprint(countOut, KeymapStyle.Render(strings.Repeat(HorizBar, rest)))
	}

	if logs := fe.logs.Logs[fe.ZoomedSpan]; logs != nil && logs.UsedHeight() > 0 {
		fmt.Fprintln(below)
		fe.renderLogs(countOut, r, logs, -1, fe.window.Height/3, progPrefix)
	}

	belowOut := strings.TrimRight(below.String(), "\n")
	progHeight -= lipgloss.Height(belowOut)

	fe.renderProgress(out, r, false, progHeight, progPrefix)
	fmt.Fprintln(out)

	fmt.Fprint(out, belowOut)
	return nil
}

func (fe *frontendPretty) recalculateViewLocked() {
	fe.rowsView = fe.db.RowsView(fe.FrontendOpts)
	fe.rows = fe.rowsView.Rows(fe.FrontendOpts)
	if len(fe.rows.Order) == 0 {
		fe.focusedIdx = -1
		fe.FocusedSpan = trace.SpanID{}
		return
	}
	if len(fe.rows.Order) < fe.focusedIdx {
		// durability: everything disappeared?
		fe.autoFocus = true
	}
	if fe.autoFocus {
		fe.focusedIdx = len(fe.rows.Order) - 1
		fe.FocusedSpan = fe.rows.Order[fe.focusedIdx].Span.ID
	} else if row := fe.rows.BySpan[fe.FocusedSpan]; row != nil {
		fe.focusedIdx = row.Index
	} else {
		// lost focus somehow
		fe.autoFocus = true
		fe.recalculateViewLocked()
	}
}

func (fe *frontendPretty) renderedRowLines(r *renderer, row *TraceRow, prefix string) []string {
	buf := new(strings.Builder)
	out := NewOutput(buf, termenv.WithProfile(fe.profile))
	fe.renderRow(out, r, row, false, prefix)
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

func (fe *frontendPretty) renderProgress(out *termenv.Output, r *renderer, full bool, height int, prefix string) bool {
	var renderedAny bool
	if fe.rowsView == nil {
		return false
	}

	rows := fe.rows

	if full {
		for _, row := range rows.Order {
			fe.renderRow(out, r, row, full, "")
			renderedAny = true
		}
		return renderedAny
	}

	lines := fe.renderLines(r, height, prefix)

	fmt.Fprint(out, strings.Join(lines, "\n"))
	renderedAny = true

	return renderedAny
}

func (fe *frontendPretty) renderLines(r *renderer, height int, prefix string) []string {
	rows := fe.rows
	if len(rows.Order) == 0 {
		return []string{}
	}
	if fe.focusedIdx == -1 {
		fe.autoFocus = true
		fe.focusedIdx = len(rows.Order) - 1
	}

	before, focused, after :=
		rows.Order[:fe.focusedIdx],
		rows.Order[fe.focusedIdx],
		rows.Order[fe.focusedIdx+1:]

	beforeLines := []string{}
	focusedLines := fe.renderedRowLines(r, focused, prefix)
	afterLines := []string{}
	renderBefore := func() {
		row := before[len(before)-1]
		before = before[:len(before)-1]
		beforeLines = append(fe.renderedRowLines(r, row, prefix), beforeLines...)
	}
	renderAfter := func() {
		row := after[0]
		after = after[1:]
		afterLines = append(afterLines, fe.renderedRowLines(r, row, prefix)...)
	}
	totalLines := func() int {
		return len(beforeLines) + len(focusedLines) + len(afterLines)
	}

	// fill in context surrounding the focused row
	contextLines := (height - len(focusedLines))
	if contextLines <= 0 {
		// lines already meets/exceeds height, just show them
		return focusedLines
	}

	beforeTargetLines := contextLines / 2
	var afterTargetLines int
	if contextLines%2 == 0 {
		afterTargetLines = beforeTargetLines
	} else {
		afterTargetLines = beforeTargetLines + 1
	}
	for len(beforeLines) < beforeTargetLines && len(before) > 0 {
		renderBefore()
	}
	for len(afterLines) < afterTargetLines && len(after) > 0 {
		renderAfter()
	}

	if total := totalLines(); total > height {
		extra := total - height
		if len(beforeLines) >= beforeTargetLines && len(afterLines) >= afterTargetLines {
			// exceeded the height, so trim the context
			if len(beforeLines) > beforeTargetLines {
				beforeLines = beforeLines[len(beforeLines)-beforeTargetLines:]
			}
			if len(afterLines) > afterTargetLines {
				afterLines = afterLines[:afterTargetLines]
			}
		} else if len(beforeLines) >= beforeTargetLines {
			beforeLines = beforeLines[extra:]
		} else if len(afterLines) >= afterTargetLines {
			afterLines = afterLines[:len(afterLines)-extra]
		}
	} else {
		// fill in the rest of the screen if there's not enough to fill both sides
		for totalLines() < height && (len(before) > 0 || len(after) > 0) {
			switch {
			case len(before) > 0:
				renderBefore()
				if total := totalLines(); total > height {
					extra := total - height
					beforeLines = beforeLines[extra:]
				}
			case len(after) > 0:
				renderAfter()
				if total := totalLines(); total > height {
					extra := total - height
					afterLines = afterLines[:len(afterLines)-extra]
				}
			}
		}
	}

	// finally, print all the lines
	focusedLines = append(beforeLines, focusedLines...)
	focusedLines = append(focusedLines, afterLines...)
	return focusedLines
}

func (fe *frontendPretty) focus(row *TraceRow) {
	if row == nil {
		return
	}
	fe.FocusedSpan = row.Span.ID
	fe.focusedIdx = row.Index
	fe.recalculateViewLocked()
}

func (fe *frontendPretty) Init() tea.Cmd {
	return tea.Batch(
		frame(fe.fps),
		fe.spawn,
	)
}

func (fe *frontendPretty) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	var cmds []tea.Cmd

	fe, cmd := fe.update(msg)
	cmds = append(cmds, cmd)

	// fe.viewport, cmd = fe.viewport.Update(msg)
	// cmds = append(cmds, cmd)
	//
	return fe, tea.Batch(cmds...)
}

func (fe *frontendPretty) View() string {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	if fe.backgrounded {
		// if we've been backgrounded, show nothing, so a user's shell session
		// doesn't have any garbage before/after
		return ""
	}
	if fe.quitting {
		// print nothing; make way for the pristine output in the final render
		return ""
	}
	return fe.view.String()
}

type doneMsg struct {
	err error
}

func (fe *frontendPretty) spawn() (msg tea.Msg) {
	defer func() {
		if r := recover(); r != nil {
			fe.restore()
			panic(r)
		}
	}()
	return doneMsg{fe.run(fe.runCtx)}
}

type backgroundDoneMsg struct{}

func (fe *frontendPretty) update(msg tea.Msg) (*frontendPretty, tea.Cmd) { //nolint: gocyclo
	switch msg := msg.(type) {
	case doneMsg: // run finished
		slog.Debug("run finished", "err", msg.err)
		fe.done = true
		fe.err = msg.err
		if fe.eof && !fe.NoExit {
			fe.quitting = true
			return fe, tea.Quit
		}
		return fe, nil

	case eofMsg: // received end of updates
		slog.Debug("got EOF")
		fe.eof = true
		if fe.done && !fe.NoExit {
			fe.quitting = true
			return fe, tea.Quit
		}
		return fe, nil

	case backgroundMsg:
		fe.backgrounded = true
		return fe, tea.Exec(msg.cmd, func(err error) tea.Msg {
			msg.errs <- err
			return backgroundDoneMsg{}
		})

	case backgroundDoneMsg:
		return fe, nil

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelDown:
			fe.goDown()
			fe.pressedKey = "down"
			fe.pressedKeyAt = time.Now()
		case tea.MouseButtonWheelUp:
			fe.goUp()
			fe.pressedKey = "up"
			fe.pressedKeyAt = time.Now()
		}
		return fe, nil

	case tea.KeyMsg:
		lastKey := fe.pressedKey
		fe.pressedKey = msg.String()
		fe.pressedKeyAt = time.Now()
		switch msg.String() {
		case "q", "ctrl+c":
			if fe.done && fe.eof {
				fe.quitting = true
				// must have configured NoExit, and now they want
				// to exit manually
				return fe, tea.Quit
			}
			if fe.interrupted {
				slog.Warn("exiting immediately")
				fe.quitting = true
				return fe, tea.Quit
			} else {
				slog.Warn("canceling... (press again to exit immediately)")
			}
			fe.interrupted = true
			fe.interrupt()
			return fe, nil // tea.Quit is deferred until we receive doneMsg
		case "ctrl+\\": // SIGQUIT
			fe.restore()
			sigquit()
			return fe, nil
		case "down", "j":
			fe.goDown()
			return fe, nil
		case "up", "k":
			fe.goUp()
			return fe, nil
		case "left", "h":
			fe.goOut()
			return fe, nil
		case "right", "l":
			fe.goIn()
			return fe, nil
		case "home":
			fe.goStart()
			return fe, nil
		case "end", "G", " ":
			fe.goEnd()
			fe.pressedKey = "end"
			fe.pressedKeyAt = time.Now()
			return fe, nil
		case "esc":
			fe.ZoomedSpan = fe.db.PrimarySpan
			fe.recalculateViewLocked()
			return fe, nil
		case "+":
			fe.FrontendOpts.Verbosity++
			fe.recalculateViewLocked()
			return fe, nil
		case "-":
			fe.FrontendOpts.Verbosity--
			if fe.FrontendOpts.Verbosity < 0 {
				fe.FrontendOpts.Verbosity = 0
			}
			fe.recalculateViewLocked()
			return fe, nil
		case "w":
			if fe.cloudURL == "" {
				return fe, nil
			}
			url := fe.cloudURL
			if fe.ZoomedSpan.IsValid() && fe.ZoomedSpan != fe.db.PrimarySpan {
				url += "?span=" + fe.ZoomedSpan.String()
			}
			if fe.FocusedSpan.IsValid() && fe.FocusedSpan != fe.db.PrimarySpan {
				url += "#" + fe.FocusedSpan.String()
			}
			return fe, func() tea.Msg {
				if err := browser.OpenURL(url); err != nil {
					slog.Warn("failed to open URL",
						"url", url,
						"err", err,
						"output", fe.browserBuf.String())
				}
				return nil
			}
		case "?":
			fe.debugged = fe.FocusedSpan
			return fe, nil
		case "enter":
			fe.ZoomedSpan = fe.FocusedSpan
			fe.recalculateViewLocked()
			return fe, nil
		}

		switch lastKey { //nolint:gocritic
		case "g":
			switch msg.String() { //nolint:gocritic
			case "g":
				fe.goStart()
				fe.pressedKey = "home"
				fe.pressedKeyAt = time.Now()
				return fe, nil
			}
		}

		return fe, nil
	case tea.WindowSizeMsg:
		fe.setWindowSizeLocked(msg)
		return fe, nil

	case frameMsg:
		fe.renderLocked()
		// NB: take care not to forward Frame downstream, since that will result
		// in runaway ticks. instead inner components should send a SetFpsMsg to
		// adjust the outermost layer.
		return fe, frame(fe.fps)

	default:
		return fe, nil
	}
}

func (fe *frontendPretty) goStart() {
	fe.autoFocus = false
	if len(fe.rows.Order) > 0 {
		fe.focus(fe.rows.Order[0])
	}
}

func (fe *frontendPretty) goEnd() {
	fe.autoFocus = true
	if len(fe.rows.Order) > 0 {
		fe.focus(fe.rows.Order[len(fe.rows.Order)-1])
	}
}

func (fe *frontendPretty) goUp() {
	fe.autoFocus = false
	newIdx := fe.focusedIdx - 1
	if newIdx < 0 || newIdx >= len(fe.rows.Order) {
		return
	}
	fe.focus(fe.rows.Order[newIdx])
}

func (fe *frontendPretty) goDown() {
	fe.autoFocus = false
	newIdx := fe.focusedIdx + 1
	if newIdx >= len(fe.rows.Order) {
		// at bottom
		return
	}
	fe.focus(fe.rows.Order[newIdx])
}

func (fe *frontendPretty) goOut() {
	fe.autoFocus = false
	focused := fe.db.Spans.Map[fe.FocusedSpan]
	if focused == nil {
		return
	}
	parent := focused.VisibleParent(fe.FrontendOpts)
	if parent == nil {
		return
	}
	fe.FocusedSpan = parent.ID
	// targeted the zoomed span; zoom on its parent instead
	if fe.FocusedSpan == fe.ZoomedSpan {
		zoomedParent := parent.VisibleParent(fe.FrontendOpts)
		if zoomedParent != nil {
			fe.ZoomedSpan = zoomedParent.ID
		}
	}
	fe.recalculateViewLocked()
}

func (fe *frontendPretty) goIn() {
	fe.autoFocus = false
	newIdx := fe.focusedIdx + 1
	if newIdx >= len(fe.rows.Order) {
		// at bottom
		return
	}
	cur := fe.rows.Order[fe.focusedIdx]
	next := fe.rows.Order[newIdx]
	if next.Depth <= cur.Depth {
		// has no children
		return
	}
	fe.focus(next)
}

func (fe *frontendPretty) setWindowSizeLocked(msg tea.WindowSizeMsg) {
	fe.window = msg
	fe.logs.SetWidth(msg.Width)
}

func (fe *frontendPretty) renderLocked() {
	fe.view.Reset()
	fe.Render(fe.viewOut)
}

func (fe *frontendPretty) renderRow(out *termenv.Output, r *renderer, row *TraceRow, final bool, prefix string) {
	if row.Previous != nil &&
		row.Previous.Depth >= row.Depth &&
		!row.Chained &&
		(row.Previous.Depth > row.Depth || row.Span.Call != nil ||
			(row.Previous.Span.Call != nil && row.Span.Call == nil)) {
		fmt.Fprint(out, prefix)
		r.indent(out, row.Depth)
		fmt.Fprintln(out)
	}
	fe.renderStep(out, r, row.Span, row.Chained, row.Depth, prefix)
	if row.IsRunningOrChildRunning || row.Span.IsFailed() || fe.Verbosity >= ShowSpammyVerbosity {
		if logs := fe.logs.Logs[row.Span.ID]; logs != nil {
			fe.renderLogs(out, r,
				logs,
				row.Depth,
				fe.window.Height/3,
				prefix,
			)
		}
	}
}

func (fe *frontendPretty) renderStep(out *termenv.Output, r *renderer, span *Span, chained bool, depth int, prefix string) error {
	isFocused := span.ID == fe.FocusedSpan

	id := span.Call
	if id != nil {
		if err := r.renderCall(out, span, id, prefix, chained, depth, false, span.Internal, isFocused); err != nil {
			return err
		}
	} else if span != nil {
		if err := r.renderSpan(out, span, span.Name(), prefix, depth, isFocused); err != nil {
			return err
		}
	}
	fmt.Fprintln(out)

	if span.Status().Code == codes.Error && span.Status().Description != "" {
		// only print the first line
		line := strings.Split(span.Status().Description, "\n")[0]
		fmt.Fprint(out, prefix)
		r.indent(out, depth)
		fmt.Fprintf(out,
			out.String("! %s").Foreground(termenv.ANSIYellow).String(),
			line,
		)
		fmt.Fprintln(out)
	}

	if span.ID == fe.debugged {
		pending, reasons := span.PendingReason()
		r.indent(out, depth+1)
		fmt.Fprintf(out, prefix+"? pending: %v\n", pending)
		for _, reason := range reasons {
			r.indent(out, depth+1)
			fmt.Fprintln(out, prefix+"- "+reason)
		}
		cached, reasons := span.CachedReason()
		r.indent(out, depth+1)
		fmt.Fprintf(out, prefix+"? cached: %v\n", cached)
		for _, reason := range reasons {
			r.indent(out, depth+1)
			fmt.Fprintln(out, prefix+"- "+reason)
		}
	}

	return nil
}

func (fe *frontendPretty) renderLogs(out *termenv.Output, r *renderer, logs *Vterm, depth int, height int, prefix string) {
	pipe := out.String(VertBoldBar).Foreground(termenv.ANSIBrightBlack)
	if depth == -1 {
		// clear prefix when zoomed
		logs.SetPrefix(prefix)
	} else {
		buf := new(strings.Builder)
		fmt.Fprint(buf, prefix)
		indentOut := NewOutput(buf, termenv.WithProfile(fe.profile))
		r.indent(indentOut, depth)
		fmt.Fprint(indentOut, pipe.String()+" ")
		logs.SetPrefix(buf.String())
	}
	logs.SetHeight(height)
	fmt.Fprint(out, logs.View())
}

type prettyLogs struct {
	Logs     map[trace.SpanID]*Vterm
	LogWidth int
}

func newPrettyLogs() *prettyLogs {
	return &prettyLogs{
		Logs:     make(map[trace.SpanID]*Vterm),
		LogWidth: -1,
	}
}

func (l *prettyLogs) Export(ctx context.Context, logs []sdklog.Record) error {
	for _, log := range logs {
		// render vterm for TUI
		_, _ = fmt.Fprint(l.spanLogs(log.SpanID()), log.Body().AsString())
	}
	return nil
}

func (l *prettyLogs) spanLogs(id trace.SpanID) *Vterm {
	term, found := l.Logs[id]
	if !found {
		term = NewVterm()
		if l.LogWidth > -1 {
			term.SetWidth(l.LogWidth)
		}
		l.Logs[id] = term
	}
	return term
}

func (l *prettyLogs) SetWidth(width int) {
	l.LogWidth = width
	for _, vt := range l.Logs {
		vt.SetWidth(width)
	}
}

func (l *prettyLogs) Shutdown(ctx context.Context) error {
	return nil
}

func findTTYs() (in *os.File, out *os.File) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		in = os.Stdin
	}
	for _, f := range []*os.File{os.Stderr, os.Stdout} {
		if term.IsTerminal(int(f.Fd())) {
			out = f
			break
		}
	}
	return
}

type frameMsg time.Time

func frame(fps float64) tea.Cmd {
	return tea.Tick(time.Duration(float64(time.Second)/fps), func(t time.Time) tea.Msg {
		return frameMsg(t)
	})
}
