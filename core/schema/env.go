package schema

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/core/pipeline"
	"github.com/dagger/dagger/engine"
	"github.com/dagger/dagger/universe"
	"github.com/dagger/graphql"
	"github.com/moby/buildkit/util/bklog"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"golang.org/x/sync/errgroup"
)

type environmentSchema struct {
	*MergedSchemas
	loadedEnvCache *core.CacheMap[string, *core.Environment] // env name -> env
}

var _ ExecutableSchema = &environmentSchema{}

func (s *environmentSchema) Name() string {
	return "environment"
}

func (s *environmentSchema) Schema() string {
	return Environment
}

var environmentIDResolver = stringResolver(core.EnvironmentID(""))

var environmentCommandIDResolver = stringResolver(core.EnvironmentCommandID(""))

var environmentCheckIDResolver = stringResolver(core.EnvironmentCheckID(""))

var environmentShellIDResolver = stringResolver(core.EnvironmentShellID(""))

var environmentFunctionIDResolver = stringResolver(core.EnvironmentFunctionID(""))

func (s *environmentSchema) Resolvers() Resolvers {
	return Resolvers{
		"EnvironmentID":         environmentIDResolver,
		"EnvironmentCommandID":  environmentCommandIDResolver,
		"EnvironmentCheckID":    environmentCheckIDResolver,
		"EnvironmentShellID":    environmentShellIDResolver,
		"EnvironmentFunctionID": environmentFunctionIDResolver,
		"Query": ObjectResolver{
			"environment":            ToResolver(s.environment),
			"environmentCommand":     ToResolver(s.environmentCommand),
			"environmentCheck":       ToResolver(s.environmentCheck),
			"environmentCheckResult": ToResolver(s.environmentCheckResult),
			"environmentShell":       ToResolver(s.environmentShell),
			"environmentFunction":    ToResolver(s.environmentFunction),
			"currentEnvironment":     ToResolver(s.currentEnvironment),
			// TODO:
			"loadUniverse": ToResolver(s.loadUniverse),
		},
		"Environment": ToIDableObjectResolver(core.EnvironmentID.ToEnvironment, ObjectResolver{
			"id":            ToResolver(s.environmentID),
			"load":          ToResolver(s.load),
			"name":          ToResolver(s.environmentName),
			"command":       ToResolver(s.command),
			"withCommand":   ToResolver(s.withCommand),
			"withCheck":     ToResolver(s.withCheck),
			"withShell":     ToResolver(s.withShell),
			"withExtension": ToResolver(s.withExtension),
			"withFunction":  ToResolver(s.withFunction),
		}),
		"EnvironmentFunction": ToIDableObjectResolver(core.EnvironmentFunctionID.ToEnvironmentFunction, ObjectResolver{
			"id":              ToResolver(s.functionID),
			"withName":        ToResolver(s.withFunctionName),
			"withDescription": ToResolver(s.withFunctionDescription),
			"withArg":         ToResolver(s.withFunctionArg),
			"withResultType":  ToResolver(s.withFunctionResultType),
		}),
		"EnvironmentCommand": ToIDableObjectResolver(core.EnvironmentCommandID.ToEnvironmentCommand, ObjectResolver{
			"id":              ToResolver(s.commandID),
			"withName":        ToResolver(s.withCommandName),
			"withDescription": ToResolver(s.withCommandDescription),
			"withFlag":        ToResolver(s.withCommandFlag),
			"withResultType":  ToResolver(s.withCommandResultType),
			"setStringFlag":   ToResolver(s.setCommandStringFlag),
			"invoke":          ToResolver(s.invokeCommand),
		}),
		"EnvironmentCheck": ToIDableObjectResolver(core.EnvironmentCheckID.ToEnvironmentCheck, ObjectResolver{
			"id":              ToResolver(s.checkID),
			"withName":        ToResolver(s.withCheckName),
			"withDescription": ToResolver(s.withCheckDescription),
			"withFlag":        ToResolver(s.withCheckFlag),
			"setStringFlag":   ToResolver(s.setCheckStringFlag),
			"withSubcheck":    ToResolver(s.withSubcheck),
			"withContainer":   ToResolver(s.withCheckContainer),
			"result":          ToResolver(s.checkResult),
		}),
		"EnvironmentShell": ToIDableObjectResolver(core.EnvironmentShellID.ToEnvironmentShell, ObjectResolver{
			"id":              ToResolver(s.shellID),
			"withName":        ToResolver(s.withShellName),
			"withDescription": ToResolver(s.withShellDescription),
			"withFlag":        ToResolver(s.withShellFlag),
			"setStringFlag":   ToResolver(s.setShellStringFlag),
			"endpoint":        ToResolver(s.shellEndpoint),
		}),
	}
}

