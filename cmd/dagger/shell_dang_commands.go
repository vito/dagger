package main

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/vito/dang/pkg/dang"
	"github.com/vito/dang/pkg/hm"
	replpkg "github.com/vito/dang/pkg/repl"
	"github.com/vito/tuist"
)

// dangCommandDef defines a REPL command.
type dangCommandDef struct {
	name    string
	aliases []string
	desc    string
	handler func(h *dangShellHandler, w io.Writer, args []string)
}

func (h *dangShellHandler) buildCommandDefs() []dangCommandDef {
	return []dangCommandDef{
		{
			name: "help",
			desc: "Show available commands",
			handler: func(h *dangShellHandler, w io.Writer, _ []string) {
				fmt.Fprintln(w, "Available commands:")
				maxName := 0
				for _, cmd := range h.commands {
					if len(cmd.name) > maxName {
						maxName = len(cmd.name)
					}
				}
				for _, cmd := range h.commands {
					fmt.Fprintf(w, "  :%-*s - %s\n", maxName, cmd.name, cmd.desc)
				}
				fmt.Fprintln(w)
				fmt.Fprintln(w, "Type Dang expressions to evaluate them.")
			},
		},
		{
			name:    "exit",
			aliases: []string{"quit"},
			desc:    "Exit the REPL",
			handler: func(h *dangShellHandler, _ io.Writer, _ []string) {
				h.cancel()
			},
		},
		{
			name: "doc",
			desc: "Interactive API browser",
			handler: func(h *dangShellHandler, _ io.Writer, _ []string) {
				h.showDocBrowser()
			},
		},
		{
			name: "env",
			desc: "Show environment bindings",
			handler: func(h *dangShellHandler, w io.Writer, args []string) {
				h.envCommand(w, args)
			},
		},
		{
			name: "type",
			desc: "Show type of an expression",
			handler: func(h *dangShellHandler, w io.Writer, args []string) {
				h.typeCommand(w, args)
			},
		},
		{
			name:    "find",
			aliases: []string{"search"},
			desc:    "Find functions/types by pattern",
			handler: func(h *dangShellHandler, w io.Writer, args []string) {
				h.findCommand(w, args)
			},
		},
		{
			name: "reset",
			desc: "Reset the environment",
			handler: func(h *dangShellHandler, w io.Writer, _ []string) {
				h.typeEnv, h.evalEnv = dang.BuildEnvFromImports("Dagger", h.importConfigs)
				h.staticCompletions = replpkg.BuildStaticCompletions(h.typeEnv)
				h.completionProvider = replpkg.NewCompletionProvider(h.ctx, h.typeEnv, h.staticCompletions)
				fmt.Fprintln(w, "Environment reset.")
			},
		},
		{
			name: "debug",
			desc: "Toggle debug mode",
			handler: func(h *dangShellHandler, w io.Writer, _ []string) {
				h.debug = !h.debug
				status := "disabled"
				if h.debug {
					status = "enabled"
				}
				fmt.Fprintf(w, "Debug mode %s.\n", status)
			},
		},
		{
			name: "version",
			desc: "Show version info",
			handler: func(h *dangShellHandler, w io.Writer, _ []string) {
				fmt.Fprintln(w, "Dagger + Dang shell")
				if len(h.importConfigs) > 0 {
					var names []string
					for _, ic := range h.importConfigs {
						names = append(names, ic.Name)
					}
					fmt.Fprintf(w, "Imports: %s\n", strings.Join(names, ", "))
				} else {
					fmt.Fprintln(w, "No imports configured")
				}
			},
		},
	}
}

// handleCommand dispatches a colon-prefixed command. Returns true if the
// input was a command (even if unknown), false if it should be evaluated
// as dang code.
func (h *dangShellHandler) handleCommand(w io.Writer, line string) bool {
	if !strings.HasPrefix(line, ":") {
		return false
	}

	parts := strings.Fields(line[1:]) // strip leading ':'
	if len(parts) == 0 {
		fmt.Fprintln(w, "empty command (type :help for available commands)")
		return true
	}

	cmd := parts[0]
	args := parts[1:]

	for _, def := range h.commands {
		if def.name == cmd || slices.Contains(def.aliases, cmd) {
			def.handler(h, w, args)
			return true
		}
	}

	fmt.Fprintf(w, "unknown command: %s (type :help for available commands)\n", cmd)
	return true
}

// commandCompletions returns completions for colon-prefixed commands.
func (h *dangShellHandler) commandCompletions(input string) tuist.CompletionResult {
	partial := strings.TrimPrefix(input, ":")
	partialLower := strings.ToLower(partial)

	var items []tuist.Completion
	for _, cmd := range h.commands {
		if strings.HasPrefix(cmd.name, partialLower) && cmd.name != partialLower {
			items = append(items, tuist.Completion{
				Label:         ":" + cmd.name,
				Detail:        "command",
				Documentation: cmd.desc,
				Kind:          "command",
			})
		}
	}
	return tuist.CompletionResult{
		Items:       items,
		ReplaceFrom: 0,
	}
}

