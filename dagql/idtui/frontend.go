package idtui

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
	"github.com/opencontainers/go-digest"
	"github.com/vito/midterm"
	"github.com/vito/progrock/ui"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/term"

	"github.com/dagger/dagger/dagql/call"
	"github.com/dagger/dagger/telemetry/sdklog"
	"github.com/dagger/dagger/tracing"
)

var consoleSink = os.Stderr

type Frontend struct {
	// Debug tells the frontend to show everything and do one big final render.
	Debug bool

	// Plain tells the frontend to render plain console output instead of using a
	// TUI. This will be automatically set to true if a TTY is not found.
	Plain bool

	// Silent tells the frontend to not display progress at all.
	Silent bool

	// Verbosity is the level of detail to show in the TUI.
	Verbosity int

	// updated by Run
	program     *tea.Program
	in          *swappableWriter
	out         *termenv.Output
	run         func(context.Context) error
	runCtx      context.Context
	interrupt   func()
	interrupted bool
	done        bool
	err         error

	// updated as events are written
	db       *DB
	eof      bool
	logsView *LogsView

	// global logs
	messagesView *Vterm
	messagesBuf  *strings.Builder
	messagesW    *termenv.Output

	// TUI state/config
	restore func()  // restore terminal
	fps     float64 // frames per second
	profile termenv.Profile
	window  tea.WindowSizeMsg // set by BubbleTea
	view    *strings.Builder  // rendered async

	// held to synchronize tea.Model with updates
	mu sync.Mutex
}

type zoomState struct {
	Input  io.Writer
	Output *midterm.Terminal
}

func New() *Frontend {
	profile := ui.ColorProfile()
	logsView := NewVterm()
	logsOut := new(strings.Builder)
	return &Frontend{
		db: NewDB(),

		fps:          30, // sane default, fine-tune if needed
		profile:      profile,
		window:       tea.WindowSizeMsg{Width: -1, Height: -1}, // be clear that it's not set
		view:         new(strings.Builder),
		messagesView: logsView,
		messagesBuf:  logsOut,
		messagesW:    ui.NewOutput(io.MultiWriter(logsView, logsOut), termenv.WithProfile(profile)),
	}
}

// Run starts the TUI, calls the run function, stops the TUI, and finally
// prints the primary output to the appropriate stdout/stderr streams.
func (fe *Frontend) Run(ctx context.Context, run func(context.Context) error) error {
	// find a TTY anywhere in stdio. stdout might be redirected, in which case we
	// can show the TUI on stderr.
	tty, isTTY := findTTY()
	if !isTTY {
		// Simplify logic elsewhere by just setting Plain to true.
		fe.Plain = true
	}

	var runErr error
	if fe.Plain || fe.Silent {
		// no TTY found; just run normally and do a final render
		runErr = run(ctx)
	} else {
		// run the TUI until it exits and cleans up the TTY
		runErr = fe.runWithTUI(ctx, tty, run)
	}

	// print the final output display to stderr
	if renderErr := fe.finalRender(); renderErr != nil {
		return renderErr
	}

	// return original err
	return runErr
}

// ConnectedToEngine is called when the CLI connects to an engine.
func (fe *Frontend) ConnectedToEngine(name string) {
	if !fe.Silent && fe.Plain {
		fmt.Fprintln(consoleSink, "Connected to engine", name)
	}
}

// ConnectedToCloud is called when the CLI has started emitting events to
// The Cloud.
func (fe *Frontend) ConnectedToCloud(cloudURL string) {
	if !fe.Silent && fe.Plain {
		fmt.Fprintln(consoleSink, "Dagger Cloud URL:", cloudURL)
	}
}

