package schema

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/moby/buildkit/frontend/dockerfile/shell"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vito/dagql"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/core/pipeline"
)

type containerSchema struct {
	srv *dagql.Server
}

var _ SchemaResolvers = &containerSchema{}

func (s *containerSchema) Name() string {
	return "container"
}

func (s *containerSchema) Install() {
	dagql.Fields[*core.Query]{
		dagql.Func("container", s.container),
	}.Install(s.srv)

	dagql.Fields[*core.Container]{
		Syncer[*core.Container](),
		dagql.Func("from", s.from),
		dagql.Func("build", s.build),
		dagql.Func("rootfs", s.rootfs),
		dagql.Func("pipeline", s.pipeline),
		dagql.Func("withRootfs", s.withRootfs),
		dagql.Func("file", s.file),
		dagql.Func("directory", s.directory),
		dagql.Func("user", s.user),
		dagql.Func("withUser", s.withUser),
		dagql.Func("withoutUser", s.withoutUser),
		dagql.Func("workdir", s.workdir),
		dagql.Func("withWorkdir", s.withWorkdir),
		dagql.Func("withoutWorkdir", s.withoutWorkdir),
		dagql.Func("envVariables", s.envVariables),
		dagql.Func("envVariable", s.envVariable),
		dagql.Func("withEnvVariable", s.withEnvVariable),
		dagql.Func("withSecretVariable", s.withSecretVariable),
		dagql.Func("withoutEnvVariable", s.withoutEnvVariable),
		dagql.Func("withLabel", s.withLabel),
		dagql.Func("label", s.label),
		dagql.Func("labels", s.labels),
		dagql.Func("withoutLabel", s.withoutLabel),
		dagql.Func("entrypoint", s.entrypoint),
		dagql.Func("withEntrypoint", s.withEntrypoint),
		dagql.Func("withoutEntrypoint", s.withoutEntrypoint),
		dagql.Func("defaultArgs", s.defaultArgs),
		dagql.Func("withDefaultArgs", s.withDefaultArgs),
		dagql.Func("withoutDefaultArgs", s.withoutDefaultArgs),
		dagql.Func("mounts", s.mounts),
		dagql.Func("withMountedDirectory", s.withMountedDirectory),
		dagql.Func("withMountedFile", s.withMountedFile),
		dagql.Func("withMountedTemp", s.withMountedTemp),
		dagql.Func("withMountedCache", s.withMountedCache),
		dagql.Func("withMountedSecret", s.withMountedSecret),
		dagql.Func("withUnixSocket", s.withUnixSocket),
		dagql.Func("withoutUnixSocket", s.withoutUnixSocket),
		dagql.Func("withoutMount", s.withoutMount),
		dagql.Func("withFile", s.withFile),
		dagql.Func("withNewFile", s.withNewFile),
		dagql.Func("withDirectory", s.withDirectory),
		dagql.Func("withExec", s.withExec),
		dagql.Func("stdout", s.stdout),
		dagql.Func("stderr", s.stderr),
		dagql.Func("publish", s.publish).Impure(),
		dagql.Func("platform", s.platform),
		dagql.Func("export", s.export),
		dagql.Func("asTarball", s.asTarball),
		dagql.Func("import", s.import_),
		dagql.Func("withRegistryAuth", s.withRegistryAuth),
		dagql.Func("withoutRegistryAuth", s.withoutRegistryAuth),
		dagql.Func("imageRef", s.imageRef),
		dagql.Func("withExposedPort", s.withExposedPort),
		dagql.Func("withoutExposedPort", s.withoutExposedPort),
		dagql.Func("exposedPorts", s.exposedPorts),
		dagql.Func("withServiceBinding", s.withServiceBinding),
		dagql.Func("withFocus", s.withFocus),
		dagql.Func("withoutFocus", s.withoutFocus),
		dagql.NodeFunc("shellEndpoint", s.shellEndpoint).Impure(),
		dagql.Func("experimentalWithGPU", s.withGPU),
		dagql.Func("experimentalWithAllGPUs", s.withAllGPUs),
	}.Install(s.srv)
}

