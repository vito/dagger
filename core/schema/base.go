package schema

import (
	"github.com/dagger/dagger/project"
	"github.com/dagger/dagger/router"
	"github.com/dagger/dagger/sessions"
	bkclient "github.com/moby/buildkit/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type InitializeArgs struct {
	Router        *router.Router
	SSHAuthSockID string
	Sessions      *sessions.Manager
	BKClient      *bkclient.Client
	SolveOpts     bkclient.SolveOpt
	SolveCh       chan *bkclient.SolveStatus
	Platform      specs.Platform
	DisableHostRW bool
}

func New(params InitializeArgs) (router.ExecutableSchema, error) {
	base := &baseSchema{
		router:        params.Router,
		sessions:      params.Sessions,
		bkClient:      params.BKClient,
		solveOpts:     params.SolveOpts,
		solveCh:       params.SolveCh,
		platform:      params.Platform,
		sshAuthSockID: params.SSHAuthSockID,
	}
	return router.MergeExecutableSchemas("core",
		&directorySchema{base},
		&fileSchema{base},
		&gitSchema{base},
		&containerSchema{base},
		&cacheSchema{base},
		&secretSchema{base},
		&hostSchema{base},
		&projectSchema{
			baseSchema:    base,
			projectStates: make(map[string]*project.State),
		},
		&httpSchema{base},
		&platformSchema{base},
	)
}

type baseSchema struct {
	router        *router.Router
	sessions      *sessions.Manager
	bkClient      *bkclient.Client
	solveOpts     bkclient.SolveOpt
	solveCh       chan *bkclient.SolveStatus
	platform      specs.Platform
	sshAuthSockID string
}