func (fe *Frontend) runWithTUI(ctx context.Context, tty *os.File, run func(context.Context) error) error {
	// redirect slog to the logs pane
	level := slog.LevelWarn
	if fe.Debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(tracing.PrettyLogger(fe.messagesW, level))

	// NOTE: establish color cache before we start consuming stdin
	fe.out = ui.NewOutput(tty, termenv.WithProfile(fe.profile), termenv.WithColorCache(true))

	// in order to allow the TUI to receive user input but _also_ allow an
	// interactive terminal to receive keyboard input, we pipe the user input
	// to an io.Writer that can have its destination swapped between the TUI
	// and the remote terminal.
	inR, inW := io.Pipe()
	fe.in = &swappableWriter{original: inW}

	// Bubbletea will just receive an `io.Reader` for its input rather than the
	// raw TTY *os.File, so we need to set up the TTY ourselves.
	ttyFd := int(tty.Fd())
	oldState, err := term.MakeRaw(ttyFd)
	if err != nil {
		return err
	}
	fe.restore = func() { _ = term.Restore(ttyFd, oldState) }
	defer fe.restore()

	// start piping from the TTY to our swappable writer.
	go io.Copy(fe.in, tty) //nolint: errcheck

	// support scrollable viewport
	// fe.out.EnableMouseCellMotion()

	// wire up the run so we can call it asynchronously with the TUI running
	fe.run = run
	// set up ctx cancellation so the TUI can interrupt via keypresses
	fe.runCtx, fe.interrupt = context.WithCancel(ctx)

	// keep program state so we can send messages to it
	fe.program = tea.NewProgram(fe,
		tea.WithInput(inR),
		tea.WithOutput(fe.out),
		// We set up the TTY ourselves, so Bubbletea's panic handler becomes
		// counter-productive.
		tea.WithoutCatchPanics(),
	)

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

// finalRender is called after the program has finished running and prints the
// final output after the TUI has exited.
func (fe *Frontend) finalRender() error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	fe.recalculateView()

	out := termenv.NewOutput(os.Stderr)

	if fe.messagesBuf.Len() > 0 {
		fmt.Fprintln(out, fe.messagesBuf.String())
	}

	if fe.Debug || fe.Verbosity > 0 || fe.err != nil {
		if renderedAny, err := fe.renderProgress(out); err != nil {
			return err
		} else if renderedAny {
			fmt.Fprintln(out)
		}
	}

	return fe.renderPrimaryOutput()
}

func (fe *Frontend) renderMessages(out *termenv.Output, full bool) (bool, error) {
	if fe.messagesView.UsedHeight() == 0 {
		return false, nil
	}
	if full {
		fe.messagesView.SetHeight(fe.messagesView.UsedHeight())
	} else {
		fe.messagesView.SetHeight(10)
	}
	_, err := fmt.Fprint(out, fe.messagesView.View())
	return true, err
}

func (fe *Frontend) renderPrimaryOutput() error {
	logs := fe.db.PrimaryLogs[fe.db.PrimarySpan]
	if len(logs) == 0 {
		return nil
	}
	var trailingLn bool
	for _, l := range logs {
		data := l.Body().AsString()
		if strings.HasSuffix(data, "\n") {
			trailingLn = true
		}
		var stream int
		l.WalkAttributes(func(attr log.KeyValue) bool {
			if attr.Key == tracing.LogStreamAttr {
				stream = int(attr.Value.AsInt64())
				return false
			}
			return true
		})
		switch stream {
		case 1:
			if _, err := fmt.Fprint(os.Stdout, data); err != nil {
				return err
			}
		case 2:
			if _, err := fmt.Fprint(os.Stderr, data); err != nil {
				return err
			}
		}
	}
	if !trailingLn && term.IsTerminal(int(os.Stdout.Fd())) {
		// NB: ensure there's a trailing newline if stdout is a TTY, so we don't
		// encourage module authors to add one of their own
		fmt.Fprintln(os.Stdout)
	}
	return nil
}

func (fe *Frontend) redirectStdin(st *zoomState) {
	if st == nil {
		fe.in.Restore()
		// restore scrolling as we transition back to the DAG UI, since an app
		// may have disabled it
		// fe.out.EnableMouseCellMotion()
	} else {
		// disable mouse events, can't assume zoomed input wants it (might be
		// regular shell like sh)
		// fe.out.DisableMouseCellMotion()
		fe.in.SetOverride(st.Input)
	}
}

func findTTY() (*os.File, bool) {
	// some of these may be redirected
	for _, f := range []*os.File{os.Stderr, os.Stdout, os.Stdin} {
		if term.IsTerminal(int(f.Fd())) {
			return f, true
		}
	}
	return nil, false
}

type swappableWriter struct {
	original io.Writer
	override io.Writer
	sync.Mutex
}

func (w *swappableWriter) SetOverride(to io.Writer) {
	w.Lock()
	w.override = to
	w.Unlock()
}

func (w *swappableWriter) Restore() {
	w.SetOverride(nil)
}

func (w *swappableWriter) Write(p []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	if w.override != nil {
		return w.override.Write(p)
	}
	return w.original.Write(p)
}

var _ sdktrace.SpanExporter = (*Frontend)(nil)

func (fe *Frontend) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	slog.Debug("frontend exporting", "spans", len(spans))
	for _, span := range spans {
		slog.Debug("frontend exporting span",
			"trace", span.SpanContext().TraceID(),
			"id", span.SpanContext().SpanID(),
			"parent", span.Parent().SpanID(),
			"span", span.Name(),
		)
	}
	return fe.db.ExportSpans(ctx, spans)
}