func (s *environmentSchema) Dependencies() []ExecutableSchema {
	return nil
}

type environmentArgs struct {
	ID core.EnvironmentID
}

func (s *environmentSchema) environment(ctx *core.Context, parent *core.Query, args environmentArgs) (*core.Environment, error) {
	return core.NewEnvironment(args.ID)
}

func (s *environmentSchema) environmentID(ctx *core.Context, parent *core.Environment, args any) (core.EnvironmentID, error) {
	return parent.ID()
}

func (s *environmentSchema) environmentName(ctx *core.Context, parent *core.Environment, args any) (string, error) {
	return parent.Config.Name, nil
}

type loadArgs struct {
	// TODO: rename Source to RootDir
	Source     core.DirectoryID
	ConfigPath string
}

func (s *environmentSchema) load(ctx *core.Context, _ *core.Environment, args loadArgs) (*core.Environment, error) {
	rootDir, err := args.Source.ToDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to load env root directory: %w", err)
	}

	envCfg, err := core.LoadEnvironmentConfig(ctx, s.bk, rootDir, args.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment config: %w", err)
	}

	var eg errgroup.Group
	for _, dep := range envCfg.Dependencies {
		dep := dep
		// TODO: currently just assuming that all deps are local and that they all share the same root
		depConfigPath := filepath.Join(filepath.Dir(args.ConfigPath), dep)
		eg.Go(func() error {
			_, err := s.load(ctx, nil, loadArgs{Source: args.Source, ConfigPath: depConfigPath})
			if err != nil {
				return fmt.Errorf("failed to load environment dependency %q: %w", dep, err)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("failed to load environment dependencies: %w", err)
	}

	return s.loadedEnvCache.GetOrInitialize(envCfg.Name, func() (*core.Environment, error) {
		env, err := core.LoadEnvironment(ctx, s.bk, s.progSockPath, rootDir.Pipeline, s.platform, rootDir, args.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load environment: %w", err)
		}

		executableSchema, err := s.envToExecutableSchema(ctx, env, rootDir.Pipeline)
		if err != nil {
			return nil, fmt.Errorf("failed to convert environment to executable schema: %w", err)
		}
		if err := s.addSchemas(executableSchema); err != nil {
			return nil, fmt.Errorf("failed to install environment schema: %w", err)
		}
		return env, nil
	})
}

func (s *environmentSchema) envToExecutableSchema(ctx *core.Context, env *core.Environment, pipeline pipeline.Path) (ExecutableSchema, error) {
	doc, err := parser.ParseSchema(&ast.Source{Input: env.Schema})
	if err != nil {
		return nil, fmt.Errorf("failed to parse environment schema: %w: %s", err, env.Schema)
	}
	objName, err := env.GQLObjectName()
	if err != nil {
		return nil, fmt.Errorf("failed to get environment object name: %w", err)
	}
	def := doc.Definitions.ForName(objName)
	if def == nil {
		return nil, fmt.Errorf("failed to find environment object %q in schema", objName)
	}

	objResolver := ObjectResolver{}
	for _, field := range def.Fields {
		field := field
		// TODO: ugly spaghetti, for checks the resolver in the environment is for just the result field, not
		// the whole check object. That's fine but need some less convoluted code implementing this pattern
		if field.Type.Name() == "EnvironmentCheck" {
			var check *core.EnvironmentCheck
			for _, candidateCheck := range env.Checks {
				if candidateCheck.Name == field.Name {
					check = candidateCheck
					break
				}
			}
			if check == nil {
				return nil, fmt.Errorf("failed to find check %q in environment", field.Name)
			}

			// TODO:
			bklog.G(ctx).Debugf("ADDING RESOLVER FOR CHECK %s", field.Name)

			objResolver[field.Name] = ToResolver(func(ctx *core.Context, parent any, args any) (any, error) {
				argBytes, err := json.Marshal(args)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal args: %w", err)
				}
				argMap := map[string]any{}
				if err := json.Unmarshal(argBytes, &argMap); err != nil {
					return nil, fmt.Errorf("failed to unmarshal args: %w", err)
				}
				for argName, argVal := range argMap {
					argValStr, ok := argVal.(string)
					if !ok {
						return nil, fmt.Errorf("expected check arg %s to be a string, got %T", argName, argVal)
					}
					check, err = check.SetStringFlag(argName, argValStr)
					if err != nil {
						return nil, fmt.Errorf("failed to set check arg %s: %w", argName, err)
					}
				}
				// TODO:
				bklog.G(ctx).Debugf("CHECK RESOLVER %s %s %+v", field.Name, ctx.ResolveParams.Info.Path.AsArray(), check)
				return check, nil
			})
			continue
		}

		fieldResolver, err := env.FieldResolver(ctx, s.bk, s.progSockPath, pipeline)
		if err != nil {
			return nil, fmt.Errorf("failed to get field resolver for %s: %w", field.Name, err)
		}
		objResolver[field.Name] = ToResolver(func(ctx *core.Context, parent any, args any) (any, error) {
			res, err := fieldResolver(ctx, parent, args)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve field %s: %w", field.Name, err)
			}
			res, err = convertOutput(res, field.Type, s.MergedSchemas)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve field %s: %w", field.Name, err)
			}
			return res, nil
		})
	}
	resolvers := Resolvers{
		"Query": ObjectResolver{
			env.Config.Name: PassthroughResolver,
		},
		def.Name: objResolver,
	}

	return StaticSchema(StaticSchemaParams{
		Name:      env.Config.Name,
		Schema:    env.Schema,
		Resolvers: resolvers,
	}), nil
}

