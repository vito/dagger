package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"dagger.io/dagger"
	"github.com/charmbracelet/bubbles/key"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/dagger/dagger/dagql/call"
	"github.com/dagger/dagger/dagql/dagui"
	"github.com/dagger/dagger/dagql/idtui"
	"github.com/dagger/dagger/engine/slog"
	telemetry "github.com/dagger/otel-go"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
	"github.com/vito/dang/pkg/dang"
	"github.com/vito/dang/pkg/hm"
	"github.com/vito/dang/pkg/ioctx"
	replpkg "github.com/vito/dang/pkg/repl"
	"github.com/vito/tuist"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

// dangShellHandler implements ShellHandler for the Dang language.
type dangShellHandler struct {
	dag      *dagger.Client
	frontend idtui.Frontend

	// Dang environments
	typeEnv       dang.Env
	evalEnv       dang.EvalEnv
	importConfigs []dang.ImportConfig
	debug         bool

	// tty is set to true when running the TUI (pretty frontend)
	tty bool

	// LLM support
	llmSession *LLMSession
	llmErr     error
	llmModel   string
	llmL       sync.Mutex

	// interpreter mode (dang or prompt)
	mode      interpreterMode
	savedMode interpreterMode

	// Completion infrastructure (from pkg/repl)
	completionProvider tuist.CompletionProvider
	staticCompletions  []string

	// cancel interrupts the entire shell session
	cancel func()

	// mu synchronizes access
	mu sync.RWMutex

	// ctx is the shell context
	ctx context.Context
}

func newDangShellHandler(dag *dagger.Client, fe idtui.Frontend) *dangShellHandler {
	return &dangShellHandler{
		dag:      dag,
		frontend: fe,
		llmModel: llmModel,
		mode:     modeShell, // reuse modeShell for "dang" mode
	}
}

// printDangValue prints a Dang value, formatting Dagger IDs (ScalarValue
// whose type name ends in "ID") using the human-readable dump format instead
// of the raw base64 encoding.
func printDangValue(w io.Writer, val dang.Value) {
	if sv, ok := val.(dang.ScalarValue); ok {
		typeName := sv.ScalarType.String()
		if strings.HasSuffix(typeName, "ID") {
			var idp call.ID
			if err := idp.Decode(sv.Val); err == nil {
				out := idtui.NewOutput(w)
				if err := new(idtui.Dump).DumpID(out, &idp); err == nil {
					return
				}
			}
		}
	}
	fmt.Fprintln(w, val.String())
}

// RunAll is the entry point for the dang shell command.
func (h *dangShellHandler) RunAll(ctx context.Context, args []string) error {
	h.tty = !silent && (hasTTY && progress == "auto" || progress == "tty")

	if err := h.Initialize(ctx); err != nil {
		return err
	}

	// Example: `dagger shell -c 'container.from("alpine")'`
	if shellCode != "" {
		return h.evalCode(ctx, shellCode)
	}

	// Use stdin only when no file paths are provided
	if len(args) == 0 {
		if isatty.IsTerminal(os.Stdin.Fd()) {
			return h.runInteractive(ctx)
		}
		// Pipe mode: read from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		return h.evalCode(ctx, string(data))
	}

	// File mode: run .dang files
	for _, path := range args {
		if err := dang.RunFile(ctx, path, h.debug); err != nil {
			return err
		}
	}

	return nil
}