var _ sdklog.LogExporter = (*Frontend)(nil)

func (fe *Frontend) ExportLogs(ctx context.Context, logs []*sdklog.LogData) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	slog.Debug("frontend exporting logs", "logs", len(logs))
	return fe.db.ExportLogs(ctx, logs)
}

func (fe *Frontend) Shutdown(ctx context.Context) error {
	// TODO this gets called twice (once for traces, once for logs)
	if err := fe.db.Shutdown(ctx); err != nil {
		return err
	}
	return fe.Close()
}

type eofMsg struct{}

func (fe *Frontend) Close() error {
	if fe.program != nil {
		fe.program.Send(eofMsg{})
	}
	return nil
}

func (fe *Frontend) Render(out *termenv.Output) error {
	fe.recalculateView()
	if _, err := fe.renderProgress(out); err != nil {
		return err
	}
	if _, err := fe.renderMessages(out, false); err != nil {
		return err
	}
	return nil
}

func (fe *Frontend) recalculateView() {
	steps := CollectSpans(fe.db, trace.TraceID{})
	rows := CollectRows(steps)
	fe.logsView = CollectLogsView(rows)
}

func (fe *Frontend) renderProgress(out *termenv.Output) (bool, error) {
	var renderedAny bool
	if fe.logsView == nil {
		return false, nil
	}
	for _, row := range fe.logsView.Body {
		if fe.Debug || row.IsInteresting(fe.Verbosity) {
			if err := fe.renderRow(out, row, 0); err != nil {
				return renderedAny, err
			}
			renderedAny = true
		}
	}
	if fe.logsView.Primary != nil && !fe.done {
		fe.renderLogs(out, fe.logsView.Primary, -1)
		renderedAny = true
	}
	return renderedAny, nil
}

func (fe *Frontend) renderZoomed(out *termenv.Output, st *zoomState) (bool, error) {
	var renderedAny bool
	for i := 0; i < st.Output.UsedHeight(); i++ {
		if i > 0 {
			fmt.Fprintln(out)
		}
		if err := st.Output.RenderLine(out, i); err != nil {
			return renderedAny, err
		}
		renderedAny = true
	}
	return renderedAny, nil
}

var _ tea.Model = (*Frontend)(nil)

func (fe *Frontend) Init() tea.Cmd {
	return tea.Batch(
		ui.Frame(fe.fps),
		fe.spawn,
	)
}

type doneMsg struct {
	err error
}

func (fe *Frontend) spawn() (msg tea.Msg) {
	defer func() {
		if r := recover(); r != nil {
			fe.restore()
			panic(r)
		}
	}()
	return doneMsg{fe.run(fe.runCtx)}
}

func (fe *Frontend) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case doneMsg: // run finished
		slog.Debug("run finished", "err", msg.err)
		fe.done = true
		fe.err = msg.err
		if fe.eof {
			return fe, tea.Quit
		}
		return fe, nil

	case eofMsg: // received end of updates
		slog.Debug("got EOF")
		fe.eof = true
		if fe.done {
			return fe, tea.Quit
		}
		return fe, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			if fe.interrupted {
				slog.Warn("exiting immediately")
				return fe, tea.Quit
			} else {
				slog.Warn("canceling... (press again to exit immediately)")
			}
			fe.interrupt()
			fe.interrupted = true
			return fe, nil // tea.Quit is deferred until we receive doneMsg
		case "ctrl+\\": // SIGQUIT
			fe.restore()
			syscall.Kill(syscall.Getpid(), syscall.SIGQUIT)
			return fe, nil
		default:
			return fe, nil
		}

	case tea.WindowSizeMsg:
		fe.window = msg
		fe.db.SetWidth(msg.Width)
		fe.messagesView.SetWidth(msg.Width)
		return fe, nil

	case ui.FrameMsg:
		fe.render()
		// NB: take care not to forward Frame downstream, since that will result
		// in runaway ticks. instead inner components should send a SetFpsMsg to
		// adjust the outermost layer.
		return fe, ui.Frame(fe.fps)

	default:
		return fe, nil
	}
}

func (fe *Frontend) render() {
	fe.mu.Lock()
	fe.view.Reset()
	fe.Render(ui.NewOutput(fe.view, termenv.WithProfile(fe.profile)))
	fe.mu.Unlock()
}

func (fe *Frontend) View() string {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	view := fe.view.String()
	if fe.done && fe.eof {
		// print nothing; make way for the pristine output in the final render
		return ""
	}
	return view
}