type containerArgs struct {
	ID       dagql.Optional[core.ContainerID]
	Platform dagql.Optional[core.Platform]
}

func (s *containerSchema) container(ctx context.Context, parent *core.Query, args containerArgs) (_ *core.Container, rerr error) {
	if args.ID.Valid {
		inst, err := args.ID.Value.Load(ctx, s.srv)
		if err != nil {
			return nil, err
		}
		// NB: what we kind of want is to return an Instance[*core.Container] in
		// this case, but this API is deprecated anyhow
		return inst.Self, nil
	}
	var platform core.Platform
	if args.Platform.Valid {
		platform = args.Platform.Value
	} else {
		platform = parent.Platform
	}
	return parent.NewContainer(platform), nil
}

type containerFromArgs struct {
	Address string `doc:"Image's address from its registry.\n\nFormatted as [host]/[user]/[repo]:[tag] (e.g., \"docker.io/dagger/dagger:main\")."`
}

func (s *containerSchema) from(ctx context.Context, parent *core.Container, args containerFromArgs) (*core.Container, error) {
	return parent.From(ctx, args.Address)
}

type containerBuildArgs struct {
	Context    core.DirectoryID
	Dockerfile string                             `default:"Dockerfile"`
	Target     string                             `default:""`
	BuildArgs  []dagql.InputObject[core.BuildArg] `default:"[]"`
	Secrets    []core.SecretID                    `default:"[]"`
}

func (s *containerSchema) build(ctx context.Context, parent *core.Container, args containerBuildArgs) (*core.Container, error) {
	dir, err := args.Context.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	secrets, err := dagql.LoadIDs(ctx, s.srv, args.Secrets)
	if err != nil {
		return nil, err
	}
	return parent.Build(
		ctx,
		dir.Self,
		args.Dockerfile,
		collectInputsSlice(args.BuildArgs),
		args.Target,
		secrets,
	)
}

type containerWithRootFSArgs struct {
	Directory core.DirectoryID
}

func (s *containerSchema) withRootfs(ctx context.Context, parent *core.Container, args containerWithRootFSArgs) (*core.Container, error) {
	dir, err := args.Directory.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.WithRootFS(ctx, dir.Self)
}

type containerPipelineArgs struct {
	Name        string
	Description string                              `default:""`
	Labels      []dagql.InputObject[pipeline.Label] `default:"[]"`
}

func (s *containerSchema) pipeline(ctx context.Context, parent *core.Container, args containerPipelineArgs) (*core.Container, error) {
	return parent.WithPipeline(ctx, args.Name, args.Description, collectInputsSlice(args.Labels))
}

func (s *containerSchema) rootfs(ctx context.Context, parent *core.Container, args struct{}) (*core.Directory, error) {
	return parent.RootFS(ctx)
}

type containerExecArgs struct {
	core.ContainerExecOpts
}

func (s *containerSchema) withExec(ctx context.Context, parent *core.Container, args containerExecArgs) (*core.Container, error) {
	return parent.WithExec(ctx, args.ContainerExecOpts)
}

func (s *containerSchema) stdout(ctx context.Context, parent *core.Container, _ struct{}) (dagql.String, error) {
	content, err := parent.MetaFileContents(ctx, "stdout")
	if err != nil {
		return "", err
	}
	return dagql.NewString(string(content)), nil
}

func (s *containerSchema) stderr(ctx context.Context, parent *core.Container, _ struct{}) (dagql.String, error) {
	content, err := parent.MetaFileContents(ctx, "stderr")
	if err != nil {
		return "", err
	}
	return dagql.NewString(string(content)), nil
}

type containerGpuArgs struct {
	core.ContainerGPUOpts
}

func (s *containerSchema) withGPU(ctx context.Context, parent *core.Container, args containerGpuArgs) (*core.Container, error) {
	return parent.WithGPU(ctx, args.ContainerGPUOpts)
}

func (s *containerSchema) withAllGPUs(ctx context.Context, parent *core.Container, args struct{}) (*core.Container, error) {
	return parent.WithGPU(ctx, core.ContainerGPUOpts{Devices: []string{"all"}})
}