func convertOutput(rawOutput any, schemaOutputType *ast.Type, s *MergedSchemas) (any, error) {
	if schemaOutputType.Elem != nil {
		schemaOutputType = schemaOutputType.Elem
	}

	// see if the output type needs to be converted from an id to a dagger object (container, directory, etc)
	for objectName, baseResolver := range s.resolvers() {
		if objectName != schemaOutputType.Name() {
			continue
		}
		resolver, ok := baseResolver.(IDableObjectResolver)
		if !ok {
			continue
		}

		// ID-able dagger objects are serialized as their ID string across the wire
		// between the server and environment container.
		outputStr, ok := rawOutput.(string)
		if !ok {
			return nil, fmt.Errorf("expected id string output for %s, got %T", objectName, rawOutput)
		}
		return resolver.FromID(outputStr)
	}
	return rawOutput, nil
}

type commandArgs struct {
	Name string
}

func (s *environmentSchema) command(ctx *core.Context, parent *core.Environment, args commandArgs) (*core.EnvironmentCommand, error) {
	for _, cmd := range parent.Commands {
		if cmd.Name == args.Name {
			return cmd, nil
		}
	}
	return nil, fmt.Errorf("no such command %s", args.Name)
}

type withCommandArgs struct {
	ID core.EnvironmentCommandID
}

func (s *environmentSchema) withCommand(ctx *core.Context, parent *core.Environment, args withCommandArgs) (*core.Environment, error) {
	cmd, err := args.ID.ToEnvironmentCommand()
	if err != nil {
		return nil, err
	}
	return parent.WithCommand(ctx, cmd)
}

type withCheckArgs struct {
	ID core.EnvironmentCheckID
}

func (s *environmentSchema) withCheck(ctx *core.Context, parent *core.Environment, args withCheckArgs) (*core.Environment, error) {
	cmd, err := args.ID.ToEnvironmentCheck()
	if err != nil {
		return nil, err
	}
	return parent.WithCheck(ctx, cmd)
}

type withShellArgs struct {
	ID core.EnvironmentShellID
}

func (s *environmentSchema) withShell(ctx *core.Context, parent *core.Environment, args withShellArgs) (*core.Environment, error) {
	cmd, err := args.ID.ToEnvironmentShell()
	if err != nil {
		return nil, err
	}
	return parent.WithShell(ctx, cmd)
}

type withFunctionArgs struct {
	ID core.EnvironmentFunctionID
}