// DumpID is exposed for troubleshooting.
func (fe *Frontend) DumpID(out *termenv.Output, id *call.ID) error {
	if id.Base() != nil {
		if err := fe.DumpID(out, id.Base()); err != nil {
			return err
		}
	}
	return fe.renderID(out, nil, id, 0, false)
}

func (fe *Frontend) renderRow(out *termenv.Output, row *TraceRow, depth int) error {
	if !row.IsInteresting(fe.Verbosity) && !fe.Debug {
		return nil
	}
	if !row.Span.Passthrough {
		fe.renderStep(out, row.Span, depth)
		fe.renderLogs(out, row.Span, depth)
		depth++
	}
	if !row.Span.Encapsulate {
		for _, child := range row.Children {
			if err := fe.renderRow(out, child, depth); err != nil {
				return err
			}
		}
	}
	return nil
}

func (fe *Frontend) renderStep(out *termenv.Output, span *Span, depth int) error {
	id := span.Call
	if id != nil {
		if err := fe.renderID(out, span, id, depth, false); err != nil {
			return err
		}
	} else if span != nil {
		if err := fe.renderVertex(out, span, depth); err != nil {
			return err
		}
	}
	if span.Status().Code == codes.Error && span.Status().Description != "" {
		indent(out, depth)
		// print error description above it
		fmt.Fprintf(out,
			out.String("! %s\n").Foreground(termenv.ANSIYellow).String(),
			span.Status().Description,
		)
	}
	return nil
}

func (fe *Frontend) renderLogs(out *termenv.Output, span *Span, depth int) {
	if logs, ok := fe.db.Logs[span.SpanContext().SpanID()]; ok {
		pipe := out.String(ui.VertBoldBar).Foreground(termenv.ANSIBrightBlack)
		if depth != -1 {
			logs.SetPrefix(strings.Repeat("  ", depth) + pipe.String() + " ")
		}
		logs.SetHeight(fe.window.Height / 3)
		fmt.Fprint(out, logs.View())
	}
}

func indent(out io.Writer, depth int) {
	fmt.Fprint(out, strings.Repeat("  ", depth))
}

const (
	kwColor     = termenv.ANSICyan
	parentColor = termenv.ANSIWhite
	moduleColor = termenv.ANSIMagenta
)

func (fe *Frontend) renderIDBase(out *termenv.Output, id *call.ID) error {
	typeName := id.Type().ToAST().Name()
	parent := out.String(typeName)
	if id.Module() != nil {
		parent = parent.Foreground(moduleColor)
	}
	fmt.Fprint(out, parent.String())
	return nil
}

func (fe *Frontend) renderID(out *termenv.Output, span *Span, id *call.ID, depth int, inline bool) error {
	if !inline {
		indent(out, depth)
	}

	if span != nil {
		fe.renderStatus(out, span, depth)
	}

	if id.Base() != nil {
		if err := fe.renderIDBase(out, id.Base()); err != nil {
			return err
		}
		fmt.Fprint(out, ".")
	}

	fmt.Fprint(out, out.String(id.Field()).Bold())

	if len(id.Args()) > 0 {
		fmt.Fprint(out, "(")
		var needIndent bool
		for _, arg := range id.Args() {
			if _, ok := arg.Value().ToInput().(*call.ID); ok {
				needIndent = true
				break
			}
		}
		if needIndent {
			fmt.Fprintln(out)
			depth++
			depth++
			for _, arg := range id.Args() {
				indent(out, depth)
				fmt.Fprintf(out, out.String("%s:").Foreground(kwColor).String(), arg.Name())
				val := arg.Value()
				fmt.Fprint(out, " ")
				switch x := val.(type) {
				case *call.LiteralID:
					arg := x.Value()
					if baseStep, ok := fe.db.HighLevelCall(arg); ok {
						arg = baseStep
					}
					argDig := arg.Digest()
					argVtx := fe.db.MostInterestingSpan(argDig.String())
					if err := fe.renderID(out, argVtx, arg, depth-1, true); err != nil {
						return err
					}
				default:
					fe.renderLiteral(out, arg.Value())
					fmt.Fprintln(out)
				}
			}
			depth--
			indent(out, depth)
			depth-- //nolint:ineffassign
		} else {
			for i, arg := range id.Args() {
				if i > 0 {
					fmt.Fprint(out, ", ")
				}
				fmt.Fprintf(out, out.String("%s:").Foreground(kwColor).String()+" ", arg.Name())
				fe.renderLiteral(out, arg.Value())
			}
		}
		fmt.Fprint(out, ")")
	}

	typeStr := out.String(": " + id.Type().ToAST().String()).Faint()
	fmt.Fprint(out, typeStr)

	if span != nil {
		fe.renderDuration(out, span)
	}

	fmt.Fprintln(out)

	return nil
}

