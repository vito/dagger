package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/dagger/dagger/auth"
	"github.com/dagger/dagger/core/pipeline"
	"github.com/dagger/dagger/engine"
	"github.com/dagger/dagger/engine/buildkit"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/opencontainers/go-digest"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vito/dagql"
	"github.com/vito/dagql/idproto"
)

// Query forms the root of the DAG and houses all necessary state and
// dependencies for evaluating queries.
type Query struct {
	Buildkit *buildkit.Client

	ProgrockSocketPath string

	Services *Services

	Secrets *SecretStore

	Auth *auth.RegistryAuthProvider

	OCIStore     content.Store
	LeaseManager *leaseutil.Manager

	// The default platform.
	Platform Platform

	// The current pipeline.
	Pipeline pipeline.Path `json:"pipeline"`

	// The default deps of every user module (currently just core)
	DefaultDeps *ModDeps

	// The DagQL query cache.
	Cache dagql.Cache

	// The metadata of client calls.
	// For the special case of the main client caller, the key is just empty string.
	// This is never explicitly deleted from; instead it will just be garbage collected
	// when this server for the session shuts down
	clientCallContext map[digest.Digest]*ClientCallContext
	clientCallMu      *sync.RWMutex

	// the http endpoints being served (as a map since APIs like shellEndpoint can add more)
	endpoints  map[string]http.Handler
	endpointMu *sync.RWMutex
}

func NewRoot() *Query {
	return &Query{
		clientCallContext: map[digest.Digest]*ClientCallContext{},
		clientCallMu:      &sync.RWMutex{},
		endpoints:         map[string]http.Handler{},
		endpointMu:        &sync.RWMutex{},
	}
}

func (q *Query) MuxEndpoint(ctx context.Context, path string, handler http.Handler) error {
	q.endpointMu.Lock()
	defer q.endpointMu.Unlock()
	q.endpoints[path] = handler
	return nil
}

func (q *Query) MuxEndpoints(mux *http.ServeMux) {
	q.endpointMu.RLock()
	defer q.endpointMu.RUnlock()
	for path, handler := range q.endpoints {
		mux.Handle(path, handler)
	}
}

type ClientCallContext struct {
	// the DAG of modules being served to this client
	Deps *ModDeps

	// If the client is itself from a function call in a user module, these are set with the
	// metadata of that ongoing function call
	ModID  *idproto.ID
	FnCall *FunctionCall
}

func (q *Query) ClientCallContext(clientDigest digest.Digest) (*ClientCallContext, bool) {
	q.clientCallMu.RLock()
	defer q.clientCallMu.RUnlock()
	ctx, ok := q.clientCallContext[clientDigest]
	return ctx, ok
}

func (q *Query) InstallDefaultClientContext(deps *ModDeps) {
	q.clientCallMu.Lock()
	defer q.clientCallMu.Unlock()

	q.DefaultDeps = deps

	q.clientCallContext[""] = &ClientCallContext{
		Deps: deps,
	}
}

func (s *Query) ServeModuleToMainClient(ctx context.Context, modMeta dagql.Instance[*Module]) error {
	clientMetadata, err := engine.ClientMetadataFromContext(ctx)
	if err != nil {
		return err
	}
	if clientMetadata.ModuleCallerDigest != "" {
		return fmt.Errorf("cannot serve module to client %s", clientMetadata.ClientID)
	}

	mod := modMeta.Self

	s.clientCallMu.Lock()
	defer s.clientCallMu.Unlock()
	callCtx, ok := s.clientCallContext[""]
	if !ok {
		return fmt.Errorf("client call not found")
	}
	callCtx.Deps = callCtx.Deps.Append(mod)
	return nil
}

func (s *Query) RegisterFunctionCall(dgst digest.Digest, deps *ModDeps, modID *idproto.ID, call *FunctionCall) error {
	if dgst == "" {
		return fmt.Errorf("cannot register function call with empty digest")
	}

	s.clientCallMu.Lock()
	defer s.clientCallMu.Unlock()
	_, ok := s.clientCallContext[dgst]
	if ok {
		return nil
	}
	s.clientCallContext[dgst] = &ClientCallContext{
		Deps:   deps,
		ModID:  modID,
		FnCall: call,
	}
	return nil
}

func (s *Query) CurrentModule(ctx context.Context) (dagql.ID[*Module], error) {
	var id dagql.ID[*Module]
	clientMetadata, err := engine.ClientMetadataFromContext(ctx)
	if err != nil {
		return id, err
	}
	metaJSON, err := json.Marshal(clientMetadata)
	log.Println("!!! CLIENT METADATA", string(metaJSON), err)
	if clientMetadata.ModuleCallerDigest == "" {
		return id, fmt.Errorf("no current module for main client caller")
	}

	s.clientCallMu.RLock()
	defer s.clientCallMu.RUnlock()
	callCtx, ok := s.clientCallContext[clientMetadata.ModuleCallerDigest]
	if !ok {
		return id, fmt.Errorf("client call %s not found", clientMetadata.ModuleCallerDigest)
	}
	return dagql.NewID[*Module](callCtx.ModID), nil
}

func (s *Query) CurrentFunctionCall(ctx context.Context) (*FunctionCall, error) {
	clientMetadata, err := engine.ClientMetadataFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if clientMetadata.ModuleCallerDigest == "" {
		return nil, fmt.Errorf("no current function call for main client caller")
	}

	s.clientCallMu.RLock()
	defer s.clientCallMu.RUnlock()
	callCtx, ok := s.clientCallContext[clientMetadata.ModuleCallerDigest]
	if !ok {
		return nil, fmt.Errorf("client call %s not found", clientMetadata.ModuleCallerDigest)
	}

	return callCtx.FnCall, nil
}

func (*Query) Type() *ast.Type {
	return &ast.Type{
		NamedType: "Query",
		NonNull:   true,
	}
}

func (q *Query) Clone() *Query {
	cp := *q
	return &cp
}

// FIXME(vito) retire, not needed anymore
func (query *Query) PipelinePath() pipeline.Path {
	if query == nil {
		return nil
	}
	return query.Pipeline
}

func (query *Query) NewContainer(platform Platform) *Container {
	return &Container{
		Query:    query,
		Platform: platform,
		Pipeline: query.Pipeline,
	}
}

func (query *Query) NewSecret(name string) *Secret {
	return &Secret{
		Query: query,
		Name:  name,
	}
}

func (query *Query) NewHost() *Host {
	return &Host{
		Query: query,
	}
}

func (query *Query) NewModule() *Module {
	return &Module{
		Query: query,
	}
}

func (query *Query) NewContainerService(ctr *Container) *Service {
	return &Service{
		Query:     query,
		Container: ctr,
	}
}

func (query *Query) NewTunnelService(upstream dagql.Instance[*Service], ports []PortForward) *Service {
	return &Service{
		Query:          query,
		TunnelUpstream: &upstream,
		TunnelPorts:    ports,
	}
}

func (query *Query) NewHostService(upstream string, ports []PortForward) *Service {
	return &Service{
		Query:        query,
		HostUpstream: upstream,
		HostPorts:    ports,
	}
}
