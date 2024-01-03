package schema

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/core/modules"
	"github.com/vito/dagql"
	"golang.org/x/sync/errgroup"
)

type moduleSchema struct {
	dag *dagql.Server
}

var _ SchemaResolvers = &moduleSchema{}

func (s *moduleSchema) Name() string {
	return "module"
}

func (s *moduleSchema) Schema() string {
	return strings.Join([]string{Module, TypeDef, InternalSDK}, "\n")
}

func (s *moduleSchema) Install() {
	dagql.Fields[*core.Query]{
		dagql.Func("module", s.module),
		dagql.Func("currentModule", s.currentModule).Impure(),
		dagql.Func("function", s.function),
		dagql.Func("currentFunctionCall", s.currentFunctionCall).Impure(),
		dagql.Func("typeDef", s.typeDef),
		dagql.Func("generatedCode", s.generatedCode),
		dagql.Func("moduleConfig", s.moduleConfig),
	}.Install(s.dag)

	dagql.Fields[*core.Directory]{
		dagql.NodeFunc("asModule", s.directoryAsModule),
	}.Install(s.dag)

	dagql.Fields[*core.FunctionCall]{
		dagql.Func("returnValue", s.functionCallReturnValue).Impure(),
		dagql.Func("parent", s.functionCallParent),
	}.Install(s.dag)

	dagql.Fields[*core.Module]{
		dagql.NodeFunc("initialize", s.moduleInitialize),
		dagql.Func("withSource", s.moduleWithSource),
		dagql.Func("withObject", s.moduleWithObject),
		dagql.NodeFunc("serve", s.moduleServe).Impure(),
	}.Install(s.dag)

	dagql.Fields[*modules.Config]{}.Install(s.dag)

	dagql.Fields[*core.Function]{
		dagql.Func("withDescription", s.functionWithDescription),
		dagql.Func("withArg", s.functionWithArg),
	}.Install(s.dag)

	dagql.Fields[*core.FunctionArg]{}.Install(s.dag)

	dagql.Fields[*core.FunctionCallArgValue]{}.Install(s.dag)

	dagql.Fields[*core.TypeDef]{
		dagql.Func("kind", s.typeDefKind),
		dagql.Func("withOptional", s.typeDefWithOptional),
		dagql.Func("withKind", s.typeDefWithKind),
		dagql.Func("withListOf", s.typeDefWithListOf),
		dagql.Func("withObject", s.typeDefWithObject),
		dagql.Func("withField", s.typeDefWithObjectField),
		dagql.Func("withFunction", s.typeDefWithObjectFunction),
		dagql.Func("withConstructor", s.typeDefWithObjectConstructor),
	}.Install(s.dag)
	dagql.Fields[*core.ObjectTypeDef]{}.Install(s.dag)
	dagql.Fields[*core.FieldTypeDef]{}.Install(s.dag)
	dagql.Fields[*core.ListTypeDef]{}.Install(s.dag)

	dagql.Fields[*core.GeneratedCode]{
		dagql.Func("withVCSIgnoredPaths", s.generatedCodeWithVCSIgnoredPaths),
		dagql.Func("withVCSGeneratedPaths", s.generatedCodeWithVCSGeneratedPaths),
	}.Install(s.dag)
}

func (s *moduleSchema) typeDef(ctx context.Context, _ *core.Query, args struct{}) (*core.TypeDef, error) {
	return &core.TypeDef{}, nil
}

func (s *moduleSchema) typeDefWithOptional(ctx context.Context, def *core.TypeDef, args struct {
	Optional bool
}) (*core.TypeDef, error) {
	return def.WithOptional(args.Optional), nil
}

func (s *moduleSchema) typeDefWithKind(ctx context.Context, def *core.TypeDef, args struct {
	Kind core.TypeDefKind
}) (*core.TypeDef, error) {
	return def.WithKind(args.Kind), nil
}

func (s *moduleSchema) typeDefWithListOf(ctx context.Context, def *core.TypeDef, args struct {
	ElementType core.TypeDefID
}) (*core.TypeDef, error) {
	elemType, err := args.ElementType.Load(ctx, s.dag)
	if err != nil {
		return nil, fmt.Errorf("failed to decode element type: %w", err)
	}
	return def.WithListOf(elemType.Self), nil
}

func (s *moduleSchema) typeDefWithObject(ctx context.Context, def *core.TypeDef, args struct {
	Name        string
	Description string `default:""`
}) (*core.TypeDef, error) {
	if args.Name == "" {
		return nil, fmt.Errorf("object type def must have a name")
	}
	return def.WithObject(args.Name, args.Description), nil
}

func (s *moduleSchema) typeDefWithObjectField(ctx context.Context, def *core.TypeDef, args struct {
	Name        string
	TypeDef     core.TypeDefID
	Description string `default:""`
}) (*core.TypeDef, error) {
	fieldType, err := args.TypeDef.Load(ctx, s.dag)
	if err != nil {
		return nil, fmt.Errorf("failed to decode element type: %w", err)
	}
	return def.WithObjectField(args.Name, fieldType.Self, args.Description)
}