func (fe *Frontend) renderVertex(out *termenv.Output, span *Span, depth int) error {
	indent(out, depth)
	fe.renderStatus(out, span, depth)
	fmt.Fprint(out, span.Name())
	fe.renderVertexTasks(out, span, depth)
	fe.renderDuration(out, span)
	fmt.Fprintln(out)
	return nil
}

func (fe *Frontend) renderLiteral(out *termenv.Output, lit call.Literal) {
	var color termenv.Color
	switch val := lit.(type) {
	case *call.LiteralBool:
		color = termenv.ANSIRed
	case *call.LiteralInt:
		color = termenv.ANSIRed
	case *call.LiteralFloat:
		color = termenv.ANSIRed
	case *call.LiteralString:
		color = termenv.ANSIYellow
		if fe.window.Width != -1 && len(val.Value()) > fe.window.Width {
			display := string(digest.FromString(val.Value()))
			fmt.Fprint(out, out.String("ETOOBIG:"+display).Foreground(color))
			return
		}
	case *call.LiteralID:
		color = termenv.ANSIMagenta
	case *call.LiteralEnum:
		color = termenv.ANSIYellow
	case *call.LiteralNull:
		color = termenv.ANSIBrightBlack
	case *call.LiteralList:
		fmt.Fprint(out, "[")
		val.Range(func(i int, item call.Literal) error {
			if i > 0 {
				fmt.Fprint(out, ", ")
			}
			fe.renderLiteral(out, item)
			return nil
		})
		fmt.Fprint(out, "]")
		return
	case *call.LiteralObject:
		fmt.Fprint(out, "{")
		val.Range(func(i int, name string, value call.Literal) error {
			if i > 0 {
				fmt.Fprint(out, ", ")
			}
			fmt.Fprintf(out, "%s: ", name)
			fe.renderLiteral(out, value)
			return nil
		})
		fmt.Fprint(out, "}")
		return
	}
	fmt.Fprint(out, out.String(lit.ToAST().String()).Foreground(color))
}

func (fe *Frontend) renderStatus(out *termenv.Output, span *Span, depth int) {
	var symbol string
	var color termenv.Color
	switch {
	case span.IsRunning():
		symbol = ui.DotFilled
		color = termenv.ANSIYellow
	case span.Canceled:
		symbol = ui.IconSkipped
		color = termenv.ANSIBrightBlack
	case span.Status().Code == codes.Error:
		symbol = ui.IconFailure
		color = termenv.ANSIRed
	default:
		symbol = ui.IconSuccess
		color = termenv.ANSIGreen
	}

	symbol = out.String(symbol).Foreground(color).String()

	fmt.Fprintf(out, "%s ", symbol)
}

func (fe *Frontend) renderDuration(out *termenv.Output, span *Span) {
	fmt.Fprint(out, " ")
	duration := out.String(fmtDuration(span.Duration()))
	if span.IsRunning() {
		duration = duration.Foreground(termenv.ANSIYellow)
	} else {
		duration = duration.Faint()
	}
	fmt.Fprint(out, duration)
}

var (
	progChars = []string{"⠀", "⡀", "⣀", "⣄", "⣤", "⣦", "⣶", "⣷", "⣿"}
)

func (fe *Frontend) renderVertexTasks(out *termenv.Output, span *Span, depth int) error {
	tasks := fe.db.Tasks[span.SpanContext().SpanID()]
	if len(tasks) == 0 {
		return nil
	}
	var spaced bool
	for _, t := range tasks {
		var sym termenv.Style
		if t.Total != 0 {
			percent := int(100 * (float64(t.Current) / float64(t.Total)))
			idx := (len(progChars) - 1) * percent / 100
			chr := progChars[idx]
			sym = out.String(chr)
		} else {
			// TODO: don't bother printing non-progress-bar tasks for now
			// else if t.Completed != nil {
			// sym = out.String(ui.IconSuccess)
			// } else if t.Started != nil {
			// sym = out.String(ui.DotFilled)
			// }
			continue
		}
		if t.Completed.IsZero() {
			sym = sym.Foreground(termenv.ANSIYellow)
		} else {
			sym = sym.Foreground(termenv.ANSIGreen)
		}
		if !spaced {
			fmt.Fprint(out, " ")
			spaced = true
		}
		fmt.Fprint(out, sym)
	}
	return nil
}
