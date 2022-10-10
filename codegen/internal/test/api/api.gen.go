package api

import (
	"context"

	"github.com/Khan/genqlient/graphql"
	"go.dagger.io/dagger/sdk/go/dagger/querybuilder"
)

// New returns a new API query object
func New(c graphql.Client) *Query {
	return &Query{
		q: querybuilder.Query(),
		c: c,
	}
}

// The address (also known as "ref") of a container published as an OCI image.
//
// Examples:
//   - "alpine"
//   - "index.docker.io/alpine"
//   - "index.docker.io/alpine:latest"
//   - "index.docker.io/alpine:latest@sha256deadbeefdeadbeefdeadbeef"
type ContainerAddress string

// A unique container identifier. Null designates an empty container (scratch).
type ContainerID string

// A content-addressed directory identifier
type DirectoryID string

type FileID string

type SecretID string

// Additional options for executing a command
type ExecOpts struct {
	// Optionally redirect the command's standard error to a file in the container.
	// Null means discard output.
	RedirectStderr string `json:"redirectStderr"`

	// Optionally redirect the command's standard output to a file in the container.
	// Null means discard output.
	RedirectStdout string `json:"redirectStdout"`

	// Optionally write to the command's standard input
	//
	// - Null means don't touch stdin (no redirection)
	// - Empty string means inject zero bytes to stdin, then send EOF
	Stdin string `json:"stdin"`
}

// An OCI-compatible container, also known as a docker container
type Container struct {
	q *querybuilder.Selection
	c graphql.Client
}

// Retrieve a directory at the given path. Mounts are included.
func (r *Container) Directory(path string) *Directory {
	q := r.q.Select("directory")
	q = q.Arg("path", path)

	return &Directory{
		q: q,
		c: r.c,
	}
}