func (s *environmentSchema) withFunction(ctx *core.Context, parent *core.Environment, args withFunctionArgs) (*core.Environment, error) {
	// TODO:
	defer func() {
		if err := recover(); err != nil {
			bklog.G(ctx).Errorf("panic in withFunction: %v %s", err, string(debug.Stack()))
			panic(err)
		}
	}()

	fn, err := args.ID.ToEnvironmentFunction()
	if err != nil {
		return nil, err
	}
	return parent.WithFunction(ctx, fn)
}

type withExtensionArgs struct {
	ID        core.EnvironmentID
	Namespace string
}

func (s *environmentSchema) withExtension(ctx *core.Context, parent *core.Environment, args withExtensionArgs) (*core.Environment, error) {
	// TODO:
	panic("implement me")
}

type environmentCommandArgs struct {
	ID core.EnvironmentCommandID
}

func (s *environmentSchema) environmentCommand(ctx *core.Context, parent *core.Query, args environmentCommandArgs) (*core.EnvironmentCommand, error) {
	return core.NewEnvironmentCommand(args.ID)
}

type environmentCheckArgs struct {
	ID core.EnvironmentCheckID
}

func (s *environmentSchema) environmentCheck(ctx *core.Context, parent *core.Query, args environmentCheckArgs) (*core.EnvironmentCheck, error) {
	return core.NewEnvironmentCheck(args.ID)
}

type environmentCheckResultArgs struct {
	Success bool
	Output  string
}

func (s *environmentSchema) environmentCheckResult(ctx *core.Context, parent *core.Query, args environmentCheckResultArgs) (*core.EnvironmentCheckResult, error) {
	return &core.EnvironmentCheckResult{
		Success: args.Success,
		Output:  args.Output,
	}, nil
}

type environmentShellArgs struct {
	ID core.EnvironmentShellID
}

func (s *environmentSchema) environmentShell(ctx *core.Context, parent *core.Query, args environmentShellArgs) (*core.EnvironmentShell, error) {
	return core.NewEnvironmentShell(args.ID)
}

type environmentFunctionArgs struct {
	ID core.EnvironmentFunctionID
}

func (s *environmentSchema) environmentFunction(ctx *core.Context, parent *core.Query, args environmentFunctionArgs) (*core.EnvironmentFunction, error) {
	return core.NewEnvironmentFunction(args.ID)
}

func (s *environmentSchema) currentEnvironment(ctx *core.Context, parent *core.Query, args any) (*core.Environment, error) {
	clientMetadata, err := engine.ClientMetadataFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.loadedEnvCache.GetOrInitialize(clientMetadata.EnvironmentName, func() (*core.Environment, error) {
		return nil, fmt.Errorf("no such environment %s", clientMetadata.EnvironmentName)
	})
}

func (s *environmentSchema) commandID(ctx *core.Context, parent *core.EnvironmentCommand, args any) (core.EnvironmentCommandID, error) {
	return parent.ID()
}

type withCommandNameArgs struct {
	Name string
}

func (s *environmentSchema) withCommandName(ctx *core.Context, parent *core.EnvironmentCommand, args withCommandNameArgs) (*core.EnvironmentCommand, error) {
	return parent.WithName(args.Name), nil
}

type withCommandFlagArgs struct {
	Name        string
	Description string
}

func (s *environmentSchema) withCommandFlag(ctx *core.Context, parent *core.EnvironmentCommand, args withCommandFlagArgs) (*core.EnvironmentCommand, error) {
	return parent.WithFlag(core.EnvironmentCommandFlag{
		Name:        args.Name,
		Description: args.Description,
	}), nil
}

type withCommandResultTypeArgs struct {
	Name string
}

func (s *environmentSchema) withCommandResultType(ctx *core.Context, parent *core.EnvironmentCommand, args withCommandResultTypeArgs) (*core.EnvironmentCommand, error) {
	return parent.WithResultType(args.Name), nil
}

type withCommandDescriptionArgs struct {
	Description string
}

func (s *environmentSchema) withCommandDescription(ctx *core.Context, parent *core.EnvironmentCommand, args withCommandDescriptionArgs) (*core.EnvironmentCommand, error) {
	return parent.WithDescription(args.Description), nil
}

type setCommandStringFlagArgs struct {
	Name  string
	Value string
}