type containerWithEntrypointArgs struct {
	Args            []string
	KeepDefaultArgs bool `default:"false"`
}

func (s *containerSchema) withEntrypoint(ctx context.Context, parent *core.Container, args containerWithEntrypointArgs) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		cfg.Entrypoint = args.Args
		if !args.KeepDefaultArgs {
			cfg.Cmd = nil
		}
		return cfg
	})
}

type containerWithoutEntrypointArgs struct {
	KeepDefaultArgs bool `default:"false"`
}

func (s *containerSchema) withoutEntrypoint(ctx context.Context, parent *core.Container, args containerWithoutEntrypointArgs) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		cfg.Entrypoint = nil
		if !args.KeepDefaultArgs {
			cfg.Cmd = nil
		}
		return cfg
	})
}

func (s *containerSchema) entrypoint(ctx context.Context, parent *core.Container, args struct{}) ([]string, error) {
	cfg, err := parent.ImageConfig(ctx)
	if err != nil {
		return nil, err
	}

	return cfg.Entrypoint, nil
}

type containerWithDefaultArgs struct {
	Args []string
}

func (s *containerSchema) withDefaultArgs(ctx context.Context, parent *core.Container, args containerWithDefaultArgs) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		if args.Args == nil {
			cfg.Cmd = []string{}
			return cfg
		}

		cfg.Cmd = args.Args
		return cfg
	})
}

func (s *containerSchema) withoutDefaultArgs(ctx context.Context, parent *core.Container, _ struct{}) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		cfg.Cmd = nil
		return cfg
	})
}

func (s *containerSchema) defaultArgs(ctx context.Context, parent *core.Container, args struct{}) ([]string, error) {
	cfg, err := parent.ImageConfig(ctx)
	if err != nil {
		return nil, err
	}

	return cfg.Cmd, nil
}

type containerWithUserArgs struct {
	Name string
}

func (s *containerSchema) withUser(ctx context.Context, parent *core.Container, args containerWithUserArgs) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		cfg.User = args.Name
		return cfg
	})
}

func (s *containerSchema) withoutUser(ctx context.Context, parent *core.Container, _ struct{}) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		cfg.User = ""
		return cfg
	})
}

func (s *containerSchema) user(ctx context.Context, parent *core.Container, args struct{}) (string, error) {
	cfg, err := parent.ImageConfig(ctx)
	if err != nil {
		return "", err
	}

	return cfg.User, nil
}

type containerWithWorkdirArgs struct {
	Path string
}

func (s *containerSchema) withWorkdir(ctx context.Context, parent *core.Container, args containerWithWorkdirArgs) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		cfg.WorkingDir = absPath(cfg.WorkingDir, args.Path)
		return cfg
	})
}

func (s *containerSchema) withoutWorkdir(ctx context.Context, parent *core.Container, _ struct{}) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		cfg.WorkingDir = ""
		return cfg
	})
}

func (s *containerSchema) workdir(ctx context.Context, parent *core.Container, args struct{}) (string, error) {
	cfg, err := parent.ImageConfig(ctx)
	if err != nil {
		return "", err
	}

	return cfg.WorkingDir, nil
}

type containerWithVariableArgs struct {
	Name   string
	Value  string
	Expand bool `default:"false"`
}

func (s *containerSchema) withEnvVariable(ctx context.Context, parent *core.Container, args containerWithVariableArgs) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		value := args.Value

		if args.Expand {
			value = os.Expand(value, func(k string) string {
				v, _ := core.LookupEnv(cfg.Env, k)
				return v
			})
		}

		cfg.Env = core.AddEnv(cfg.Env, args.Name, value)

		return cfg
	})
}

type containerWithoutVariableArgs struct {
	Name string
}

func (s *containerSchema) withoutEnvVariable(ctx context.Context, parent *core.Container, args containerWithoutVariableArgs) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		newEnv := []string{}

		core.WalkEnv(cfg.Env, func(k, _, env string) {
			if !shell.EqualEnvKeys(k, args.Name) {
				newEnv = append(newEnv, env)
			}
		})

		cfg.Env = newEnv

		return cfg
	})
}