// Entrypoint to be prepended to the arguments of all commands
func (r *Container) Entrypoint(ctx context.Context) ([]string, error) {
	q := r.q.Select("entrypoint")

	var response []string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// ContainerExecOptions contains options for Container.Exec
type ContainerExecOptions struct {
	Opts *ExecOpts
}

// ContainerExecOption represents an option handler for Container.Exec
type ContainerExecOption func(*ContainerExecOptions)

// WithContainerExecOpts sets the "opts" option for Exec
func WithContainerExecOpts(opts ExecOpts) ContainerExecOption {
	return func(daggerOptions *ContainerExecOptions) {
		daggerOptions.Opts = &opts
	}
}

// This container after executing the specified command inside it
func (r *Container) Exec(args []string, options ...ContainerExecOption) *Container {
	opts := &ContainerExecOptions{}
	for _, fn := range options {
		fn(opts)
	}
	q := r.q.Select("exec")
	q = q.Arg("args", args)
	if opts != nil && opts.Opts != nil {
		q = q.Arg("opts", opts.Opts)
	}

	return &Container{
		q: q,
		c: r.c,
	}
}

// Exit code of the last executed command. Zero means success.
// Null if no command has been executed.
func (r *Container) ExitCode(ctx context.Context) (int, error) {
	q := r.q.Select("exitCode")

	var response int
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// Initialize this container from the base image published at the given address
func (r *Container) From(address ContainerAddress) *Container {
	q := r.q.Select("from")
	q = q.Arg("address", address)

	return &Container{
		q: q,
		c: r.c,
	}
}

// A unique identifier for this container
func (r *Container) ID(ctx context.Context) (ContainerID, error) {
	q := r.q.Select("id")

	var response ContainerID
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// List of paths where a directory is mounted
func (r *Container) Mounts(ctx context.Context) ([]string, error) {
	q := r.q.Select("mounts")

	var response []string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// Publish this container as a new image
func (r *Container) Publish(ctx context.Context, address ContainerAddress) (ContainerAddress, error) {
	q := r.q.Select("publish")
	q = q.Arg("address", address)

	var response ContainerAddress
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// This container's root filesystem. Mounts are not included.
func (r *Container) Rootfs() *Directory {
	q := r.q.Select("rootfs")

	return &Directory{
		q: q,
		c: r.c,
	}
}

// The error stream of the last executed command.
// Null if no command has been executed.
func (r *Container) Stderr() *File {
	q := r.q.Select("stderr")

	return &File{
		q: q,
		c: r.c,
	}
}

// The output stream of the last executed command.
// Null if no command has been executed.
func (r *Container) Stdout() *File {
	q := r.q.Select("stdout")

	return &File{
		q: q,
		c: r.c,
	}
}

// The user to be set for all commands
func (r *Container) User(ctx context.Context) (string, error) {
	q := r.q.Select("user")

	var response string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// The value of the specified environment variable
func (r *Container) Variable(ctx context.Context, name string) (string, error) {
	q := r.q.Select("variable")
	q = q.Arg("name", name)

	var response string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// A list of environment variables passed to commands
func (r *Container) Variables(ctx context.Context) ([]string, error) {
	q := r.q.Select("variables")

	var response []string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// This container but with a different command entrypoint
func (r *Container) WithEntrypoint(args []string) *Container {
	q := r.q.Select("withEntrypoint")
	q = q.Arg("args", args)

	return &Container{
		q: q,
		c: r.c,
	}
}

// ContainerWithMountedCacheOptions contains options for Container.WithMountedCache
type ContainerWithMountedCacheOptions struct {
	Source *DirectoryID
}

// ContainerWithMountedCacheOption represents an option handler for Container.WithMountedCache
type ContainerWithMountedCacheOption func(*ContainerWithMountedCacheOptions)

// WithContainerWithMountedCacheSource sets the "source" option for WithMountedCache
func WithContainerWithMountedCacheSource(source DirectoryID) ContainerWithMountedCacheOption {
	return func(daggerOptions *ContainerWithMountedCacheOptions) {
		daggerOptions.Source = &source
	}
}

// This container plus a cache directory mounted at the given path
func (r *Container) WithMountedCache(path string, options ...ContainerWithMountedCacheOption) *Container {
	opts := &ContainerWithMountedCacheOptions{}
	for _, fn := range options {
		fn(opts)
	}
	q := r.q.Select("withMountedCache")
	q = q.Arg("path", path)
	if opts != nil && opts.Source != nil {
		q = q.Arg("source", opts.Source)
	}

	return &Container{
		q: q,
		c: r.c,
	}
}

// This container plus a directory mounted at the given path
func (r *Container) WithMountedDirectory(path string, source DirectoryID) *Container {
	q := r.q.Select("withMountedDirectory")
	q = q.Arg("path", path)
	q = q.Arg("source", source)

	return &Container{
		q: q,
		c: r.c,
	}
}

// This container plus a file mounted at the given path
func (r *Container) WithMountedFile(path string, source FileID) *Container {
	q := r.q.Select("withMountedFile")
	q = q.Arg("path", path)
	q = q.Arg("source", source)

	return &Container{
		q: q,
		c: r.c,
	}
}

// This container plus a secret mounted into a file at the given path
func (r *Container) WithMountedSecret(path string, source SecretID) *Container {
	q := r.q.Select("withMountedSecret")
	q = q.Arg("path", path)
	q = q.Arg("source", source)

	return &Container{
		q: q,
		c: r.c,
	}
}

// This container plus a temporary directory mounted at the given path
func (r *Container) WithMountedTemp(path string) *Container {
	q := r.q.Select("withMountedTemp")
	q = q.Arg("path", path)

	return &Container{
		q: q,
		c: r.c,
	}
}

// This container plus an env variable containing the given secret
func (r *Container) WithSecretVariable(name string, secret SecretID) *Container {
	q := r.q.Select("withSecretVariable")
	q = q.Arg("name", name)
	q = q.Arg("secret", secret)

	return &Container{
		q: q,
		c: r.c,
	}
}

// This container but with a different command user
func (r *Container) WithUser(name string) *Container {
	q := r.q.Select("withUser")
	q = q.Arg("name", name)

	return &Container{
		q: q,
		c: r.c,
	}
}

// This container plus the given environment variable
func (r *Container) WithVariable(name string, value string) *Container {
	q := r.q.Select("withVariable")
	q = q.Arg("name", name)
	q = q.Arg("value", value)

	return &Container{
		q: q,
		c: r.c,
	}
}

// This container but with a different working directory
func (r *Container) WithWorkdir(path string) *Container {
	q := r.q.Select("withWorkdir")
	q = q.Arg("path", path)

	return &Container{
		q: q,
		c: r.c,
	}
}

// This container after unmounting everything at the given path.
func (r *Container) WithoutMount(path string) *Container {
	q := r.q.Select("withoutMount")
	q = q.Arg("path", path)

	return &Container{
		q: q,
		c: r.c,
	}
}

// This container minus the given environment variable
func (r *Container) WithoutVariable(name string) *Container {
	q := r.q.Select("withoutVariable")
	q = q.Arg("name", name)

	return &Container{
		q: q,
		c: r.c,
	}
}

// The working directory for all commands
func (r *Container) Workdir(ctx context.Context) (string, error) {
	q := r.q.Select("workdir")

	var response string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// A directory
type Directory struct {
	q *querybuilder.Selection
	c graphql.Client
}

// DirectoryContentsOptions contains options for Directory.Contents
type DirectoryContentsOptions struct {
	Path *string
}

// DirectoryContentsOption represents an option handler for Directory.Contents
type DirectoryContentsOption func(*DirectoryContentsOptions)

// WithDirectoryContentsPath sets the "path" option for Contents
func WithDirectoryContentsPath(path string) DirectoryContentsOption {
	return func(daggerOptions *DirectoryContentsOptions) {
		daggerOptions.Path = &path
	}
}

// Return a list of files and directories at the given path
func (r *Directory) Contents(ctx context.Context, options ...DirectoryContentsOption) ([]string, error) {
	opts := &DirectoryContentsOptions{}
	for _, fn := range options {
		fn(opts)
	}
	q := r.q.Select("contents")
	if opts != nil && opts.Path != nil {
		q = q.Arg("path", opts.Path)
	}

	var response []string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// The difference between this directory and an another directory
func (r *Directory) Diff(other DirectoryID) *Directory {
	q := r.q.Select("diff")
	q = q.Arg("other", other)

	return &Directory{
		q: q,
		c: r.c,
	}
}

// Retrieve a directory at the given path
func (r *Directory) Directory(path string) *Directory {
	q := r.q.Select("directory")
	q = q.Arg("path", path)

	return &Directory{
		q: q,
		c: r.c,
	}
}

// Retrieve a file at the given path
func (r *Directory) File(path string) *File {
	q := r.q.Select("file")
	q = q.Arg("path", path)

	return &File{
		q: q,
		c: r.c,
	}
}

// The content-addressed identifier of the directory
func (r *Directory) ID(ctx context.Context) (DirectoryID, error) {
	q := r.q.Select("id")

	var response DirectoryID
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// A secret backed by the file at the given path
func (r *Directory) Secret(ctx context.Context, path string) (SecretID, error) {
	q := r.q.Select("secret")
	q = q.Arg("path", path)

	var response SecretID
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// This directory plus the contents of the given file copied to the given path
func (r *Directory) WithCopiedFile(path string, source FileID) *Directory {
	q := r.q.Select("withCopiedFile")
	q = q.Arg("path", path)
	q = q.Arg("source", source)

	return &Directory{
		q: q,
		c: r.c,
	}
}

// This directory plus a directory written at the given path
func (r *Directory) WithDirectory(directory DirectoryID, path string) *Directory {
	q := r.q.Select("withDirectory")
	q = q.Arg("directory", directory)
	q = q.Arg("path", path)

	return &Directory{
		q: q,
		c: r.c,
	}
}

// DirectoryWithNewFileOptions contains options for Directory.WithNewFile
type DirectoryWithNewFileOptions struct {
	Contents *string
}

// DirectoryWithNewFileOption represents an option handler for Directory.WithNewFile
type DirectoryWithNewFileOption func(*DirectoryWithNewFileOptions)

// WithDirectoryWithNewFileContents sets the "contents" option for WithNewFile
func WithDirectoryWithNewFileContents(contents string) DirectoryWithNewFileOption {
	return func(daggerOptions *DirectoryWithNewFileOptions) {
		daggerOptions.Contents = &contents
	}
}

// This directory plus a new file written at the given path
func (r *Directory) WithNewFile(path string, options ...DirectoryWithNewFileOption) *Directory {
	opts := &DirectoryWithNewFileOptions{}
	for _, fn := range options {
		fn(opts)
	}
	q := r.q.Select("withNewFile")
	if opts != nil && opts.Contents != nil {
		q = q.Arg("contents", opts.Contents)
	}
	q = q.Arg("path", path)

	return &Directory{
		q: q,
		c: r.c,
	}
}

// This directory with the directory at the given path removed
func (r *Directory) WithoutDirectory(path string) *Directory {
	q := r.q.Select("withoutDirectory")
	q = q.Arg("path", path)

	return &Directory{
		q: q,
		c: r.c,
	}
}

// This directory with the file at the given path removed
func (r *Directory) WithoutFile(path string) *Directory {
	q := r.q.Select("withoutFile")
	q = q.Arg("path", path)

	return &Directory{
		q: q,
		c: r.c,
	}
}

// A file
type File struct {
	q *querybuilder.Selection
	c graphql.Client
}

// The contents of the file
func (r *File) Contents(ctx context.Context) (string, error) {
	q := r.q.Select("contents")

	var response string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// The content-addressed identifier of the file
func (r *File) ID(ctx context.Context) (FileID, error) {
	q := r.q.Select("id")

	var response FileID
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// The size of the file, in bytes
func (r *File) Size(ctx context.Context) (int, error) {
	q := r.q.Select("size")

	var response int
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// A git ref (tag or branch)
type GitRef struct {
	q *querybuilder.Selection
	c graphql.Client
}

// The digest of the current value of this ref
func (r *GitRef) Digest(ctx context.Context) (string, error) {
	q := r.q.Select("digest")

	var response string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// The filesystem tree at this ref
func (r *GitRef) Tree() *Directory {
	q := r.q.Select("tree")

	return &Directory{
		q: q,
		c: r.c,
	}
}

// A git repository
type GitRepository struct {
	q *querybuilder.Selection
	c graphql.Client
}

// Details on one branch
func (r *GitRepository) Branch(name string) *GitRef {
	q := r.q.Select("branch")
	q = q.Arg("name", name)

	return &GitRef{
		q: q,
		c: r.c,
	}
}

// List of branches on the repository
func (r *GitRepository) Branches(ctx context.Context) ([]string, error) {
	q := r.q.Select("branches")

	var response []string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

// Details on one tag
func (r *GitRepository) Tag(name string) *GitRef {
	q := r.q.Select("tag")
	q = q.Arg("name", name)

	return &GitRef{
		q: q,
		c: r.c,
	}
}

// List of tags on the repository
func (r *GitRepository) Tags(ctx context.Context) ([]string, error) {
	q := r.q.Select("tags")

	var response []string
	q = q.Bind(&response)
	return response, q.Execute(ctx, r.c)
}

type Query struct {
	q *querybuilder.Selection
	c graphql.Client
}

// QueryContainerOptions contains options for Query.Container
type QueryContainerOptions struct {
	ID *ContainerID
}

// QueryContainerOption represents an option handler for Query.Container
type QueryContainerOption func(*QueryContainerOptions)

// WithQueryContainerID sets the "id" option for Container
func WithQueryContainerID(id ContainerID) QueryContainerOption {
	return func(daggerOptions *QueryContainerOptions) {
		daggerOptions.ID = &id
	}
}

// Load a container from ID.
// Null ID returns an empty container (scratch).
func (r *Query) Container(options ...QueryContainerOption) *Container {
	opts := &QueryContainerOptions{}
	for _, fn := range options {
		fn(opts)
	}
	q := r.q.Select("container")
	if opts != nil && opts.ID != nil {
		q = q.Arg("id", opts.ID)
	}

	return &Container{
		q: q,
		c: r.c,
	}
}

// QueryDirectoryOptions contains options for Query.Directory
type QueryDirectoryOptions struct {
	ID *DirectoryID
}

// QueryDirectoryOption represents an option handler for Query.Directory
type QueryDirectoryOption func(*QueryDirectoryOptions)

// WithQueryDirectoryID sets the "id" option for Directory
func WithQueryDirectoryID(id DirectoryID) QueryDirectoryOption {
	return func(daggerOptions *QueryDirectoryOptions) {
		daggerOptions.ID = &id
	}
}

// Load a directory by ID. No argument produces an empty directory.
func (r *Query) Directory(options ...QueryDirectoryOption) *Directory {
	opts := &QueryDirectoryOptions{}
	for _, fn := range options {
		fn(opts)
	}
	q := r.q.Select("directory")
	if opts != nil && opts.ID != nil {
		q = q.Arg("id", opts.ID)
	}

	return &Directory{
		q: q,
		c: r.c,
	}
}

// Load a file by ID
func (r *Query) File(id FileID) *File {
	q := r.q.Select("file")
	q = q.Arg("id", id)

	return &File{
		q: q,
		c: r.c,
	}
}

// Query a git repository
func (r *Query) Git(url string) *GitRepository {
	q := r.q.Select("git")
	q = q.Arg("url", url)

	return &GitRepository{
		q: q,
		c: r.c,
	}
}

// An http remote
func (r *Query) HTTP(url string) *File {
	q := r.q.Select("http")
	q = q.Arg("url", url)

	return &File{
		q: q,
		c: r.c,
	}
}
