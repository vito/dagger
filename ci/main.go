package main

import (
	"context"
	"fmt"

	"dagger/ci/internal/dagger"

	"github.com/sourcegraph/conc/pool"
)

type CI struct{}

func (ci *CI) Build(ctx context.Context) (*Built, error) {
	// TODO: syncing kind of makes sense intuitively - we don't want this function
	// to return a Built unless it, you know, builds.
	//
	// HOWEVER, it would be neat if this could be simplified to pure form, and
	// still have all the benefits, i.e. Build considered 'failed' because its
	// effects failed
	cli, err := dag.DaggerCli().Binary().Sync(ctx)
	if err != nil {
		return nil, err
	}
	engine, err := dag.DaggerEngine().Container().Sync(ctx)
	if err != nil {
		return nil, err
	}
	return &Built{
		CLI:    cli,
		Engine: engine,
	}, nil
}

type Built struct {
	CLI    *dagger.File
	Engine *dagger.Container
}

func (ci *CI) Scan(ctx context.Context) (*Scanned, error) {
	if err := dag.DaggerDev().Scan(ctx); err != nil {
		return nil, err
	}
	return &Scanned{}, nil
}

type Scanned struct{}

func (ci *CI) Test(ctx context.Context) (*Tested, error) {
	baseOpts := dagger.DaggerDevTestSpecificOpts{
		Race:     true,
		Parallel: 16,
	}
	eg := pool.New().WithErrors()
	for _, run := range []string{
		"TestCall|TestShell|TestDaggerCMD",
		"TestProvision|TestTelemetry",
		"TestCLI|TestEngine",
		"TestClientGenerator",
		"TestContainer|TestDockerfile",
		"TestInterface",
		"TestLLM",
		"TestGo|TestPython|TestTypescript|TestElixir|TestPHP|TestJava",
		"TestModule",
	} {
		opts := baseOpts
		opts.Run = run
		// TOOD: this is gonna melt whatever machine it lands on. use PARC!
		// somehow...
		eg.Go(func() error {
			return dag.DaggerDev().Test().Specific(ctx, opts)
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &Tested{}, nil
}

type Tested struct{}

func (ci *CI) Release(
	ctx context.Context,
	built *Built,
	scanned *Scanned,
	tested *Tested,
	version string, // must be provided by a human?
) error {
	fmt.Println("tagging and stuff")
	return nil
}
