package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/vito/dagql"
	"github.com/vito/dagql/introspection"
	"github.com/vito/progrock"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/core/pipeline"
	"github.com/dagger/dagger/engine"
)

type querySchema struct {
	srv *dagql.Server
}

var _ SchemaResolvers = &querySchema{}

func (s *querySchema) Name() string {
	return "query"
}

func (s *querySchema) Schema() string {
	return Query
}

func (s *querySchema) Install() {
	introspection.Install[*core.Query](s.srv)

	s.srv.InstallScalar(core.JSON{})
	s.srv.InstallScalar(core.Void{})
	s.srv.InstallScalar(core.DynamicID{})

	core.NetworkProtocols.Install(s.srv)
	core.ImageLayerCompressions.Install(s.srv)
	core.ImageMediaTypesEnum.Install(s.srv)
	core.CacheSharingModes.Install(s.srv)
	core.TypeDefKinds.Install(s.srv)

	dagql.MustInputSpec(pipeline.Label{}).Install(s.srv)
	dagql.MustInputSpec(core.PortForward{}).Install(s.srv)
	dagql.MustInputSpec(core.BuildArg{}).Install(s.srv)

	dagql.Fields[EnvVariable]{}.Install(s.srv)

	dagql.Fields[core.Port]{}.Install(s.srv)

	dagql.Fields[Label]{}.Install(s.srv)

	dagql.Fields[*core.Query]{
		dagql.Func("pipeline", s.pipeline),
		dagql.Func("checkVersionCompatibility", s.checkVersionCompatibility),
	}.Install(s.srv)
}

type pipelineArgs struct {
	Name        dagql.String
	Description dagql.String `default:""`
	Labels      dagql.Optional[dagql.ArrayInput[dagql.InputObject[pipeline.Label]]]
}

func (s *querySchema) pipeline(ctx context.Context, parent *core.Query, args pipelineArgs) (*core.Query, error) {
	parent = parent.Clone()
	parent.Pipeline = parent.Pipeline.Add(pipeline.Pipeline{
		Name:        args.Name.String(),
		Description: args.Description.String(),
		Labels:      collectInputs(args.Labels),
	})
	return parent, nil
}

type checkVersionCompatibilityArgs struct {
	Version string
}

func (s *querySchema) checkVersionCompatibility(ctx context.Context, _ *core.Query, args checkVersionCompatibilityArgs) (dagql.Boolean, error) {
	recorder := progrock.FromContext(ctx)

	// Skip development version
	if strings.Contains(engine.Version, "devel") {
		recorder.Debug("Using development engine; skipping version compatibility check.")
		return true, nil
	}

	engineVersionStr := strings.TrimPrefix(engine.Version, "v")
	engineVersion, err := semver.Parse(engineVersionStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse engine version as semver: %s", err)
	}

	sdkVersionStr := strings.TrimPrefix(args.Version, "v")
	sdkVersion, err := semver.Parse(sdkVersionStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse SDK version as semver: %s", err)
	}

	// If the Engine is a major version above the SDK version, fails
	// TODO: throw an error and abort the session
	if engineVersion.Major > sdkVersion.Major {
		recorder.Warn(fmt.Sprintf("Dagger engine version (%s) is significantly newer than the SDK's required version (%s). Please update your SDK.", engineVersion, sdkVersion))

		// return false, fmt.Errorf("Dagger engine version (%s) is not compatible with the SDK (%s)", engineVersion, sdkVersion)
		return false, nil
	}

	// If the Engine is older than the SDK, fails
	// TODO: throw an error and abort the session
	if engineVersion.LT(sdkVersion) {
		recorder.Warn(fmt.Sprintf("Dagger engine version (%s) is older than the SDK's required version (%s). Please update your Dagger CLI.", engineVersion, sdkVersion))

		// return false, fmt.Errorf("API version is older than the SDK, please update your Dagger CLI")
		return false, nil
	}

	// If the Engine is a minor version newer, warn
	if engineVersion.Minor > sdkVersion.Minor {
		recorder.Warn(fmt.Sprintf("Dagger engine version (%s) is newer than the SDK's required version (%s). Consider updating your SDK.", engineVersion, sdkVersion))
	}

	return true, nil
}
