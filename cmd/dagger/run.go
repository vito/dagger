package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dagger/dagger/internal/engine/journal"
	"github.com/dagger/dagger/internal/tui"
	"github.com/dagger/dagger/router"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:                   "run [command]",
	Aliases:               []string{"r"},
	DisableFlagsInUseLine: true,
	Long:                  "Runs the specified command in a Dagger session\n\nDAGGER_SESSION_PORT and DAGGER_SESSION_TOKEN will be convieniently injected automatically.",
	Short:                 "Runs a command in a Dagger session",
	Example: `
dagger run -- sh -c 'curl \
-u $DAGGER_SESSION_TOKEN: \
-H "content-type:application/json" \
-d "{\"query\":\"{container{id}}\"}" http://127.0.0.1:$DAGGER_SESSION_PORT/query'`,
	Run:          Run,
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
}

func init() {
	runCmd.Flags().SetInterspersed(false)
}

func Run(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	err := run(ctx, args)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "run canceled")
			os.Exit(2)
			return
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
		return
	}
}

func run(ctx context.Context, args []string) error {
	sessionToken, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("generate uuid: %w", err)
	}

	sessionL, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("session listen: %w", err)
	}
	defer sessionL.Close()

	journalR, journalW := journal.Pipe()

	sessionPort := fmt.Sprintf("%d", sessionL.Addr().(*net.TCPAddr).Port)
	os.Setenv("DAGGER_SESSION_PORT", sessionPort)
	os.Setenv("DAGGER_SESSION_TOKEN", sessionToken.String())

	ctx, quit := context.WithCancel(ctx)
	defer quit()

	subCmd := exec.CommandContext(ctx, args[0], args[1:]...) // #nosec

	// NB: go run lets its child process roam free when you interrupt it, so
	// make sure they all get signalled. (you don't normally notice this in a
	// shell because Ctrl+C sends to the process group.)
	ensureChildProcessesAreKilled(subCmd)

	cmdline := strings.Join(args, " ")
	model := tui.New(quit, journalR, cmdline)
	program := tea.NewProgram(model, tea.WithAltScreen())
	subCmd.Stdout = progOutWriter{program}
	subCmd.Stderr = progOutWriter{program}

	return withEngine(ctx, sessionToken.String(), journalW, progOutWriter{program}, func(ctx context.Context, api *router.Router) error {
		go http.Serve(sessionL, api) // nolint:gosec

		err := subCmd.Start()
		if err != nil {
			return err
		}

		go func() {
			program.Send(tui.CommandExitMsg(subCmd.Wait()))
		}()

		_, err = program.Run()
		return err
	})
}

type progOutWriter struct {
	prog *tea.Program
}

func (w progOutWriter) Write(p []byte) (int, error) {
	w.prog.Send(tui.CommandOutMsg(p))
	return len(p), nil
}
