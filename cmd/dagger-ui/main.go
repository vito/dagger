package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime/pprof"
	"runtime/trace"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dagger/dagger/internal/engine/journal"
)

var journalFile string

func init() {
	flag.StringVar(&journalFile, "journal", os.Getenv("_EXPERIMENTAL_DAGGER_JOURNAL"), "replay journal file")
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()

	if err := run(flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd []string) error {
	if *cpuprofile != "" {
		profF, err := os.Create(*cpuprofile)
		if err != nil {
			return fmt.Errorf("create profile: %w", err)
		}
		pprof.StartCPUProfile(profF)
		defer pprof.StopCPUProfile()

		tracePath := *cpuprofile + ".trace"
		traceF, err := os.Create(tracePath)
		if err != nil {
			return fmt.Errorf("create trace: %w", err)
		}
		defer traceF.Close()

		if err := trace.Start(traceF); err != nil {
			return fmt.Errorf("start trace: %w", err)
		}
		defer trace.Stop()
	}

	if len(cmd) == 0 && journalFile == "" {
		return fmt.Errorf("usage: %s ([cmd...] | --journal <file>)", os.Args[0])
	}

	var r journal.Reader
	var err error
	if journalFile != "" {
		r, err = tailJournal(journalFile, true, nil)
		if err != nil {
			return fmt.Errorf("tail: %w", err)
		}
	} else {
		sink, err := journal.ServeWriters("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		defer sink.Close()

		r = sink

		journalFile = sink.Endpoint()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var rootName string
	var rootLogs groupModel
	if len(cmd) > 0 {
		rootName = strings.Join(cmd, " ")

		vt := NewVterm(80)
		rootLogs = vt

		cmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...) // nolint:gosec
		cmd.Env = append(os.Environ(), "_EXPERIMENTAL_DAGGER_JOURNAL="+journalFile)
		cmd.Stdout = vt
		cmd.Stderr = vt

		// NB: go run lets its child process roam free when you interrupt it, so
		// make sure they all get signalled. (you don't normally notice this in a
		// shell because Ctrl+C sends to the process group.)
		ensureChildProcessesAreKilled(cmd)

		err := cmd.Start()
		if err != nil {
			return fmt.Errorf("start command: %w", err)
		}

		defer cmd.Wait()
	} else {
		rootLogs = &emptyGroup{}
	}

	model := New(cancel, r, rootName, rootLogs)

	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run UI: %w", err)
	}

	return nil
}

type syncBuffer struct {
}
