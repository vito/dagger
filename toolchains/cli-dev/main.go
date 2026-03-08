// Develop the Dagger CLI
package main

import (
	"context"
	"fmt"
	"path"
	"runtime"
	"strings"

	"dagger/cli-dev/internal/dagger"

	"github.com/containerd/platforms"
)

func New(
	ctx context.Context,

	// +optional
	runnerHost string,

	// +optional
	// +defaultPath="/"
	// +ignore=[
	//   "*",
	//   ".*",
	//   "!cmd/dagger/*",
	//   "!**/go.sum",
	//   "!**/go.mod",
	//   "!**/*.go",
	//   "!vendor/**/*",
	//   "!**.graphql",
	//   "!.goreleaser*.yml",
	//   "!.changes",
	//   "!LICENSE",
	//   "!install.sh",
	//   "!install.ps1",
	//   "!**/*.sql"
	// ]
	source *dagger.Directory,

	// Base image for go build environment
	// +optional
	base *dagger.Container,

	// Explicit version to set on the Dagger CLI.
	// +optional
	version string,
) (*CliDev, error) {
	// FIXME: this go builder config is duplicated with engine build
	// move into a shared engine/builder module
	v := dag.Version()

	var err error
	if version == "" {
		version, err = v.Version(ctx)
		if err != nil {
			return nil, err
		}
	}

	imageTag, err := v.ImageTag(ctx)
	if err != nil {
		return nil, err
	}
	values := []string{
		// FIXME: how to avoid duplication with engine module?
		"github.com/dagger/dagger/engine.Version=" + version,
		"github.com/dagger/dagger/engine.Tag=" + imageTag,
	}
	if runnerHost != "" {
		values = append(values, "main.RunnerHost="+runnerHost)
	}

	return &CliDev{
		Version: version,
		Tag:     version,
		Go: dag.Go(dagger.GoOpts{
			Source: source,
			Base:   base,
			Values: values,
			// Enable CGo with zig as the C toolchain for static builds
			Cgo:           true,
			ExtraPackages: []string{"zig"},
			Ldflags:       []string{"-linkmode", "external", `-extldflags "-static"`},
		}),
	}, nil
}

type CliDev struct {
	Version string
	Tag     string

	Go *dagger.Go // +private
}

// zigTarget returns the zig target triple for the given platform,
// defaulting to the current platform if empty.
func zigTarget(platform dagger.Platform) string {
	goarch := runtime.GOARCH
	goos := "linux"
	if platform != "" {
		p := platforms.MustParse(string(platform))
		goarch = p.Architecture
		goos = p.OS
	}
	zigArch := goarch
	switch zigArch {
	case "arm64":
		zigArch = "aarch64"
	case "amd64":
		zigArch = "x86_64"
	case "arm":
		zigArch = "arm"
	}
	switch goos {
	case "linux":
		return zigArch + "-linux-musl"
	case "darwin":
		return zigArch + "-macos"
	case "windows":
		return zigArch + "-windows"
	default:
		return zigArch + "-linux-musl"
	}
}

// useZig returns whether zig should be used as the C compiler for the given platform.
// Zig is used for Linux static builds with musl; other platforms use the default toolchain.
func useZig(platform dagger.Platform) bool {
	if platform == "" {
		return true // default platform is linux (container runtime)
	}
	p := platforms.MustParse(string(platform))
	return p.OS == "linux"
}

// Build the dagger CLI binary for a single platform
func (cli CliDev) Binary(
	ctx context.Context,
	// +optional
	platform dagger.Platform,
) (*dagger.File, error) {
	env := cli.Go.Env(dagger.GoEnvOpts{Platform: platform})

	if useZig(platform) {
		target := zigTarget(platform)
		env = env.
			WithEnvVariable("CC", fmt.Sprintf("zig cc -target %s", target)).
			WithEnvVariable("CXX", fmt.Sprintf("zig c++ -target %s", target))
	}

	// Retrieve values and ldflags to build the go command
	values, err := cli.Go.Values(ctx)
	if err != nil {
		return nil, err
	}
	ldflags, err := cli.Go.Ldflags(ctx)
	if err != nil {
		return nil, err
	}
	// Strip symbols and DWARF info
	ldflags = append(ldflags, "-s", "-w")
	// Add -X flags for values
	for _, val := range values {
		ldflags = append(ldflags, "-X '"+val+"'")
	}

	pkg := "./cmd/dagger"
	cmd := []string{
		"go", "build",
		"-o", "./bin/",
		"-ldflags", strings.Join(ldflags, " "),
		pkg,
	}

	env = env.WithExec(cmd)

	files, err := env.Directory("./bin").Glob(ctx, path.Base(pkg)+"*")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no matching binary found")
	}
	return env.Directory("./bin").File(files[0]), nil
}

// Generate a markdown CLI reference doc
func (cli CliDev) Reference(
	// +optional
	frontmatter string,
	// +optional
	// Include experimental commands
	includeExperimental bool,
) *dagger.File {
	cmd := []string{"go", "run", "./cmd/dagger", "gen", "--output", "cli.mdx"}
	if includeExperimental {
		cmd = append(cmd, "--include-experimental")
	}
	if frontmatter != "" {
		cmd = append(cmd, "--frontmatter="+frontmatter)
	}
	return cli.Go.
		Env().
		WithExec(cmd).
		File("cli.mdx")
}

// Build dev CLI binaries
// TODO: remove this
func (cli *CliDev) DevBinaries(
	ctx context.Context,
	// +optional
	platform dagger.Platform,
) (*dagger.Directory, error) {
	p := platforms.MustParse(string(platform))
	bin, err := cli.Binary(ctx, platform)
	if err != nil {
		return nil, err
	}
	binName := "dagger"
	if p.OS == "windows" {
		binName += ".exe"
	}
	dir := dag.Directory().WithFile(binName, bin)
	if p.OS != "linux" {
		p2 := p
		p2.OS = "linux"
		p2.OSFeatures = nil
		p2.OSVersion = ""
		linuxBin, err := cli.Binary(ctx, dagger.Platform(platforms.Format(p2)))
		if err != nil {
			return nil, err
		}
		dir = dir.WithFile("dagger-linux", linuxBin)
	}
	return dir, nil
}