func (s *environmentSchema) setCommandStringFlag(ctx *core.Context, parent *core.EnvironmentCommand, args setCommandStringFlagArgs) (*core.EnvironmentCommand, error) {
	return parent.SetStringFlag(args.Name, args.Value)
}

func (s *environmentSchema) invokeCommand(ctx *core.Context, cmd *core.EnvironmentCommand, _ any) (map[string]any, error) {
	fieldResolver, resolveParams, err := s.getEnvFieldResolver(ctx, cmd.EnvironmentName, cmd.Name)
	if err != nil {
		return nil, err
	}
	for _, flag := range cmd.Flags {
		resolveParams.Args[flag.Name] = flag.SetValue
	}
	res, err := fieldResolver(*resolveParams)
	if err != nil {
		return nil, err
	}

	// TODO: actual struct for this
	// return a map in the shape of the InvokeResult object in environment.graphqls
	return map[string]any{
		strings.ToLower(cmd.ResultType): res,
	}, nil
}

func (s *environmentSchema) checkID(ctx *core.Context, parent *core.EnvironmentCheck, args any) (core.EnvironmentCheckID, error) {
	return parent.ID()
}

type withCheckNameArgs struct {
	Name string
}

func (s *environmentSchema) withCheckName(ctx *core.Context, parent *core.EnvironmentCheck, args withCheckNameArgs) (*core.EnvironmentCheck, error) {
	return parent.WithName(args.Name), nil
}

type withCheckDescriptionArgs struct {
	Description string
}

func (s *environmentSchema) withCheckDescription(ctx *core.Context, parent *core.EnvironmentCheck, args withCheckDescriptionArgs) (*core.EnvironmentCheck, error) {
	return parent.WithDescription(args.Description), nil
}

type withCheckFlagArgs struct {
	Name        string
	Description string
}

func (s *environmentSchema) withCheckFlag(ctx *core.Context, parent *core.EnvironmentCheck, args withCheckFlagArgs) (*core.EnvironmentCheck, error) {
	return parent.WithFlag(core.EnvironmentCheckFlag{
		Name:        args.Name,
		Description: args.Description,
	}), nil
}

type setCheckStringFlagArgs struct {
	Name  string
	Value string
}

func (s *environmentSchema) setCheckStringFlag(ctx *core.Context, parent *core.EnvironmentCheck, args setCheckStringFlagArgs) (*core.EnvironmentCheck, error) {
	return parent.SetStringFlag(args.Name, args.Value)
}

type withSubcheckArgs struct {
	ID core.EnvironmentCheckID
}

func (s *environmentSchema) withSubcheck(ctx *core.Context, parent *core.EnvironmentCheck, args withSubcheckArgs) (*core.EnvironmentCheck, error) {
	return parent.WithSubcheck(args.ID), nil
}

type withCheckContainerArgs struct {
	ID core.ContainerID
}

func (s *environmentSchema) withCheckContainer(ctx *core.Context, parent *core.EnvironmentCheck, args withCheckContainerArgs) (*core.EnvironmentCheck, error) {
	return parent.WithContainer(args.ID), nil
}

