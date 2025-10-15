package sdk

import (
	"context"
	"fmt"
	"os"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/dagql"
	"github.com/dagger/dagger/engine/distconsts"
)

type dangSDK struct {
	root      *core.Query
	rawConfig map[string]any
}

type dangSDKConfig struct {
}

func (sdk *dangSDK) AsRuntime() (core.Runtime, bool) {
	return sdk, true
}

func (sdk *dangSDK) AsCodeGenerator() (core.CodeGenerator, bool) {
	return sdk, true
}

func (sdk *dangSDK) AsClientGenerator() (core.ClientGenerator, bool) {
	return sdk, true
}

func (sdk *dangSDK) RequiredClientGenerationFiles(_ context.Context) (dagql.Array[dagql.String], error) {
	return dagql.NewStringArray(), nil
}

func (sdk *dangSDK) GenerateClient(
	ctx context.Context,
	modSource dagql.ObjectResult[*core.ModuleSource],
	deps *core.ModDeps,
	outputDir string,
) (inst dagql.ObjectResult[*core.Directory], err error) {
	return inst, fmt.Errorf("dang SDK does not have a client to generate")
}

func (sdk *dangSDK) Codegen(
	ctx context.Context,
	deps *core.ModDeps,
	source dagql.ObjectResult[*core.ModuleSource],
) (_ *core.GeneratedCode, rerr error) {
	dag, err := sdk.root.Server.Server(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dag for dang module sdk client generation: %w", err)
	}

	contextDir := source.Self().ContextDirectory
	rootSourcePath := source.Self().SourceRootSubpath

	var srcDir dagql.ObjectResult[*core.Directory]
	if err := dag.Select(ctx, contextDir, &srcDir, dagql.Selector{
		Field: "directory",
		Args: []dagql.NamedInput{
			{
				Name:  "path",
				Value: dagql.String(rootSourcePath),
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to get modified source directory for dang module sdk codegen: %w", err)
	}

	return &core.GeneratedCode{
		Code: srcDir,
	}, nil
}

func (sdk *dangSDK) Runtime(
	ctx context.Context,
	deps *core.ModDeps,
	source dagql.ObjectResult[*core.ModuleSource],
) (inst dagql.ObjectResult[*core.Container], rerr error) {
	dag, err := sdk.root.Server.Server(ctx)
	if err != nil {
		return inst, fmt.Errorf("failed to get dag for dang module sdk client generation: %w", err)
	}

	schemaFile, err := deps.SchemaIntrospectionJSONFile(ctx, nil)
	if err != nil {
		return inst, fmt.Errorf("failed to get schema introspection json file for dang module sdk runtime: %w", err)
	}

	modSrc := "/src/" + source.Self().SourceSubpath

	if err := dag.Select(ctx, dag.Root(), &inst,
		dagql.Selector{
			View:  dag.View,
			Field: "_builtinContainer",
			Args: []dagql.NamedInput{
				{
					Name:  "digest",
					Value: dagql.String(os.Getenv(distconsts.DangSDKManifestDigestEnvName)),
				},
			},
		},
		dagql.Selector{
			View:  dag.View,
			Field: "withDirectory",
			Args: []dagql.NamedInput{
				{
					Name:  "path",
					Value: dagql.String("/src"),
				},
				{
					Name:  "source",
					Value: dagql.NewID[*core.Directory](source.Self().ContextDirectory.ID()),
				},
			},
		},
		dagql.Selector{
			View:  dag.View,
			Field: "withFile",
			Args: []dagql.NamedInput{
				{
					Name:  "path",
					Value: dagql.String("/introspection.json"),
				},
				{
					Name:  "source",
					Value: dagql.NewID[*core.File](schemaFile.ID()),
				},
			},
		},
		dagql.Selector{
			View:  dag.View,
			Field: "withWorkdir",
			Args: []dagql.NamedInput{
				{
					Name:  "path",
					Value: dagql.String(modSrc),
				},
			},
		},
		dagql.Selector{
			View:  dag.View,
			Field: "withDefaultArgs",
			Args: []dagql.NamedInput{
				{
					Name:  "args",
					Value: dagql.ArrayInput[dagql.String]{"dang", dagql.String(modSrc)},
				},
			},
		},
	); err != nil {
		return inst, fmt.Errorf("failed to get base container from dang module sdk tarball: %w", err)
	}

	return inst, nil
}