// Initialize sets up the Dang type and eval environments, including
// loading the Dagger module if present.
func (h *dangShellHandler) Initialize(ctx context.Context) error {
	h.ctx = ctx

	// Set up service registry for service-based imports
	services := &dang.ServiceRegistry{}
	ctx = dang.ContextWithServices(ctx, services)

	// Find and resolve dang.toml imports
	cwd, _ := os.Getwd()
	var importConfigs []dang.ImportConfig
	configPath, config, err := dang.FindProjectConfig(cwd)
	if err != nil {
		slog.Debug("failed to find dang.toml", "error", err)
	} else if config != nil {
		configDir := filepath.Dir(configPath)
		ctx = dang.ContextWithProjectConfig(ctx, configPath, config)
		resolved, err := dang.ResolveImportConfigs(ctx, config, configDir)
		if err != nil {
			slog.Warn("failed to resolve imports from dang.toml", "error", err)
		} else {
			importConfigs = resolved
		}
	}

	// Load the Dagger module via the existing Dagger connection
	moduleDir := findDaggerModuleDir(cwd)
	if moduleDir != "" {
		provider := dang.NewGraphQLClientProvider(dang.GraphQLConfig{})
		client, schema, err := provider.ServeDaggerModule(ctx, h.dag, moduleDir)
		if err != nil {
			slog.Warn("failed to load Dagger module for Dang", "error", err)
		} else {
			importConfigs = append(importConfigs, dang.ImportConfig{
				Name:       "Dagger",
				Client:     client,
				Schema:     schema,
				AutoImport: true,
			})
		}
	}

	h.importConfigs = importConfigs
	h.typeEnv, h.evalEnv = dang.BuildEnvFromImports("Dagger", importConfigs)

	if len(importConfigs) > 0 {
		ctx = dang.ContextWithImportConfigs(ctx, importConfigs...)
	}
	h.ctx = ctx

	// Build completion infrastructure
	h.staticCompletions = replpkg.BuildStaticCompletions(h.typeEnv)
	h.completionProvider = replpkg.NewCompletionProvider(ctx, h.typeEnv, h.staticCompletions)
	return nil
}

// findDaggerModuleDir searches for a dagger.json starting from dir, walking up.
func findDaggerModuleDir(startPath string) string {
	dir, err := filepath.Abs(startPath)
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "dagger.json")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return "" // stop at repo boundary
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func (h *dangShellHandler) runInteractive(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	h.cancel = cancel

	// give ourselves a blank slate by zooming into a passthrough span
	ctx, shellSpan := Tracer().Start(ctx, "dang", telemetry.Passthrough())
	defer shellSpan.End()
	Frontend.SetPrimary(dagui.SpanID{SpanID: shellSpan.SpanContext().SpanID()})
	slog.SetDefault(slog.SpanLogger(ctx, InstrumentationLibrary))

	h.ctx = ctx

	// Start the shell loop
	Frontend.Shell(ctx, h)

	return nil
}

// evalCode evaluates a string of Dang code.
func (h *dangShellHandler) evalCode(ctx context.Context, code string) error {
	result, err := dang.ParseWithRecovery("eval", []byte(code))
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	forms := result.(*dang.ModuleBlock).Forms

	fresh := hm.NewSimpleFresher()
	_, err = dang.InferFormsWithPhases(ctx, forms, h.typeEnv, fresh)
	if err != nil {
		return fmt.Errorf("type error: %w", err)
	}

	var stdoutBuf bytes.Buffer
	evalCtx := ioctx.StdoutToContext(ctx, &stdoutBuf)
	evalCtx = ioctx.StderrToContext(evalCtx, &stdoutBuf)

	for _, node := range forms {
		val, err := dang.EvalNode(evalCtx, h.evalEnv, node)
		if err != nil {
			return fmt.Errorf("evaluation error: %w", err)
		}

		if stdoutBuf.Len() > 0 {
			stdio := telemetry.SpanStdio(ctx, InstrumentationLibrary)
			fmt.Fprint(stdio.Stdout, stdoutBuf.String())
			stdoutBuf.Reset()
		}

		// Print the result
		stdio := telemetry.SpanStdio(ctx, InstrumentationLibrary)
		printDangValue(stdio.Stdout, val)
	}

	return nil
}

var _ idtui.ShellHandler = (*dangShellHandler)(nil)