func (s *environmentSchema) checkResult(ctx *core.Context, check *core.EnvironmentCheck, _ any) (*core.EnvironmentCheckResult, error) {
	// if there's no subchecks, resolve the result directly
	if len(check.Subchecks) == 0 {
		// TODO:
		bklog.G(ctx).Debugf("CHECK RESULT RESOLVER %s %+v %+v", check.Name, ctx.ResolveParams.Info.Path.AsArray(), check)

		if check.ContainerID != "" {
			ctr, err := check.ContainerID.ToContainer()
			if err != nil {
				return nil, err
			}
			err = ctr.Evaluate(ctx, s.bk)
			if err != nil {
				return &core.EnvironmentCheckResult{
					Name:    check.EnvironmentName + "." + check.Name,
					Success: false,
					Output:  err.Error(),
				}, nil
			}
			return &core.EnvironmentCheckResult{
				Name:    check.EnvironmentName + "." + check.Name,
				Success: true,
				// TODO: get stdout/stderr, set to output
			}, nil
		}

		// resolve the result directly
		// TODO: more strands of spaghetti
		env, err := s.loadedEnvCache.GetOrInitialize(check.EnvironmentName, func() (*core.Environment, error) {
			return nil, fmt.Errorf("environment %s not found", check.EnvironmentName)
		})
		if err != nil {
			return nil, err
		}
		fieldResolver, err := env.FieldResolver(ctx, s.bk, s.progSockPath, nil) // TODO: set pipline to something
		if err != nil {
			return nil, err
		}
		envObjName := strings.ToUpper(env.Config.Name[:1]) + env.Config.Name[1:]
		resolveParams := graphql.ResolveParams{
			Context: ctx,
			Source:  struct{}{},
			Args:    map[string]any{},
			Info: graphql.ResolveInfo{
				FieldName:  check.Name,
				ParentType: s.MergedSchemas.Schema().Type(envObjName),
			},
		}
		for _, flag := range check.Flags {
			resolveParams.Args[flag.Name] = flag.SetValue
		}
		res, err := fieldResolver(&core.Context{
			Context:       ctx,
			ResolveParams: resolveParams,
			Vertex:        ctx.Vertex,
		}, resolveParams.Source, resolveParams.Args)
		if err != nil {
			return nil, fmt.Errorf("error resolving check %s.%s: %w", check.EnvironmentName, check.Name, err)
		}

		// TODO: ugly

		var checkRes core.EnvironmentCheckResult
		switch v := res.(type) {
		case string:
			// if the sdk returned a string, that means success and the output is the string
			checkRes.Success = true
			checkRes.Output = v
		default:
			// otherwise assume it is a json serialized CheckResult object
			// TODO: more efficient if it works? err := mapstructure.Decode(rawOutput, &res)
			bs, err := json.Marshal(res)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal environment check result: %w", err)
			}
			if err := json.Unmarshal(bs, &checkRes); err != nil {
				return nil, fmt.Errorf("failed to unmarshal environment check result %s: %w", string(bs), err)
			}
		}

		checkRes.Name = check.EnvironmentName + "." + check.Name

		// TODO:
		bklog.G(ctx).Debugf("CHECK RESULT RESOLVER RETURNED %s %+v %+v", check.Name, res, checkRes)
		return &checkRes, nil
	}

	// otherwise, resolve each subcheck and construct the result from that

	// TODO:
	bklog.G(ctx).Debugf("CHECK SUBRESULT RESOLVER %s %+v", check.Name, check)

	// TODO: guard against infinite recursion
	checkRes := &core.EnvironmentCheckResult{
		Name:       check.EnvironmentName + "." + check.Name,
		Subresults: make([]*core.EnvironmentCheckResult, len(check.Subchecks)),
		Success:    true,
		// TODO: could combine output in theory, but not sure what the format would be.
		// For now, output can just be collected from subresults
	}
	var eg errgroup.Group
	for i, subcheckID := range check.Subchecks {
		i, subcheckID := i, subcheckID
		eg.Go(func() error {
			subcheck, err := subcheckID.ToEnvironmentCheck()
			if err != nil {
				return err
			}
			subresult, err := s.checkResult(ctx, subcheck, nil)
			if err != nil {
				return err
			}
			checkRes.Subresults[i] = subresult
			if !subresult.Success {
				checkRes.Success = false
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return checkRes, nil
}

func (s *environmentSchema) shellID(ctx *core.Context, parent *core.EnvironmentShell, args any) (core.EnvironmentShellID, error) {
	return parent.ID()
}

type withShellNameArgs struct {
	Name string
}

func (s *environmentSchema) withShellName(ctx *core.Context, parent *core.EnvironmentShell, args withShellNameArgs) (*core.EnvironmentShell, error) {
	return parent.WithName(args.Name), nil
}

type withShellDescriptionArgs struct {
	Description string
}

func (s *environmentSchema) withShellDescription(ctx *core.Context, parent *core.EnvironmentShell, args withShellDescriptionArgs) (*core.EnvironmentShell, error) {
	return parent.WithDescription(args.Description), nil
}

type withShellFlagArgs struct {
	Name        string
	Description string
}

func (s *environmentSchema) withShellFlag(ctx *core.Context, parent *core.EnvironmentShell, args withShellFlagArgs) (*core.EnvironmentShell, error) {
	return parent.WithFlag(core.EnvironmentShellFlag{
		Name:        args.Name,
		Description: args.Description,
	}), nil
}

type setShellStringFlagArgs struct {
	Name  string
	Value string
}

func (s *environmentSchema) setShellStringFlag(ctx *core.Context, parent *core.EnvironmentShell, args setShellStringFlagArgs) (*core.EnvironmentShell, error) {
	return parent.SetStringFlag(args.Name, args.Value)
}

func (s *environmentSchema) shellEndpoint(ctx *core.Context, shell *core.EnvironmentShell, args any) (string, error) {
	fieldResolver, resolveParams, err := s.getEnvFieldResolver(ctx, shell.EnvironmentName, shell.Name)
	if err != nil {
		return "", err
	}
	for _, flag := range shell.Flags {
		resolveParams.Args[flag.Name] = flag.SetValue
	}
	res, err := fieldResolver(*resolveParams)
	if err != nil {
		return "", fmt.Errorf("error resolving shell container: %w", err)
	}

	ctr, ok := res.(*core.Container)
	if !ok {
		return "", fmt.Errorf("unexpected result type %T from shell resolver", res)
	}

	// TODO: dedupe w/ containerSchema
	endpoint, handler, err := ctr.ShellEndpoint(s.bk, s.progSockPath)
	if err != nil {
		return "", fmt.Errorf("error getting shell endpoint: %w", err)
	}

	s.MuxEndpoint(path.Join("/", endpoint), handler)
	return "ws://dagger/" + endpoint, nil
}

// TODO:
var loadUniverseOnce = &sync.Once{}
var loadUniverseLocalPath string
var universeEnvPaths []string
var unpackUniverseError error

func (s *environmentSchema) loadUniverse(ctx *core.Context, _ any, _ any) (any, error) {
	// TODO: unpacking to a tmpdir and loading as a local dir is dumb
	loadUniverseOnce.Do(func() {
		unpackUniverseError = func() error {
			var err error
			loadUniverseLocalPath, err = os.MkdirTemp("", "dagger-universe")
			if err != nil {
				return fmt.Errorf("failed to create tempdir: %w", err)
			}

			tarReader := tar.NewReader(bytes.NewReader(universe.Tar))
			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("failed to read tar header: %w", err)
				}

				// TODO: hack to skip broken envs, remove
				if strings.HasPrefix(filepath.Clean(header.Name), "universe/_") {
					bklog.G(ctx).Warnf("SKIPPING ENV %s", header.Name)
					continue
				}

				if header.FileInfo().IsDir() {
					if err := os.MkdirAll(filepath.Join(loadUniverseLocalPath, header.Name), header.FileInfo().Mode()); err != nil {
						return fmt.Errorf("failed to create dir %s: %w", header.Name, err)
					}
				} else {
					if filepath.Base(header.Name) == "dagger.json" && strings.HasPrefix(filepath.Clean(header.Name), "universe/") {
						universeEnvPaths = append(universeEnvPaths, filepath.Dir(header.Name))
					}

					if err := os.MkdirAll(filepath.Join(loadUniverseLocalPath, filepath.Dir(header.Name)), header.FileInfo().Mode()); err != nil {
						return fmt.Errorf("failed to create dir %s: %w", filepath.Dir(header.Name), err)
					}
					f, err := os.OpenFile(filepath.Join(loadUniverseLocalPath, header.Name), os.O_CREATE|os.O_WRONLY, header.FileInfo().Mode())
					if err != nil {
						return fmt.Errorf("failed to create file %s: %w", header.Name, err)
					}
					defer f.Close()
					if _, err := io.Copy(f, tarReader); err != nil {
						return fmt.Errorf("failed to copy file %s: %w", header.Name, err)
					}
				}
			}
			return nil
		}()
	})
	if unpackUniverseError != nil {
		bklog.G(ctx).Fatalf("FAILED TO LOAD UNIVERSE: %s", unpackUniverseError)
		return nil, unpackUniverseError
	}

	universeDir, err := core.NewHost().EngineServerDirectory(ctx, s.bk, loadUniverseLocalPath, nil, "universe", s.platform, core.CopyFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to load universe dir: %w", err)
	}

	universeSchemas := make([]ExecutableSchema, len(universeEnvPaths))
	var eg errgroup.Group
	for i, envPath := range universeEnvPaths {
		i, envPath := i, envPath
		eg.Go(func() error {
			// TODO: support dependencies
			envCfg, err := core.LoadEnvironmentConfig(ctx, s.bk, universeDir, envPath)
			if err != nil {
				return fmt.Errorf("failed to load environment config: %w", err)
			}

			// TODO: this doesn't work if the universe env was loaded before this call, but that's currently not possible
			// since load universe is hardcoded in the engine client to be called before Connect returns. Once that's fixed,
			// keep in mind
			_, err = s.loadedEnvCache.GetOrInitialize(envCfg.Name, func() (*core.Environment, error) {
				env, err := core.LoadEnvironment(ctx, s.bk, s.progSockPath, universeDir.Pipeline, s.platform, universeDir, envPath)
				if err != nil {
					return nil, fmt.Errorf("failed to load environment: %w", err)
				}
				executableSchema, err := s.envToExecutableSchema(ctx, env, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to convert environment to executable schema: %w", err)
				}
				universeSchemas[i] = executableSchema
				return env, nil
			})
			if err != nil {
				return fmt.Errorf("failed to load environment %s: %w", envCfg.Name, err)
			}
			return nil
		})
	}
	err = eg.Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to load universe: %w", err)
	}

	err = s.addSchemas(universeSchemas...)
	if err != nil {
		return nil, fmt.Errorf("failed to add universe schemas: %w", err)
	}
	return true, nil
}