type EnvVariable struct {
	Name  string `field:"true"`
	Value string `field:"true"`
}

func (EnvVariable) Type() *ast.Type {
	return &ast.Type{
		NamedType: "EnvVariable",
		NonNull:   true,
	}
}

func (s *containerSchema) envVariables(ctx context.Context, parent *core.Container, args struct{}) ([]EnvVariable, error) {
	cfg, err := parent.ImageConfig(ctx)
	if err != nil {
		return nil, err
	}

	vars := make([]EnvVariable, 0, len(cfg.Env))

	core.WalkEnv(cfg.Env, func(k, v, _ string) {
		vars = append(vars, EnvVariable{Name: k, Value: v})
	})

	return vars, nil
}

type containerVariableArgs struct {
	Name string
}

func (s *containerSchema) envVariable(ctx context.Context, parent *core.Container, args containerVariableArgs) (dagql.Nullable[dagql.String], error) {
	none := dagql.Null[dagql.String]()

	cfg, err := parent.ImageConfig(ctx)
	if err != nil {
		return none, err
	}

	if val, ok := core.LookupEnv(cfg.Env, args.Name); ok {
		return dagql.NonNull(dagql.NewString(val)), nil
	}

	return none, nil
}

type Label struct {
	Name  string `field:"true"`
	Value string `field:"true"`
}

func (Label) Type() *ast.Type {
	return &ast.Type{
		NamedType: "Label",
		NonNull:   true,
	}
}

func (s *containerSchema) labels(ctx context.Context, parent *core.Container, args struct{}) ([]Label, error) {
	cfg, err := parent.ImageConfig(ctx)
	if err != nil {
		return nil, err
	}

	labels := make([]Label, 0, len(cfg.Labels))
	for name, value := range cfg.Labels {
		label := Label{
			Name:  name,
			Value: value,
		}

		labels = append(labels, label)
	}

	// FIXME(vito): sort, test; order must be stable for IDs to work as expected

	return labels, nil
}

type containerLabelArgs struct {
	Name string
}

func (s *containerSchema) label(ctx context.Context, parent *core.Container, args containerLabelArgs) (dagql.Nullable[dagql.String], error) {
	none := dagql.Null[dagql.String]()

	cfg, err := parent.ImageConfig(ctx)
	if err != nil {
		return none, err
	}

	if val, ok := cfg.Labels[args.Name]; ok {
		return dagql.NonNull(dagql.NewString(val)), nil
	}

	return none, nil
}

type containerWithMountedDirectoryArgs struct {
	Path   string
	Source core.DirectoryID
	Owner  string `default:""`
}

func (s *containerSchema) withMountedDirectory(ctx context.Context, parent *core.Container, args containerWithMountedDirectoryArgs) (*core.Container, error) {
	dir, err := args.Source.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.WithMountedDirectory(ctx, args.Path, dir.Self, args.Owner, false)
}

type containerPublishArgs struct {
	Address           dagql.String
	PlatformVariants  []core.ContainerID `default:"[]"`
	ForcedCompression dagql.Optional[core.ImageLayerCompression]
	MediaTypes        core.ImageMediaTypes `default:"OCIMediaTypes"`
}

func (s *containerSchema) publish(ctx context.Context, parent *core.Container, args containerPublishArgs) (dagql.String, error) {
	variants, err := dagql.LoadIDs(ctx, s.srv, args.PlatformVariants)
	if err != nil {
		return "", err
	}
	ref, err := parent.Publish(
		ctx,
		args.Address.String(),
		variants,
		args.ForcedCompression.Value,
		args.MediaTypes,
	)
	if err != nil {
		return "", err
	}
	return dagql.NewString(ref), nil
}

type containerWithMountedFileArgs struct {
	Path   string
	Source core.FileID
	Owner  string `default:""`
}

func (s *containerSchema) withMountedFile(ctx context.Context, parent *core.Container, args containerWithMountedFileArgs) (*core.Container, error) {
	file, err := args.Source.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.WithMountedFile(ctx, args.Path, file.Self, args.Owner, false)
}

