package main

import (
	"strings"

	"dagger.io/dagger"
)

func main() {
	dagger.DefaultContext().Client().Environment().
		WithFunction_(Build).
		Serve()
}

type GoBuildOpts struct {
	Packages []string

	// Optional subdirectory in which to place the built
	// artifacts.
	Subdir string

	// -X definitions to pass to go build -ldflags.
	Xdefs []string

	// Whether to enable CGO.
	Static bool

	// Whether to build with race detection.
	Race bool

	// Cross-compile via GOOS and GOARCH.
	GOOS, GOARCH string

	// Arbitrary flags to pass along to go build.
	BuildFlags []string
}

func Build(
	ctx dagger.Context,
	base *dagger.Container,
	src *dagger.Directory,

	// Packages to build.
	packages *[]string,

	// Optional subdirectory in which to place the built
	// artifacts.
	subdir *string,

	// -X definitions to pass to go build -ldflags.
	xdefs *[]string,

	// Whether to enable CGO.
	static *bool,

	// Whether to build with race detection.
	race *bool,

	// Cross-compile via GOOS and GOARCH.
	goos, goarch *string,

	// Arbitrary flags to pass along to go build.
	buildFlags *[]string,
) *dagger.Directory {
	ctr := base.
		With(GlobalCache(ctx)).
		WithDirectory("/out", ctx.Client().Directory()).
		With(Cd("/src", src))

	if static != nil && *static {
		ctr = ctr.WithEnvVariable("CGO_ENABLED", "0")
	}

	if goos != nil {
		ctr = ctr.WithEnvVariable("GOOS", *goos)
	}

	if goarch != nil {
		ctr = ctr.WithEnvVariable("GOARCH", *goarch)
	}

	cmd := []string{
		"go", "build",
		"-o", "/out/",
		"-trimpath", // unconditional for reproducible builds
	}

	if race != nil && *race {
		cmd = append(cmd, "-race")
	}

	if buildFlags != nil {
		cmd = append(cmd, *buildFlags...)
	}

	if xdefs != nil {
		cmd = append(cmd, "-ldflags", "-X "+strings.Join(*xdefs, " -X "))
	}

	if packages != nil {
		cmd = append(cmd, *packages...)
	}

	out := ctr.
		WithExec(cmd).
		Directory("/out")

	if subdir != nil {
		out = ctx.Client().
			Directory().
			WithDirectory(*subdir, out)
	}

	return out
}

// type GoTestOpts struct {
// 	Packages  []string
// 	Race      bool
// 	Verbose   bool
// 	TestFlags []string
// }

// func Test(
// 	ctx dagger.Context,
// 	base *dagger.Container,
// 	src *dagger.Directory,
// 	opts_ ...GoTestOpts,
// ) *dagger.Container {
// 	var opts GoTestOpts
// 	if len(opts_) > 0 {
// 		opts = opts_[0]
// 	}
// 	cmd := []string{"go", "test"}
// 	if opts.Race {
// 		cmd = append(cmd, "-race")
// 	}
// 	if opts.Verbose {
// 		cmd = append(cmd, "-v")
// 	}
// 	cmd = append(cmd, opts.TestFlags...)
// 	if len(opts.Packages) > 0 {
// 		cmd = append(cmd, opts.Packages...)
// 	} else {
// 		cmd = append(cmd, "./...")
// 	}
// 	return base.
// 		With(GlobalCache(ctx)).
// 		WithMountedDirectory("/src", src).
// 		WithWorkdir("/src").
// 		WithFocus().
// 		WithExec(cmd).
// 		WithoutFocus()
// }

// type GotestsumOpts struct {
// 	Packages       []string
// 	Format         string
// 	Race           bool
// 	GoTestFlags    []string
// 	GotestsumFlags []string
// }

// func Gotestsum(
// 	ctx dagger.Context,
// 	base *dagger.Container,
// 	src *dagger.Directory,
// 	opts_ ...GotestsumOpts,
// ) *dagger.Container {
// 	var opts GotestsumOpts
// 	if len(opts_) > 0 {
// 		opts = opts_[0]
// 	}
// 	if opts.Format == "" {
// 		opts.Format = "testname"
// 	}
// 	cmd := []string{
// 		"gotestsum",
// 		"--no-color=false", // force color
// 		"--format=" + opts.Format,
// 	}
// 	cmd = append(cmd, opts.GotestsumFlags...)
// 	cmd = append(cmd, opts.GoTestFlags...)
// 	goTestFlags := []string{}
// 	if opts.Race {
// 		goTestFlags = append(goTestFlags, "-race")
// 	}
// 	if len(opts.Packages) > 0 {
// 		goTestFlags = append(goTestFlags, opts.Packages...)
// 	}
// 	if len(goTestFlags) > 0 {
// 		cmd = append(cmd, "--")
// 		cmd = append(cmd, goTestFlags...)
// 	}
// 	return base.
// 		With(GlobalCache(ctx)).
// 		WithMountedDirectory("/src", src).
// 		WithWorkdir("/src").
// 		WithFocus().
// 		WithExec(cmd).
// 		WithoutFocus()
// }

// func Generate(
// 	ctx dagger.Context,
// 	base *dagger.Container,
// 	src *dagger.Directory,
// ) *dagger.Directory {
// 	return base.
// 		With(GlobalCache(ctx)).
// 		With(Cd("/src", src)).
// 		WithFocus().
// 		WithExec([]string{"go", "generate", "./..."}).
// 		WithoutFocus().
// 		Directory("/src")
// }

// type GolangCILintOpts struct {
// 	Verbose bool
// 	Timeout int
// }

// func GolangCILint(
// 	ctx dagger.Context,
// 	base *dagger.Container,
// 	src *dagger.Directory,
// 	opts_ ...GolangCILintOpts,
// ) *dagger.Container {
// 	var opts GolangCILintOpts
// 	if len(opts_) > 0 {
// 		opts = opts_[0]
// 	}
// 	cmd := []string{"golangci-lint", "run"}
// 	if opts.Verbose {
// 		cmd = append(cmd, "--verbose")
// 	}
// 	if opts.Timeout > 0 {
// 		cmd = append(cmd, fmt.Sprintf("--timeout=%ds", opts.Timeout))
// 	}
// 	return base.
// 		With(GlobalCache(ctx)).
// 		WithMountedDirectory("/src", src).
// 		WithWorkdir("/src").
// 		WithFocus().
// 		WithExec(cmd).
// 		WithoutFocus()
// }