func (s *environmentSchema) functionID(ctx *core.Context, parent *core.EnvironmentFunction, args any) (core.EnvironmentFunctionID, error) {
	return parent.ID()
}

type withFunctionNameArgs struct {
	Name string
}

func (s *environmentSchema) withFunctionName(ctx *core.Context, parent *core.EnvironmentFunction, args withFunctionNameArgs) (*core.EnvironmentFunction, error) {
	return parent.WithName(args.Name), nil
}

type withFunctionDescriptionArgs struct {
	Description string
}

func (s *environmentSchema) withFunctionDescription(ctx *core.Context, parent *core.EnvironmentFunction, args withFunctionDescriptionArgs) (*core.EnvironmentFunction, error) {
	return parent.WithDescription(args.Description), nil
}

type withFunctionArgArgs struct {
	Name        string
	Description string
	ArgType     string
	IsList      bool
	IsOptional  bool
}

func (s *environmentSchema) withFunctionArg(ctx *core.Context, parent *core.EnvironmentFunction, args withFunctionArgArgs) (*core.EnvironmentFunction, error) {
	return parent.WithArg(core.EnvironmentFunctionArg{
		Name:        args.Name,
		Description: args.Description,
		ArgType:     args.ArgType,
		IsList:      args.IsList,
		IsOptional:  args.IsOptional,
	}), nil
}