type containerWithMountedCacheArgs struct {
	Path    string
	Cache   core.CacheVolumeID
	Source  dagql.Optional[core.DirectoryID]
	Sharing core.CacheSharingMode `default:"SHARED"`
	Owner   string                `default:""`
}

func (s *containerSchema) withMountedCache(ctx context.Context, parent *core.Container, args containerWithMountedCacheArgs) (*core.Container, error) {
	var dir *core.Directory
	if args.Source.Valid {
		inst, err := args.Source.Value.Load(ctx, s.srv)
		if err != nil {
			return nil, err
		}
		dir = inst.Self
	}

	cache, err := args.Cache.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}

	return parent.WithMountedCache(
		ctx,
		args.Path,
		cache.Self,
		dir,
		args.Sharing,
		args.Owner,
	)
}

type containerWithMountedTempArgs struct {
	Path string
}

func (s *containerSchema) withMountedTemp(ctx context.Context, parent *core.Container, args containerWithMountedTempArgs) (*core.Container, error) {
	return parent.WithMountedTemp(ctx, args.Path)
}

type containerWithoutMountArgs struct {
	Path string
}

func (s *containerSchema) withoutMount(ctx context.Context, parent *core.Container, args containerWithoutMountArgs) (*core.Container, error) {
	return parent.WithoutMount(ctx, args.Path)
}

func (s *containerSchema) mounts(ctx context.Context, parent *core.Container, _ struct{}) (dagql.Array[dagql.String], error) {
	targets, err := parent.MountTargets(ctx)
	if err != nil {
		return nil, err
	}
	return dagql.NewStringArray(targets...), nil
}

type containerWithLabelArgs struct {
	Name  string
	Value string
}

func (s *containerSchema) withLabel(ctx context.Context, parent *core.Container, args containerWithLabelArgs) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		if cfg.Labels == nil {
			cfg.Labels = make(map[string]string)
		}
		cfg.Labels[args.Name] = args.Value
		return cfg
	})
}

type containerWithoutLabelArgs struct {
	Name string
}

func (s *containerSchema) withoutLabel(ctx context.Context, parent *core.Container, args containerWithoutLabelArgs) (*core.Container, error) {
	return parent.UpdateImageConfig(ctx, func(cfg specs.ImageConfig) specs.ImageConfig {
		delete(cfg.Labels, args.Name)
		return cfg
	})
}

type containerDirectoryArgs struct {
	Path string
}

func (s *containerSchema) directory(ctx context.Context, parent *core.Container, args containerDirectoryArgs) (*core.Directory, error) {
	return parent.Directory(ctx, args.Path)
}

type containerFileArgs struct {
	Path string
}

func (s *containerSchema) file(ctx context.Context, parent *core.Container, args containerFileArgs) (*core.File, error) {
	return parent.File(ctx, args.Path)
}

func absPath(workDir string, containerPath string) string {
	if path.IsAbs(containerPath) {
		return containerPath
	}

	if workDir == "" {
		workDir = "/"
	}

	return path.Join(workDir, containerPath)
}

type containerWithSecretVariableArgs struct {
	Name   string
	Secret core.SecretID
}

func (s *containerSchema) withSecretVariable(ctx context.Context, parent *core.Container, args containerWithSecretVariableArgs) (*core.Container, error) {
	secret, err := args.Secret.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.WithSecretVariable(ctx, args.Name, secret.Self)
}

type containerWithMountedSecretArgs struct {
	Path   string
	Source core.SecretID
	Owner  string `default:""`
	Mode   int    `default:"0400"` // FIXME(vito): verify octal
}

func (s *containerSchema) withMountedSecret(ctx context.Context, parent *core.Container, args containerWithMountedSecretArgs) (*core.Container, error) {
	secret, err := args.Source.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.WithMountedSecret(ctx, args.Path, secret.Self, args.Owner, fs.FileMode(args.Mode))
}

type containerWithDirectoryArgs struct {
	WithDirectoryArgs
	Owner string `default:""`
}

