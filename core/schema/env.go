package schema

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/engine/buildkit"
	"github.com/dagger/dagger/universe"
	"github.com/dagger/graphql"
	"github.com/opencontainers/go-digest"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"golang.org/x/sync/errgroup"
)

type environmentSchema struct {
	*MergedSchemas
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

func (s *environmentSchema) Resolvers() Resolvers {
	return Resolvers{
		"EnvironmentID":        environmentIDResolver,
		"EnvironmentCommandID": environmentCommandIDResolver,
		"EnvironmentCheckID":   environmentCheckIDResolver,
		"EnvironmentShellID":   environmentShellIDResolver,
		"Query": ObjectResolver{
			"environment":        ToResolver(s.environment),
			"environmentCommand": ToResolver(s.environmentCommand),
			"environmentCheck":   ToResolver(s.environmentCheck),
			"environmentShell":   ToResolver(s.environmentShell),
		},
		"Environment": ObjectResolver{
			"id":               ToResolver(s.environmentID),
			"load":             ToResolver(s.load),
			"loadFromUniverse": ToResolver(s.loadFromUniverse),
			"name":             ToResolver(s.environmentName),
			"command":          ToResolver(s.command),
			"withCommand":      ToResolver(s.withCommand),
			"withCheck":        ToResolver(s.withCheck),
			"withShell":        ToResolver(s.withShell),
			"withExtension":    ToResolver(s.withExtension),
		},
		"EnvironmentCommand": ObjectResolver{
			"id":              ToResolver(s.commandID),
			"withName":        ToResolver(s.withCommandName),
			"withDescription": ToResolver(s.withCommandDescription),
			"withFlag":        ToResolver(s.withCommandFlag),
			"withResultType":  ToResolver(s.withCommandResultType),
			"setStringFlag":   ToResolver(s.setCommandStringFlag),
			"invoke":          ToResolver(s.invokeCommand),
		},
		"EnvironmentCheck": ObjectResolver{
			"id":              ToResolver(s.checkID),
			"subchecks":       ToResolver(s.subchecks),
			"withSubcheck":    ToResolver(s.withSubcheck),
			"withName":        ToResolver(s.withCheckName),
			"withDescription": ToResolver(s.withCheckDescription),
			"withFlag":        ToResolver(s.withCheckFlag),
			"setStringFlag":   ToResolver(s.setCheckStringFlag),
			"result":          ToResolver(s.checkResult),
		},
		"EnvironmentShell": ObjectResolver{
			"id":              ToResolver(s.shellID),
			"withName":        ToResolver(s.withShellName),
			"withDescription": ToResolver(s.withShellDescription),
			"withFlag":        ToResolver(s.withShellFlag),
			"setStringFlag":   ToResolver(s.setShellStringFlag),
			"endpoint":        ToResolver(s.shellEndpoint),
		},
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
	env, resolver, err := core.LoadEnvironment(ctx, s.bk, s.progSockPath, rootDir.Pipeline, s.platform, rootDir, args.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment: %w", err)
	}

	resolvers := make(Resolvers)
	doc, err := parser.ParseSchema(&ast.Source{Input: env.Schema})
	if err != nil {
		return nil, fmt.Errorf("failed to parse environment schema: %w: %s", err, env.Schema)
	}
	for _, def := range append(doc.Definitions, doc.Extensions...) {
		def := def
		if def.Kind != ast.Object {
			continue
		}
		existingResolver, ok := resolvers[def.Name]
		if !ok {
			existingResolver = ObjectResolver{}
		}
		objResolver, ok := existingResolver.(ObjectResolver)
		if !ok {
			return nil, fmt.Errorf("failed to load environment: resolver for %s is not an object resolver", def.Name)
		}
		for _, field := range def.Fields {
			field := field
			objResolver[field.Name] = ToResolver(func(ctx *core.Context, parent any, args any) (any, error) {
				res, err := resolver(ctx, parent, args)
				// don't check err yet, convert output may do some handling of that
				res, err = convertOutput(res, err, field.Type, s.MergedSchemas)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve field %s: %w", field.Name, err)
				}
				return res, nil
			})
		}
		resolvers[def.Name] = objResolver
	}

	envId, err := env.ID()
	if err != nil {
		return nil, fmt.Errorf("failed to get environment id: %w", err)
	}
	if err := s.addSchemas(StaticSchema(StaticSchemaParams{
		Name:      digest.FromString(string(envId)).Encoded(),
		Schema:    env.Schema,
		Resolvers: resolvers,
	})); err != nil {
		return nil, fmt.Errorf("failed to install environment schema: %w", err)
	}

	return env, nil
}

func convertOutput(rawOutput any, resErr error, schemaOutputType *ast.Type, s *MergedSchemas) (any, error) {
	if schemaOutputType.Elem != nil {
		schemaOutputType = schemaOutputType.Elem
	}

	// TODO: avoid hardcoding type names amap
	if schemaOutputType.Name() == "EnvironmentCheckResult" {
		checkRes := &core.EnvironmentCheckResult{}
		if resErr != nil {
			checkRes.Success = false
			// TODO: forcing users to include all relevent error output in the error/exception is probably annoying
			execErr := new(buildkit.ExecError)
			if errors.As(resErr, &execErr) {
				// TODO: stdout and then stderr is weird, need interleaved stream
				checkRes.Output = strings.Join([]string{execErr.Stdout, execErr.Stderr}, "\n")
			} else {
				return nil, fmt.Errorf("failed to execute check: %w", resErr)
			}
			return checkRes, nil
		}
		// TODO: should collect all the progress and prints from user code and set that to output instead
		output, ok := rawOutput.(string)
		if !ok {
			return nil, fmt.Errorf("expected string output for check entrypoint")
		}
		checkRes.Success = true
		checkRes.Output = output
		return checkRes, nil
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
		// between the session and environment container.
		outputStr, ok := rawOutput.(string)
		if !ok {
			return nil, fmt.Errorf("expected id string output for %s", objectName)
		}
		return resolver.FromID(outputStr)
	}
	return rawOutput, nil
}

type loadFromUniverseArgs struct {
	Name string
}

var loadUniverseOnce = &sync.Once{}
var universeDirID core.DirectoryID
var loadUniverseErr error

func (s *environmentSchema) loadFromUniverse(ctx *core.Context, parent *core.Environment, args loadFromUniverseArgs) (*core.Environment, error) {
	// TODO: unpacking to a tmpdir and loading as a local dir sucks, but what's better?
	loadUniverseOnce.Do(func() {
		tempdir, err := os.MkdirTemp("", "dagger-universe")
		if err != nil {
			loadUniverseErr = err
			return
		}

		tarReader := tar.NewReader(bytes.NewReader(universe.Tar))
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				loadUniverseErr = err
				return
			}
			if header.FileInfo().IsDir() {
				if err := os.MkdirAll(filepath.Join(tempdir, header.Name), header.FileInfo().Mode()); err != nil {
					loadUniverseErr = err
					return
				}
			} else {
				if err := os.MkdirAll(filepath.Join(tempdir, filepath.Dir(header.Name)), header.FileInfo().Mode()); err != nil {
					loadUniverseErr = err
					return
				}
				f, err := os.OpenFile(filepath.Join(tempdir, header.Name), os.O_CREATE|os.O_WRONLY, header.FileInfo().Mode())
				if err != nil {
					loadUniverseErr = err
					return
				}
				defer f.Close()
				if _, err := io.Copy(f, tarReader); err != nil {
					loadUniverseErr = err
					return
				}
			}
		}

		dir, err := core.NewHost().EngineServerDirectory(ctx, s.bk, tempdir, nil, "universe", s.platform, core.CopyFilter{})
		if err != nil {
			loadUniverseErr = err
			return
		}
		universeDirID, loadUniverseErr = dir.ID()
	})
	if loadUniverseErr != nil {
		return nil, loadUniverseErr
	}

	return s.load(ctx, parent, loadArgs{
		Source: universeDirID,
		// TODO: should be by name, not path
		ConfigPath: filepath.Join("universe", args.Name),
	})
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