type withFunctionResultTypeArgs struct {
	Name string
}

func (s *environmentSchema) withFunctionResultType(ctx *core.Context, parent *core.EnvironmentFunction, args withFunctionResultTypeArgs) (*core.EnvironmentFunction, error) {
	return parent.WithResultType(args.Name), nil
}

func (s *environmentSchema) getEnvFieldResolver(ctx context.Context, envName, fieldName string) (graphql.FieldResolveFn, *graphql.ResolveParams, error) {
	// TODO: don't hardcode
	envObjName := strings.ToUpper(envName[:1]) + envName[1:]

	var resolver Resolver
	for objectName, possibleResolver := range s.resolvers() {
		if objectName == envObjName {
			resolver = possibleResolver
		}
	}
	if resolver == nil {
		return nil, nil, fmt.Errorf("no resolver for %s", envObjName)
	}
	objResolver, ok := resolver.(ObjectResolver)
	if !ok {
		return nil, nil, fmt.Errorf("resolver for %s is not an object resolver", envObjName)
	}
	var fieldResolver graphql.FieldResolveFn
	for possibleFieldName, possibleFieldResolver := range objResolver {
		if possibleFieldName == fieldName {
			fieldResolver = possibleFieldResolver
		}
	}
	if fieldResolver == nil {
		return nil, nil, fmt.Errorf("no field resolver for %s.%s", envObjName, fieldName)
	}

	return fieldResolver, &graphql.ResolveParams{
		Context: ctx,
		Source:  struct{}{}, // TODO: could support data fields too
		Args:    map[string]any{},
		Info: graphql.ResolveInfo{
			FieldName:  fieldName,
			ParentType: s.MergedSchemas.Schema().Type(envObjName),
			// TODO: we don't currently use any of the other resolve info fields, but that could change
		},
	}, nil
}