func (s *containerSchema) withDirectory(ctx context.Context, parent *core.Container, args containerWithDirectoryArgs) (*core.Container, error) {
	dir, err := args.Directory.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.WithDirectory(ctx, args.Path, dir.Self, args.CopyFilter, args.Owner)
}

type containerWithFileArgs struct {
	WithFileArgs
	Owner string `default:""`
}

func (s *containerSchema) withFile(ctx context.Context, parent *core.Container, args containerWithFileArgs) (*core.Container, error) {
	file, err := args.Source.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.WithFile(ctx, args.Path, file.Self, args.Permissions, args.Owner)
}

type containerWithNewFileArgs struct {
	Path        string
	Contents    string `default:""`
	Permissions int    `default:"0644"`
	Owner       string `default:""`
}

func (s *containerSchema) withNewFile(ctx context.Context, parent *core.Container, args containerWithNewFileArgs) (*core.Container, error) {
	return parent.WithNewFile(ctx, args.Path, []byte(args.Contents), fs.FileMode(args.Permissions), args.Owner)
}

type containerWithUnixSocketArgs struct {
	Path   string
	Source core.SocketID
	Owner  string `default:""`
}

func (s *containerSchema) withUnixSocket(ctx context.Context, parent *core.Container, args containerWithUnixSocketArgs) (*core.Container, error) {
	socket, err := args.Source.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.WithUnixSocket(ctx, args.Path, socket.Self, args.Owner)
}

type containerWithoutUnixSocketArgs struct {
	Path string
}

func (s *containerSchema) withoutUnixSocket(ctx context.Context, parent *core.Container, args containerWithoutUnixSocketArgs) (*core.Container, error) {
	return parent.WithoutUnixSocket(ctx, args.Path)
}

func (s *containerSchema) platform(ctx context.Context, parent *core.Container, args struct{}) (core.Platform, error) {
	return parent.Platform, nil
}

type containerExportArgs struct {
	Path              string
	PlatformVariants  []core.ContainerID `default:"[]"`
	ForcedCompression dagql.Optional[core.ImageLayerCompression]
	MediaTypes        core.ImageMediaTypes `default:"OCIMediaTypes"`
}

func (s *containerSchema) export(ctx context.Context, parent *core.Container, args containerExportArgs) (dagql.Boolean, error) {
	variants, err := dagql.LoadIDs(ctx, s.srv, args.PlatformVariants)
	if err != nil {
		return false, err
	}
	if err := parent.Export(
		ctx,
		args.Path,
		variants,
		args.ForcedCompression.Value,
		args.MediaTypes,
	); err != nil {
		return false, err
	}

	return true, nil
}

type containerAsTarballArgs struct {
	PlatformVariants  []core.ContainerID `default:"[]"`
	ForcedCompression dagql.Optional[core.ImageLayerCompression]
	MediaTypes        core.ImageMediaTypes `default:"OCIMediaTypes"`
}

func (s *containerSchema) asTarball(ctx context.Context, parent *core.Container, args containerAsTarballArgs) (*core.File, error) {
	variants, err := dagql.LoadIDs(ctx, s.srv, args.PlatformVariants)
	if err != nil {
		return nil, err
	}
	return parent.AsTarball(ctx, variants, args.ForcedCompression.Value, args.MediaTypes)
}

type containerImportArgs struct {
	Source core.FileID
	Tag    string `default:""`
}

func (s *containerSchema) import_(ctx context.Context, parent *core.Container, args containerImportArgs) (*core.Container, error) { // nolint:revive
	log.Println("!!! CONTAINER IMPORTING", args.Source.Display(), args.Tag)
	start := time.Now()
	defer func() {
		log.Println("!!! DONE CONTAINER IMPORTING", time.Since(start), args.Source.Display(), args.Tag)
	}()
	source, err := args.Source.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.Import(
		ctx,
		source.Self,
		args.Tag,
	)
}

type containerWithRegistryAuthArgs struct {
	Address  string
	Username string
	Secret   core.SecretID
}