// showDocBrowser opens the interactive API documentation browser, temporarily
// removing the normal TUI children and replacing them with the browser.
// Blocks until the user dismisses the browser, preventing handleShellDone
// from stealing focus back.
func (h *dangShellHandler) showDocBrowser() {
	if h.tui == nil {
		return
	}

	done := make(chan struct{})

	h.tui.Dispatch(func() {
		// Remember current focus and children
		prevFocus := h.tui.Focused()
		prevChildren := make([]tuist.Component, len(h.tui.Children))
		copy(prevChildren, h.tui.Children)

		// Remove existing children (don't dismount — we'll restore them)
		for _, ch := range prevChildren {
			h.tui.RemoveChild(ch)
		}

		db := replpkg.NewDocBrowserOverlay(h.typeEnv)
		db.OnExit = func() {
			h.tui.RemoveChild(db)
			// Restore previous children and focus
			for _, ch := range prevChildren {
				h.tui.AddChild(ch)
			}
			if prevFocus != nil {
				h.tui.SetFocus(prevFocus)
			}
			close(done)
		}
		h.tui.AddChild(db)
		h.tui.SetFocus(db)
	})

	// Block until the doc browser is dismissed, so handleShellDone
	// doesn't steal focus back.
	<-done
}

// envCommand lists environment bindings.
func (h *dangShellHandler) envCommand(w io.Writer, args []string) {
	filter := ""
	showAll := false
	if len(args) > 0 {
		if args[0] == "all" {
			showAll = true
		} else {
			filter = args[0]
		}
	}
	fmt.Fprintln(w, "Current environment bindings:")
	fmt.Fprintln(w)
	count := 0
	for name, scheme := range h.typeEnv.Bindings(dang.PublicVisibility) {
		if filter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
			continue
		}
		if !showAll && count >= 20 {
			fmt.Fprintln(w, "  ... use ':env all' to see all")
			break
		}
		t, _ := scheme.Type()
		if t != nil {
			fmt.Fprintf(w, "  %s : %s\n", name, t)
		} else {
			fmt.Fprintf(w, "  %s\n", name)
		}
		count++
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Use ':doc' for interactive API browsing")
}

// typeCommand shows the inferred type of an expression.
func (h *dangShellHandler) typeCommand(w io.Writer, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(w, "Usage: :type <expression>")
		return
	}
	expr := strings.Join(args, " ")
	result, err := dang.ParseWithRecovery("type-check", []byte(expr))
	if err != nil {
		fmt.Fprintf(w, "parse error: %v\n", err)
		return
	}
	node, ok := result.(hm.Expression)
	if !ok {
		fmt.Fprintln(w, "unexpected parse result")
		return
	}
	inferredType, err := dang.Infer(h.ctx, h.typeEnv, node, false)
	if err != nil {
		fmt.Fprintf(w, "type error: %v\n", err)
		return
	}
	fmt.Fprintf(w, "Expression: %s\n", expr)
	fmt.Fprintf(w, "Type: %s\n", inferredType)
	trimmed := strings.TrimSpace(expr)
	if !strings.Contains(trimmed, " ") {
		if scheme, found := h.typeEnv.SchemeOf(trimmed); found {
			if t, _ := scheme.Type(); t != nil {
				fmt.Fprintf(w, "Scheme: %s\n", scheme)
			}
		}
	}
}

// findCommand searches for functions/types matching a pattern.
func (h *dangShellHandler) findCommand(w io.Writer, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(w, "Usage: :find <pattern>")
		return
	}
	pattern := strings.ToLower(args[0])
	fmt.Fprintf(w, "Searching for '%s'...\n", pattern)
	found := false
	for name, scheme := range h.typeEnv.Bindings(dang.PublicVisibility) {
		if strings.Contains(strings.ToLower(name), pattern) {
			t, _ := scheme.Type()
			if t != nil {
				fmt.Fprintf(w, "  %s : %s\n", name, t)
			} else {
				fmt.Fprintf(w, "  %s\n", name)
			}
			found = true
		}
	}
	for name, env := range h.typeEnv.NamedTypes() {
		if strings.Contains(strings.ToLower(name), pattern) {
			doc := env.GetModuleDocString()
			if doc != "" {
				if len(doc) > 60 {
					doc = doc[:57] + "..."
				}
				fmt.Fprintf(w, "  %s - %s\n", name, doc)
			} else {
				fmt.Fprintf(w, "  %s\n", name)
			}
			found = true
		}
	}
	if !found {
		fmt.Fprintf(w, "No matches found for '%s'\n", pattern)
	}
}