func (s *moduleSchema) typeDefWithObjectFunction(ctx context.Context, def *core.TypeDef, args struct {
	Function core.FunctionID
}) (*core.TypeDef, error) {
	fn, err := args.Function.Load(ctx, s.dag)
	if err != nil {
		return nil, fmt.Errorf("failed to decode element type: %w", err)
	}
	return def.WithObjectFunction(fn.Self)
}

func (s *moduleSchema) typeDefWithObjectConstructor(ctx context.Context, def *core.TypeDef, args struct {
	Function core.FunctionID
}) (*core.TypeDef, error) {
	inst, err := args.Function.Load(ctx, s.dag)
	if err != nil {
		return nil, fmt.Errorf("failed to decode element type: %w", err)
	}
	fn := inst.Self.Clone()
	// Constructors are invoked by setting the ObjectName to the name of the object its constructing and the
	// FunctionName to "", so ignore the name of the function.
	fn.Name = ""
	fn.OriginalName = ""
	return def.WithObjectConstructor(fn)
}

func (s *moduleSchema) typeDefKind(ctx context.Context, def *core.TypeDef, args struct{}) (dagql.String, error) {
	return dagql.NewString(def.Kind.String()), nil
}

func (s *moduleSchema) generatedCode(ctx context.Context, _ *core.Query, args struct {
	Code core.DirectoryID
}) (*core.GeneratedCode, error) {
	dir, err := args.Code.Load(ctx, s.dag)
	if err != nil {
		return nil, err
	}
	return core.NewGeneratedCode(dir.Self), nil
}

func (s *moduleSchema) generatedCodeWithVCSIgnoredPaths(ctx context.Context, code *core.GeneratedCode, args struct {
	Paths []string
}) (*core.GeneratedCode, error) {
	return code.WithVCSIgnoredPaths(args.Paths), nil
}

func (s *moduleSchema) generatedCodeWithVCSGeneratedPaths(ctx context.Context, code *core.GeneratedCode, args struct {
	Paths []string
}) (*core.GeneratedCode, error) {
	return code.WithVCSGeneratedPaths(args.Paths), nil
}

func (s *moduleSchema) module(ctx context.Context, query *core.Query, _ struct{}) (*core.Module, error) {
	return query.NewModule(), nil
}

type moduleConfigArgs struct {
	SourceDirectory core.DirectoryID
	Subpath         string `default:""`
}

func (s *moduleSchema) moduleConfig(ctx context.Context, query *core.Query, args moduleConfigArgs) (*modules.Config, error) {
	srcDir, err := args.SourceDirectory.Load(ctx, s.dag)
	if err != nil {
		return nil, fmt.Errorf("failed to decode source directory: %w", err)
	}

	_, cfg, err := core.LoadModuleConfig(ctx, srcDir.Self, args.Subpath)
	return cfg, err
}

func (s *moduleSchema) function(ctx context.Context, _ *core.Query, args struct {
	Name       string
	ReturnType core.TypeDefID
}) (*core.Function, error) {
	returnType, err := args.ReturnType.Load(ctx, s.dag)
	if err != nil {
		return nil, fmt.Errorf("failed to decode return type: %w", err)
	}
	return core.NewFunction(args.Name, returnType.Self), nil
}

func (s *moduleSchema) functionWithDescription(ctx context.Context, fn *core.Function, args struct {
	Description string
}) (*core.Function, error) {
	return fn.WithDescription(args.Description), nil
}

func (s *moduleSchema) functionWithArg(ctx context.Context, fn *core.Function, args struct {
	Name         string
	TypeDef      core.TypeDefID
	Description  string    `default:""`
	DefaultValue core.JSON `default:""`
}) (*core.Function, error) {
	argType, err := args.TypeDef.Load(ctx, s.dag)
	if err != nil {
		return nil, fmt.Errorf("failed to decode arg type: %w", err)
	}
	return fn.WithArg(args.Name, argType.Self, args.Description, args.DefaultValue), nil
}

func (s *moduleSchema) functionCallParent(ctx context.Context, fnCall *core.FunctionCall, _ struct{}) (core.JSON, error) {
	return fnCall.Parent, nil
}