func (h *dangShellHandler) Handle(ctx context.Context, line string) (rerr error) {
	line = strings.TrimSpace(line)

	if line == "exit" || line == "/exit" || line == ":exit" || line == ":quit" {
		h.cancel()
		return nil
	}

	if line == "" {
		_, span := Tracer().Start(ctx, "",
			telemetry.Reveal(),
			trace.WithAttributes(attribute.Bool(telemetry.CanceledAttr, true)))
		span.End()
		return nil
	}

	// Handle LLM prompt mode
	if h.mode == modePrompt {
		llm, err := h.llm(ctx)
		if err != nil {
			return err
		}
		newLLM, err := llm.WithPrompt(ctx, line)
		if err != nil {
			return err
		}
		h.llmSession = newLLM
		h.llmModel = newLLM.model
		return nil
	}

	// Ensure we always see new telemetry
	if bag, err := baggage.Parse("repeat-telemetry=true"); err == nil {
		ctx = baggage.ContextWithBaggage(ctx, bag)
	}

	// Create a new span for this command
	var span trace.Span
	ctx, span = Tracer().Start(ctx, line,
		telemetry.Reveal(),
		trace.WithAttributes(
			attribute.String(telemetry.ContentTypeAttr, "text/x-dang"),
		))
	var telemetryErr error
	defer telemetry.EndWithCause(span, &telemetryErr)
	defer func() {
		if errors.Is(rerr, context.Canceled) {
			span.SetAttributes(attribute.Bool(telemetry.CanceledAttr, true))
		} else {
			telemetryErr = rerr
		}
	}()

	// redirect stdio to the current span
	stdio := telemetry.SpanStdio(ctx, InstrumentationLibrary)
	defer stdio.Close()

	// Parse
	result, err := dang.ParseWithRecovery("repl", []byte(line))
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	forms := result.(*dang.ModuleBlock).Forms

	// Type check
	fresh := hm.NewSimpleFresher()
	_, err = dang.InferFormsWithPhases(h.ctx, forms, h.typeEnv, fresh)
	if err != nil {
		return fmt.Errorf("type error: %w", err)
	}

	// Eval
	var stdoutBuf bytes.Buffer
	evalCtx := ioctx.StdoutToContext(ctx, &stdoutBuf)
	evalCtx = ioctx.StderrToContext(evalCtx, &stdoutBuf)

	for _, node := range forms {
		val, err := dang.EvalNode(evalCtx, h.evalEnv, node)

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if stdoutBuf.Len() > 0 {
			fmt.Fprint(stdio.Stdout, stdoutBuf.String())
			stdoutBuf.Reset()
		}

		if err != nil {
			return fmt.Errorf("evaluation error: %w", err)
		}

		printDangValue(stdio.Stdout, val)
	}

	return nil
}

func (h *dangShellHandler) Prompt(ctx context.Context, out idtui.TermOutput, fg termenv.Color) (string, func()) {
	sb := new(strings.Builder)
	sb.WriteString(termenv.CSI + termenv.ResetSeq + "m")

	// Purple color (63) for Dang prompt, matching the standalone REPL
	dangColor := termenv.ANSIMagenta

	var init func()

	switch h.mode {
	case modeShell: // "dang" mode
		sb.WriteString(out.String("dang").Bold().Foreground(dangColor).String())
		sb.WriteString(out.String("> ").Bold().Foreground(dangColor).String())
	case modePrompt:
		llm, err := h.llmMaybe()
		if err != nil {
			sb.WriteString(out.String("error").Bold().Foreground(termenv.ANSIRed).String())
			sb.WriteString(out.String(" ").String())
			fg = termenv.ANSIRed
		} else if llm != nil {
			sb.WriteString(out.String(llm.model).Bold().Foreground(termenv.ANSICyan).String())
			sb.WriteString(out.String(" ").String())
		} else {
			sb.WriteString(out.String("loading...").Bold().Foreground(termenv.ANSIYellow).String())
			sb.WriteString(out.String(" ").String())
			init = func() {
				h.llm(ctx) //nolint:errcheck
			}
		}
		sb.WriteString(out.String(idtui.LLMPrompt).Bold().Foreground(fg).String())
		sb.WriteString(out.String(out.String(" ").String()).String())
	}

	return sb.String(), init
}

func (h *dangShellHandler) AutoComplete(input string, cursorPos int) tuist.CompletionResult {
	if h.mode == modePrompt {
		return tuist.CompletionResult{}
	}
	if h.completionProvider == nil {
		return tuist.CompletionResult{}
	}
	return h.completionProvider(input, cursorPos)
}

func (h *dangShellHandler) IsComplete(input string) bool {
	if h.mode == modePrompt {
		return true
	}

	// Try to parse; if we get an error, check if it looks like
	// the input just needs more text (e.g., unclosed parens, brackets)
	_, err := dang.ParseWithRecovery("check", []byte(input))
	if err != nil {
		errStr := err.Error()
		// These patterns indicate incomplete input that needs more lines
		if strings.Contains(errStr, "unexpected end") ||
			strings.Contains(errStr, "unterminated") ||
			strings.Contains(errStr, "unclosed") ||
			strings.Contains(errStr, "expected") {
			return false
		}
	}
	return true
}