type environmentShellArgs struct {
	ID core.EnvironmentShellID
}

func (s *environmentSchema) environmentShell(ctx *core.Context, parent *core.Query, args environmentShellArgs) (*core.EnvironmentShell, error) {
	return core.NewEnvironmentShell(args.ID)
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
	// TODO: just for now, should namespace asap
	parentObj := s.MergedSchemas.Schema().QueryType()
	parentVal := map[string]any{}

	// find the field resolver for this command, as installed during "load" above
	var resolver Resolver
	for objectName, possibleResolver := range s.resolvers() {
		if objectName == parentObj.Name() {
			resolver = possibleResolver
		}
	}
	if resolver == nil {
		return nil, fmt.Errorf("no resolver for %s", parentObj.Name())
	}
	objResolver, ok := resolver.(ObjectResolver)
	if !ok {
		return nil, fmt.Errorf("resolver for %s is not an object resolver", parentObj.Name())
	}
	var fieldResolver graphql.FieldResolveFn
	for fieldName, possibleFieldResolver := range objResolver {
		if fieldName == cmd.Name {
			fieldResolver = possibleFieldResolver
		}
	}
	if fieldResolver == nil {
		return nil, fmt.Errorf("no field resolver for %s.%s", parentObj.Name(), cmd.Name)
	}

	// setup the inputs and invoke it
	resolveParams := graphql.ResolveParams{
		Context: ctx,
		Source:  parentVal,
		Args:    map[string]any{},
		Info: graphql.ResolveInfo{
			FieldName:  cmd.Name,
			ParentType: parentObj,
			// TODO: we don't currently use any of the other resolve info fields, but that could change
		},
	}
	for _, flag := range cmd.Flags {
		resolveParams.Args[flag.Name] = flag.SetValue
	}
	res, err := fieldResolver(resolveParams)
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

func (s *environmentSchema) subchecks(ctx *core.Context, parent *core.EnvironmentCheck, args any) ([]*core.EnvironmentCheck, error) {
	var subchecks []*core.EnvironmentCheck
	for _, subcheckID := range parent.Subchecks {
		subcheck, err := core.NewEnvironmentCheck(subcheckID)
		if err != nil {
			return nil, err
		}
		subchecks = append(subchecks, subcheck)
	}
	return subchecks, nil
}

type withSubcheckArgs struct {
	ID core.EnvironmentCheckID
}

func (s *environmentSchema) withSubcheck(ctx *core.Context, parent *core.EnvironmentCheck, args withSubcheckArgs) (*core.EnvironmentCheck, error) {
	subcheck, err := core.NewEnvironmentCheck(args.ID)
	if err != nil {
		return nil, err
	}

	return parent.WithSubcheck(subcheck)
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

func (s *environmentSchema) checkResult(ctx *core.Context, check *core.EnvironmentCheck, _ any) ([]*core.EnvironmentCheckResult, error) {
	if len(check.Subchecks) > 0 {
		// run them in parallel instead
		var eg errgroup.Group
		results := make([]*core.EnvironmentCheckResult, len(check.Subchecks))
		for i, subcheckID := range check.Subchecks {
			i := i
			subcheck, err := core.NewEnvironmentCheck(subcheckID)
			if err != nil {
				return nil, err
			}
			eg.Go(func() error {
				res, err := s.runCheck(ctx, subcheck)
				if err != nil {
					return err
				}
				results[i] = res
				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			return nil, err
		}
		return results, nil
	}

	/* TODO: codegen clients currently request every field when list of objects are returned, so we need to avoid doing expensive work
	// here, right? Or if not then simplify this
	// Or maybe graphql-go lets you return structs with methods that match the field names?
	checkID, err := check.ID()
	if err != nil {
		return nil, err
	}
	return []*core.EnvironmentCheckResult{{ParentCheck: checkID}}, nil
	*/

	res, err := s.runCheck(ctx, check)
	if err != nil {
		return nil, err
	}
	return []*core.EnvironmentCheckResult{res}, nil
}

// private helper, not in schema
func (s *environmentSchema) runCheck(ctx *core.Context, check *core.EnvironmentCheck) (*core.EnvironmentCheckResult, error) {
	// TODO: just for now, should namespace asap
	parentObj := s.MergedSchemas.Schema().QueryType()
	parentVal := map[string]any{}

	// find the field resolver for this check, as installed during "load" above
	var resolver Resolver
	for objectName, possibleResolver := range s.resolvers() {
		if objectName == parentObj.Name() {
			resolver = possibleResolver
		}
	}
	if resolver == nil {
		return nil, fmt.Errorf("no resolver for %s", parentObj.Name())
	}
	objResolver, ok := resolver.(ObjectResolver)
	if !ok {
		return nil, fmt.Errorf("resolver for %s is not an object resolver", parentObj.Name())
	}
	var fieldResolver graphql.FieldResolveFn
	for fieldName, possibleFieldResolver := range objResolver {
		if fieldName == check.Name {
			fieldResolver = possibleFieldResolver
		}
	}
	if fieldResolver == nil {
		return nil, fmt.Errorf("no field resolver for %s.%s", parentObj.Name(), check.Name)
	}

	// setup the inputs and invoke it
	resolveParams := graphql.ResolveParams{
		Context: ctx,
		Source:  parentVal,
		Args:    map[string]any{},
		Info: graphql.ResolveInfo{
			FieldName:  check.Name,
			ParentType: parentObj,
			// TODO: we don't currently use any of the other resolve info fields, but that could change
		},
	}
	for _, flag := range check.Flags {
		resolveParams.Args[flag.Name] = flag.SetValue
	}

	res, err := fieldResolver(resolveParams)
	if err != nil {
		return nil, err
	}
	// all the result type handling is done in convertOutput above
	checkRes, ok := res.(*core.EnvironmentCheckResult)
	if !ok {
		return nil, fmt.Errorf("unexpected result type %T from check resolver", res)
	}
	checkRes.Name = check.Name
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

func (s *environmentSchema) shellEndpoint(ctx *core.Context, parent *core.EnvironmentShell, args any) (string, error) {
	// TODO: just for now, should namespace asap
	parentObj := s.MergedSchemas.Schema().QueryType()
	parentVal := map[string]any{}

	// find the field resolver for this shell, as installed during "load" above
	var resolver Resolver
	for objectName, possibleResolver := range s.resolvers() {
		if objectName == parentObj.Name() {
			resolver = possibleResolver
		}
	}
	if resolver == nil {
		return "", fmt.Errorf("no resolver for %s", parentObj.Name())
	}
	objResolver, ok := resolver.(ObjectResolver)
	if !ok {
		return "", fmt.Errorf("resolver for %s is not an object resolver", parentObj.Name())
	}
	var fieldResolver graphql.FieldResolveFn
	for fieldName, possibleFieldResolver := range objResolver {
		if fieldName == parent.Name {
			fieldResolver = possibleFieldResolver
		}
	}
	if fieldResolver == nil {
		return "", fmt.Errorf("no field resolver for %s.%s", parentObj.Name(), parent.Name)
	}

	// setup the inputs and invoke it
	resolveParams := graphql.ResolveParams{
		Context: ctx,
		Source:  parentVal,
		Args:    map[string]any{},
		Info: graphql.ResolveInfo{
			FieldName:  parent.Name,
			ParentType: parentObj,
			// TODO: we don't currently use any of the other resolve info fields, but that could change
		},
	}
	for _, flag := range parent.Flags {
		resolveParams.Args[flag.Name] = flag.SetValue
	}
	res, err := fieldResolver(resolveParams)
	if err != nil {
		return "", err
	}

	ctr, ok := res.(*core.Container)
	if !ok {
		return "", fmt.Errorf("unexpected result type %T from shell resolver", res)
	}

	// TODO: dedupe w/ containerSchema
	endpoint, handler, err := ctr.ShellEndpoint(s.bk, s.progSockPath)
	if err != nil {
		return "", err
	}

	s.MuxEndpoint(path.Join("/", endpoint), handler)
	return "ws://dagger/" + endpoint, nil
}