func (s *moduleSchema) moduleWithSource(ctx context.Context, self *core.Module, args struct {
	Directory core.DirectoryID
	Subpath   string `default:""`
}) (_ *core.Module, rerr error) {
	sourceDir, err := args.Directory.Load(ctx, s.dag)
	if err != nil {
		return nil, err
	}

	configPath, cfg, err := core.LoadModuleConfig(ctx, sourceDir.Self, args.Subpath)
	if err != nil {
		return nil, err
	}

	// Reposition the root of the sourceDir in case it's pointing to a subdir of current sourceDir
	if filepath.Clean(cfg.Root) != "." {
		rootPath := filepath.Join(filepath.Dir(configPath), cfg.Root)
		if rootPath != filepath.Dir(configPath) {
			configPathAbs, err := filepath.Abs(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to get config absolute path: %w", err)
			}
			rootPathAbs, err := filepath.Abs(rootPath)
			if err != nil {
				return nil, fmt.Errorf("failed to get root absolute path: %w", err)
			}
			configPath, err = filepath.Rel(rootPathAbs, configPathAbs)
			if err != nil {
				return nil, fmt.Errorf("failed to get config relative to root: %w", err)
			}
			if strings.HasPrefix(configPath, "../") {
				// this likely shouldn't happen, a client shouldn't submit a
				// module config that escapes the module root
				return nil, fmt.Errorf("module subpath is not under module root")
			}
			if rootPath != "." {
				err = s.dag.Select(ctx, sourceDir, &sourceDir, dagql.Selector{
					Field: "directory",
					Args: []dagql.NamedInput{
						{Name: "path", Value: dagql.String(rootPath)},
					},
				})
				if err != nil {
					return nil, fmt.Errorf("failed to get root directory: %w", err)
				}
			}
		}
	}

	sourceDirSubpath := filepath.Dir(configPath)

	var eg errgroup.Group
	deps := make([]dagql.Instance[*core.Module], len(cfg.Dependencies))
	for i, depRef := range cfg.Dependencies {
		i, depRef := i, depRef
		eg.Go(func() error {
			dep, err := core.LoadRef(ctx, s.dag, sourceDir, sourceDirSubpath, depRef)
			if err != nil {
				return err
			}
			deps[i] = dep
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		if errors.Is(err, dagql.ErrCacheMapRecursiveCall) {
			err = fmt.Errorf("module %s has a circular dependency: %w", cfg.Name, err)
		}
		return nil, err
	}

	self = self.Clone()
	self.NameField = cfg.Name
	self.DependencyConfig = cfg.Dependencies
	self.SDKConfig = cfg.SDK
	self.SourceDirectory = sourceDir
	self.SourceDirectorySubpath = sourceDirSubpath
	self.DependenciesField = deps

	self.Deps = core.NewModDeps(self.Dependencies()).
		Append(self.Query.DefaultDeps.Mods...)

	sdk, err := s.sdkForModule(ctx, self.Query, cfg.SDK, sourceDir, sourceDirSubpath)
	if err != nil {
		return nil, err
	}

	self.GeneratedCode, err = sdk.Codegen(ctx, self, sourceDir, sourceDirSubpath)
	if err != nil {
		return nil, err
	}

	self.Runtime, err = sdk.Runtime(ctx, self, sourceDir, sourceDirSubpath)
	if err != nil {
		return nil, fmt.Errorf("failed to get module runtime: %w", err)
	}

	return self, nil
}

func (s *moduleSchema) moduleInitialize(ctx context.Context, inst dagql.Instance[*core.Module], args struct{}) (*core.Module, error) {
	return inst.Self.Initialize(ctx, inst, dagql.CurrentID(ctx))
}

type asModuleArgs struct {
	SourceSubpath string `default:""`
}

func (s *moduleSchema) directoryAsModule(ctx context.Context, sourceDir dagql.Instance[*core.Directory], args asModuleArgs) (inst dagql.Instance[*core.Module], rerr error) {
	rerr = s.dag.Select(ctx, s.dag.Root(), &inst, dagql.Selector{
		Field: "module",
	}, dagql.Selector{
		Field: "withSource",
		Args: []dagql.NamedInput{
			{Name: "directory", Value: dagql.NewID[*core.Directory](sourceDir.ID())},
			{Name: "subpath", Value: dagql.String(args.SourceSubpath)},
		},
	}, dagql.Selector{
		Field: "initialize",
	})
	return
}

func (s *moduleSchema) currentModule(ctx context.Context, self *core.Query, _ struct{}) (inst dagql.Instance[*core.Module], err error) {
	id, err := self.CurrentModule(ctx)
	if err != nil {
		return inst, err
	}
	return id.Load(ctx, s.dag)
}

func (s *moduleSchema) currentFunctionCall(ctx context.Context, self *core.Query, _ struct{}) (*core.FunctionCall, error) {
	return self.CurrentFunctionCall(ctx)
}

func (s *moduleSchema) moduleServe(ctx context.Context, modMeta dagql.Instance[*core.Module], _ struct{}) (dagql.Nullable[core.Void], error) {
	return dagql.Null[core.Void](), modMeta.Self.Query.ServeModuleToMainClient(ctx, modMeta)
}

func (s *moduleSchema) functionCallReturnValue(ctx context.Context, fnCall *core.FunctionCall, args struct {
	Value core.JSON
}) (dagql.Nullable[core.Void], error) {
	// TODO: error out if caller is not coming from a module
	return dagql.Null[core.Void](), fnCall.ReturnValue(ctx, args.Value)
}

func (s *moduleSchema) moduleWithObject(ctx context.Context, modMeta *core.Module, args struct {
	Object core.TypeDefID
}) (_ *core.Module, rerr error) {
	def, err := args.Object.Load(ctx, s.dag)
	if err != nil {
		return nil, err
	}
	return modMeta.WithObject(ctx, def.Self)
}