func (h *dangShellHandler) KeyBindings(out idtui.TermOutput) []key.Binding {
	autoCompactHelp := "auto-compact"
	if h.llmSession != nil && h.llmSession.ShouldAutocompact() {
		autoCompactHelp = out.String(autoCompactHelp).Foreground(termenv.ANSIGreen).String()
	}
	return []key.Binding{
		key.NewBinding(
			key.WithKeys(">"),
			key.WithHelp(">", "run prompt"),
			idtui.KeyEnabled(h.mode == modeShell),
		),
		key.NewBinding(
			key.WithKeys("<"),
			key.WithHelp("<", "run dang"),
			idtui.KeyEnabled(h.mode == modePrompt),
		),
		key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "upload changes"),
			idtui.KeyEnabled(h.mode == modePrompt),
		),
		key.NewBinding(
			key.WithKeys("ctrl+x"),
			key.WithHelp("ctrl+x", autoCompactHelp),
			idtui.KeyEnabled(h.llmSession != nil),
		),
	}
}

func (h *dangShellHandler) ReactToInput(ctx context.Context, ev uv.KeyPressEvent, inputValue string, editing bool) func() {
	key := uv.Key(ev)
	switch {
	case key.MatchString(">"):
		if inputValue == "" {
			h.mode = modePrompt
			return func() {
				h.llm(ctx) // initialize LLM
			}
		}
	case key.MatchString("<"):
		if inputValue == "" {
			h.mode = modeShell // back to dang
			return noop
		}
	case key.MatchString("ctrl+x"):
		if h.llmSession != nil {
			h.llmSession.ToggleAutocompact()
			return noop
		}
	case key.MatchString("ctrl+s"):
		if h.llmSession != nil {
			return func() {
				if err := h.llmSession.SyncToLocal(ctx); err != nil {
					slog.Error("failed to sync changes to local filesystem", "error", err.Error())
					Frontend.SetSidebarContent(idtui.SidebarSection{
						Title:   "Changes",
						Content: termenv.String("SAVE ERROR: " + err.Error()).Foreground(termenv.ANSIRed).String(),
					})
				}
			}
		}
	case key.MatchString("ctrl+u"):
		if h.llmSession != nil {
			return func() {
				if err := h.llmSession.SyncFromLocal(ctx); err != nil {
					slog.Error("failed to load current working directory into agent workspace", "error", err.Error())
					Frontend.SetSidebarContent(idtui.SidebarSection{
						Title:   "Changes",
						Content: termenv.String("UPLOAD ERROR: " + err.Error()).Foreground(termenv.ANSIRed).String(),
					})
				}
			}
		}
	}
	return nil
}

func (h *dangShellHandler) EncodeHistory(entry string) string {
	switch h.mode {
	case modePrompt:
		return ">" + entry
	case modeShell:
		return "d" + entry // "d" for dang
	}
	return entry
}

func (h *dangShellHandler) DecodeHistory(entry string) string {
	if len(entry) > 0 {
		switch entry[0] {
		case '>':
			h.mode = modePrompt
			return entry[1:]
		case 'd':
			h.mode = modeShell
			return entry[1:]
		case '!':
			// legacy shell entry - display as-is
			h.mode = modeShell
			return entry[1:]
		default:
			h.mode = modeUnset
		}
	}
	return entry
}

func (h *dangShellHandler) SaveBeforeHistory() {
	h.savedMode = h.mode
}

func (h *dangShellHandler) RestoreAfterHistory() {
	h.mode = h.savedMode
	h.savedMode = modeUnset
}


// LLM support methods (shared pattern with shellCallHandler)

func (h *dangShellHandler) llmMaybe() (*LLMSession, error) {
	h.llmL.Lock()
	defer h.llmL.Unlock()
	return h.llmSession, h.llmErr
}

func (h *dangShellHandler) llm(ctx context.Context) (*LLMSession, error) {
	if s, e := h.llmMaybe(); s != nil || e != nil {
		return s, e
	}

	s, err := NewLLMSession(ctx, h.dag, h.llmModel, h, h.frontend)

	h.llmL.Lock()
	defer h.llmL.Unlock()

	if err != nil {
		slog.Error("failed to initialize LLM", "error", err)
		h.llmErr = err
		return nil, err
	}
	h.llmSession = s
	h.llmModel = s.model
	return h.llmSession, h.llmErr
}