func (s *containerSchema) withRegistryAuth(ctx context.Context, parent *core.Container, args containerWithRegistryAuthArgs) (*core.Container, error) {
	secret, err := args.Secret.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}

	secretBytes, err := parent.Query.Secrets.GetSecret(ctx, secret.Self.Name)
	if err != nil {
		return nil, err
	}

	if err := parent.Query.Auth.AddCredential(args.Address, args.Username, string(secretBytes)); err != nil {
		return nil, err
	}

	return parent, nil
}

type containerWithoutRegistryAuthArgs struct {
	Address string
}

func (s *containerSchema) withoutRegistryAuth(_ context.Context, parent *core.Container, args containerWithoutRegistryAuthArgs) (*core.Container, error) {
	if err := parent.Query.Auth.RemoveCredential(args.Address); err != nil {
		return nil, err
	}

	return parent, nil
}

func (s *containerSchema) imageRef(ctx context.Context, parent *core.Container, args struct{}) (string, error) {
	return parent.ImageRefOrErr(ctx)
}

type containerWithServiceBindingArgs struct {
	Alias   string
	Service core.ServiceID
}

func (s *containerSchema) withServiceBinding(ctx context.Context, parent *core.Container, args containerWithServiceBindingArgs) (*core.Container, error) {
	svc, err := args.Service.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}

	return parent.WithServiceBinding(ctx, svc.ID(), svc.Self, args.Alias)
}

type containerWithExposedPortArgs struct {
	Port        int
	Protocol    core.NetworkProtocol `default:"TCP"`
	Description *string
}

func (s *containerSchema) withExposedPort(ctx context.Context, parent *core.Container, args containerWithExposedPortArgs) (*core.Container, error) {
	return parent.WithExposedPort(core.Port{
		Protocol:    args.Protocol,
		Port:        args.Port,
		Description: args.Description,
	})
}

type containerWithoutExposedPortArgs struct {
	Port     int
	Protocol core.NetworkProtocol `default:"TCP"`
}

func (s *containerSchema) withoutExposedPort(ctx context.Context, parent *core.Container, args containerWithoutExposedPortArgs) (*core.Container, error) {
	return parent.WithoutExposedPort(args.Port, args.Protocol)
}

func (s *containerSchema) exposedPorts(ctx context.Context, parent *core.Container, args struct{}) ([]core.Port, error) {
	// get descriptions from `Container.Ports` (not in the OCI spec)
	ports := make(map[string]core.Port, len(parent.Ports))
	for _, p := range parent.Ports {
		ociPort := fmt.Sprintf("%d/%s", p.Port, p.Protocol.Network())
		ports[ociPort] = p
	}

	exposedPorts := []core.Port{}
	for ociPort := range parent.Config.ExposedPorts {
		p, exists := ports[ociPort]
		if !exists {
			// ignore errors when parsing from OCI
			port, protoStr, ok := strings.Cut(ociPort, "/")
			if !ok {
				continue
			}
			portNr, err := strconv.Atoi(port)
			if err != nil {
				continue
			}
			proto, err := core.NetworkProtocols.Lookup(strings.ToUpper(protoStr))
			if err != nil {
				// FIXME(vito): should this and above return nil, err instead?
				continue
			}
			p = core.Port{
				Port:     portNr,
				Protocol: proto,
			}
		}
		exposedPorts = append(exposedPorts, p)
	}

	return exposedPorts, nil
}

func (s *containerSchema) withFocus(ctx context.Context, parent *core.Container, args struct{}) (*core.Container, error) {
	child := parent.Clone()
	child.Focused = true
	return child, nil
}

func (s *containerSchema) withoutFocus(ctx context.Context, parent *core.Container, args struct{}) (*core.Container, error) {
	child := parent.Clone()
	child.Focused = false
	return child, nil
}

func (s *containerSchema) shellEndpoint(ctx context.Context, parent dagql.Instance[*core.Container], args struct{}) (dagql.String, error) {
	endpoint, handler, err := parent.Self.ShellEndpoint(parent.ID())
	if err != nil {
		return "", err
	}

	if err := parent.Self.Query.MuxEndpoint(ctx, path.Join("/", endpoint), handler); err != nil {
		return "", err
	}

	return dagql.NewString("ws://dagger/" + endpoint), nil
}
