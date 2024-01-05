package schema

import (
	"context"

	"github.com/dagger/dagger/core"
	"github.com/vito/dagql"
)

var _ SchemaResolvers = &gitSchema{}

type gitSchema struct {
	srv *dagql.Server
}

func (s *gitSchema) Name() string {
	return "git"
}

func (s *gitSchema) Schema() string {
	return Git
}

func (s *gitSchema) Install() {
	dagql.Fields[*core.Query]{
		dagql.Func("git", s.git),
	}.Install(s.srv)

	dagql.Fields[*core.GitRepository]{
		dagql.Func("branch", s.branch),
		dagql.Func("tag", s.tag),
		dagql.Func("commit", s.commit),
	}.Install(s.srv)

	dagql.Fields[*core.GitRef]{
		dagql.Func("tree", s.tree),
		dagql.Func("commit", s.fetchCommit),
	}.Install(s.srv)
}

type gitArgs struct {
	URL                     string
	KeepGitDir              bool `default:"false"`
	ExperimentalServiceHost dagql.Optional[core.ServiceID]

	SSHKnownHosts string                        `name:"sshKnownHosts" default:""`
	SSHAuthSocket dagql.Optional[core.SocketID] `name:"sshAuthSocket"`
}

func (s *gitSchema) git(ctx context.Context, parent *core.Query, args gitArgs) (*core.GitRepository, error) {
	var svcs core.ServiceBindings
	if args.ExperimentalServiceHost.Valid {
		svc, err := args.ExperimentalServiceHost.Value.Load(ctx, s.srv)
		if err != nil {
			return nil, err
		}
		host, err := svc.Self.Hostname(ctx, svc.ID())
		if err != nil {
			return nil, err
		}
		svcs = append(svcs, core.ServiceBinding{
			ID:       svc.ID(),
			Service:  svc.Self,
			Hostname: host,
		})
	}
	var authSock *core.Socket
	if args.SSHAuthSocket.Valid {
		sock, err := args.SSHAuthSocket.Value.Load(ctx, s.srv)
		if err != nil {
			return nil, err
		}
		authSock = sock.Self
	}
	return &core.GitRepository{
		Query:         parent,
		URL:           args.URL,
		KeepGitDir:    args.KeepGitDir,
		SSHKnownHosts: args.SSHKnownHosts,
		SSHAuthSocket: authSock,
		Services:      svcs,
		Platform:      parent.Platform,
	}, nil
}

type commitArgs struct {
	ID string
}

func (s *gitSchema) commit(ctx context.Context, parent *core.GitRepository, args commitArgs) (*core.GitRef, error) {
	return &core.GitRef{
		Query: parent.Query,
		Ref:   args.ID,
		Repo:  parent,
	}, nil
}

type branchArgs struct {
	Name string
}

func (s *gitSchema) branch(ctx context.Context, parent *core.GitRepository, args branchArgs) (*core.GitRef, error) {
	return &core.GitRef{
		Query: parent.Query,
		Ref:   args.Name,
		Repo:  parent,
	}, nil
}

type tagArgs struct {
	Name string
}

func (s *gitSchema) tag(ctx context.Context, parent *core.GitRepository, args tagArgs) (*core.GitRef, error) {
	return &core.GitRef{
		Query: parent.Query,
		Ref:   args.Name,
		Repo:  parent,
	}, nil
}

type treeArgs struct {
	// FIXME(vito): support deprecated in dagql
	// SSHKnownHosts is deprecated
	SSHKnownHosts dagql.Optional[dagql.String] `name:"sshKnownHosts"`
	// SSHAuthSocket is deprecated
	SSHAuthSocket dagql.Optional[core.SocketID] `name:"sshAuthSocket"`
}

func (s *gitSchema) tree(ctx context.Context, parent *core.GitRef, args treeArgs) (*core.Directory, error) {
	var authSock *core.Socket
	if args.SSHAuthSocket.Valid {
		sock, err := args.SSHAuthSocket.Value.Load(ctx, s.srv)
		if err != nil {
			return nil, err
		}
		authSock = sock.Self
	}
	res := parent
	if args.SSHKnownHosts.Valid || args.SSHAuthSocket.Valid {
		// no need for a full clone() here, we're only modifying string fields
		cp := *res.Repo
		cp.SSHKnownHosts = args.SSHKnownHosts.GetOr("").String()
		cp.SSHAuthSocket = authSock
		res.Repo = &cp
	}
	return res.Tree(ctx)
}

func (s *gitSchema) fetchCommit(ctx context.Context, parent *core.GitRef, _ struct{}) (dagql.String, error) {
	str, err := parent.Commit(ctx)
	if err != nil {
		return "", err
	}
	return dagql.NewString(str), nil
}
