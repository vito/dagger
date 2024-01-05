package schema

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/containerd/containerd/content"
	"github.com/dagger/dagger/auth"
	"github.com/dagger/dagger/cmd/codegen/introspection"
	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/engine"
	"github.com/dagger/dagger/engine/buildkit"
	"github.com/dagger/graphql"
	"github.com/dagger/graphql/gqlerrors"
	"github.com/iancoleman/strcase"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/leaseutil"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vito/dagql"
)

type InitializeArgs struct {
	BuildkitClient *buildkit.Client
	Platform       specs.Platform
	ProgSockPath   string
	OCIStore       content.Store
	LeaseManager   *leaseutil.Manager
	Auth           *auth.RegistryAuthProvider
	Secrets        *core.SecretStore
}

type APIServer struct {
	// The root of the schema, housing all of the state and dependencies for the
	// server for easy access from descendent objects.
	root *core.Query
}

func New(ctx context.Context, params InitializeArgs) (*APIServer, error) {
	svcs := core.NewServices(params.BuildkitClient)

	root := core.NewRoot()
	root.Buildkit = params.BuildkitClient
	root.Services = svcs
	root.ProgrockSocketPath = params.ProgSockPath
	root.Platform = core.Platform(params.Platform)
	root.Secrets = params.Secrets
	root.OCIStore = params.OCIStore
	root.LeaseManager = params.LeaseManager
	root.Auth = params.Auth

	dag := dagql.NewServer(root)

	// stash away the cache so we can share it between other servers
	root.Cache = dag.Cache

	dag.Around(core.TelemetryFunc)

	coreMod := &CoreMod{dag: dag}
	if err := coreMod.Install(ctx, dag); err != nil {
		return nil, err
	}

	// the main client caller starts out with the core API loaded
	root.InstallDefaultClientContext(
		core.NewModDeps(root, []core.Mod{coreMod}),
	)

	return &APIServer{
		root: root,
	}, nil
}

func (s *APIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	errorOut := func(err error, code int) {
		bklog.G(ctx).WithError(err).Error("failed to serve request")
		http.Error(w, err.Error(), code)
	}

	clientMetadata, err := engine.ClientMetadataFromContext(ctx)
	if err != nil {
		errorOut(err, http.StatusInternalServerError)
		return
	}

	callContext, ok := s.root.ClientCallContext(clientMetadata.ModuleCallerDigest)
	if !ok {
		errorOut(fmt.Errorf("client call %s not found", clientMetadata.ModuleCallerDigest), http.StatusInternalServerError)
		return
	}

	schema, err := callContext.Deps.Schema(ctx)
	if err != nil {
		// TODO: technically this is not *always* bad request, should ideally be more specific and differentiate
		errorOut(err, http.StatusBadRequest)
		return
	}

	defer func() {
		if v := recover(); v != nil {
			bklog.G(context.TODO()).Errorf("panic serving schema: %v %s", v, string(debug.Stack()))

			// check whether this is a hijacked connection, if so we can't write any http errors to it
			_, err := w.Write(nil)
			if err == http.ErrHijacked {
				return
			}

			msg := "Internal Server Error"
			code := http.StatusInternalServerError
			switch v := v.(type) {
			case error:
				msg = v.Error()
				if errors.As(v, &InvalidInputError{}) {
					// panics can happen on invalid input in scalar serde
					code = http.StatusBadRequest
				}
			case string:
				msg = v
			}
			res := graphql.Result{
				Errors: []gqlerrors.FormattedError{
					gqlerrors.NewFormattedError(msg),
				},
			}
			bytes, err := json.Marshal(res)
			if err != nil {
				panic(err)
			}
			http.Error(w, string(bytes), code)
		}
	}()

	srv := handler.NewDefaultServer(schema)
	// srv.AroundResponses(func(ctx context.Context, next gqlnew.ResponseHandler) *gqlnew.Response {
	// 	res := next(ctx)
	// 	pl, err := json.Marshal(res)
	// 	log.Println("!!! GRAPHQL RESPONSE", string(pl), err)
	// 	return res
	// })
	mux := http.NewServeMux()
	mux.Handle("/query", srv)
	mux.Handle("/shutdown", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		bklog.G(ctx).Debugf("shutting down client %s", clientMetadata.ClientID)
		if err := s.root.Services.StopClientServices(ctx, clientMetadata); err != nil {
			bklog.G(ctx).WithError(err).Error("failed to shutdown")
		}
	}))

	s.root.MuxEndpoints(mux)

	var handler http.Handler = mux
	handler = flushAfterNBytes(buildkit.MaxFileContentsChunkSize)(handler)
	handler.ServeHTTP(w, r)
}

func (s *APIServer) ShutdownClient(ctx context.Context, client *engine.ClientMetadata) error {
	return s.root.Services.StopClientServices(ctx, client)
}

func (s *APIServer) CurrentServedDeps(ctx context.Context) (*core.ModDeps, error) {
	clientMetadata, err := engine.ClientMetadataFromContext(ctx)
	if err != nil {
		return nil, err
	}
	callCtx, ok := s.root.ClientCallContext(clientMetadata.ModuleCallerDigest)
	if !ok {
		return nil, fmt.Errorf("client call %s not found", clientMetadata.ModuleCallerDigest)
	}
	return callCtx.Deps, nil
}

type CompiledSchema struct {
	SchemaResolvers
	Compiled *graphql.Schema
}

type SchemaResolvers interface {
	Install()
}

func schemaIntrospectionJSON(ctx context.Context, dag *dagql.Server) (json.RawMessage, error) {
	data, err := dag.Query(ctx, introspection.Query, nil)
	if err != nil {
		return nil, fmt.Errorf("introspection query failed: %w", err)
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal introspection result: %w", err)
	}
	return json.RawMessage(jsonBytes), nil
}

// getSourceContext is a little helper to extract a target line with a number
// of lines of surrounding context. If surrounding is negative, then all the
// lines will be returned.
func getSourceContext(input string, target int, surrounding int) string {
	removeLines := surrounding > 0

	output := strings.Builder{}
	scanner := bufio.NewScanner(strings.NewReader(input))

	padding := len(fmt.Sprint(target + surrounding))

	count := 0
	if removeLines && target-surrounding > 1 {
		output.WriteString(fmt.Sprintf(" %*s | ...\n", padding, ""))
	}
	for scanner.Scan() {
		count += 1
		if removeLines && (count < target-surrounding || count > target+surrounding) {
			continue
		}
		output.WriteString(fmt.Sprintf(" %*d | ", padding, count))
		output.WriteString(scanner.Text())
		output.WriteString("\n")
	}
	if removeLines && target+surrounding < count {
		output.WriteString(fmt.Sprintf(" %*s | ...\n", padding, ""))
	}
	return output.String()
}

/*
This formats comments in the schema as:
"""
comment
"""

Which avoids corner cases where the comment ends in a `"`.
*/
func formatGqlDescription(desc string, args ...any) string {
	if desc == "" {
		return ""
	}
	return "\n" + strings.TrimSpace(fmt.Sprintf(desc, args...)) + "\n"
}

func typeDefToASTType(typeDef *core.TypeDef, isInput bool) (*ast.Type, error) {
	switch typeDef.Kind {
	case core.TypeDefKindString:
		return &ast.Type{
			NamedType: "String",
			NonNull:   !typeDef.Optional,
		}, nil
	case core.TypeDefKindInteger:
		return &ast.Type{
			NamedType: "Int",
			NonNull:   !typeDef.Optional,
		}, nil
	case core.TypeDefKindBoolean:
		return &ast.Type{
			NamedType: "Boolean",
			NonNull:   !typeDef.Optional,
		}, nil
	case core.TypeDefKindVoid:
		return &ast.Type{
			NamedType: "Void",
			NonNull:   !typeDef.Optional,
		}, nil
	case core.TypeDefKindList:
		if !typeDef.AsList.Valid {
			return nil, fmt.Errorf("expected list type def, got nil")
		}
		astType, err := typeDefToASTType(typeDef.AsList.Value.ElementTypeDef, isInput)
		if err != nil {
			return nil, err
		}
		return &ast.Type{
			Elem:    astType,
			NonNull: !typeDef.Optional,
		}, nil
	case core.TypeDefKindObject:
		if !typeDef.AsObject.Valid {
			return nil, fmt.Errorf("expected object type def, got nil")
		}
		objTypeDef := typeDef.AsObject
		objName := gqlObjectName(objTypeDef.Value.Name)
		if isInput {
			// idable types use their ID scalar as the input value
			return &ast.Type{NamedType: objName + "ID", NonNull: !typeDef.Optional}, nil
		}
		return &ast.Type{NamedType: objName, NonNull: !typeDef.Optional}, nil
	case core.TypeDefKindInterface:
		if !typeDef.AsInterface.Valid {
			return nil, fmt.Errorf("expected interface type def, got nil")
		}
		ifaceTypeDef := typeDef.AsInterface
		ifaceName := gqlObjectName(ifaceTypeDef.Value.Name)
		if isInput {
			// idable types use their ID scalar as the input value
			return &ast.Type{NamedType: ifaceName + "ID", NonNull: !typeDef.Optional}, nil
		}
		return &ast.Type{NamedType: ifaceName, NonNull: !typeDef.Optional}, nil
	default:
		return nil, fmt.Errorf("unsupported type kind %q", typeDef.Kind)
	}
}

// relevant ast code we need to work with here:
// https://github.com/vektah/gqlparser/blob/35199fce1fa1b73c27f23c84f4430f47ac93329e/ast/value.go#L44
func astDefaultValue(typeDef *core.TypeDef, val any) (*ast.Value, error) {
	if val == nil {
		// no default value for this arg
		return nil, nil
	}
	switch typeDef.Kind {
	case core.TypeDefKindString:
		strVal, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("expected string default value, got %T", val)
		}
		return &ast.Value{
			Kind: ast.StringValue,
			Raw:  strVal,
		}, nil
	case core.TypeDefKindInteger:
		var intVal int
		switch val := val.(type) {
		case int:
			intVal = val
		case float64: // JSON unmarshaling to `any'
			intVal = int(val)
		default:
			return nil, fmt.Errorf("expected integer default value, got %T", val)
		}
		return &ast.Value{
			Kind: ast.IntValue,
			Raw:  strconv.Itoa(intVal),
		}, nil
	case core.TypeDefKindBoolean:
		boolVal, ok := val.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool default value, got %T", val)
		}
		return &ast.Value{
			Kind: ast.BooleanValue,
			Raw:  strconv.FormatBool(boolVal),
		}, nil
	case core.TypeDefKindVoid:
		if val != nil {
			return nil, fmt.Errorf("expected nil value, got %T", val)
		}
		return &ast.Value{
			Kind: ast.NullValue,
			Raw:  "null",
		}, nil
	case core.TypeDefKindList:
		astVal := &ast.Value{Kind: ast.ListValue}
		// val is coming from deserializing a json string (see jsonResolver), so it should be []any
		listVal, ok := val.([]any)
		if !ok {
			return nil, fmt.Errorf("expected list default value, got %T", val)
		}
		for _, elemVal := range listVal {
			elemASTVal, err := astDefaultValue(typeDef.AsList.Value.ElementTypeDef, elemVal)
			if err != nil {
				return nil, fmt.Errorf("failed to get default value for list element: %w", err)
			}
			astVal.Children = append(astVal.Children, &ast.ChildValue{
				Value: elemASTVal,
			})
		}
		return astVal, nil
	case core.TypeDefKindObject:
		astVal := &ast.Value{Kind: ast.ObjectValue}
		// val is coming from deserializing a json string (see jsonResolver), so it should be map[string]any
		mapVal, ok := val.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object default value, got %T", val)
		}
		for name, val := range mapVal {
			name = gqlFieldName(name)
			field, ok := typeDef.AsObject.Value.FieldByName(name)
			if !ok {
				return nil, fmt.Errorf("object field %s.%s not found", typeDef.AsObject.Value.Name, name)
			}
			fieldASTVal, err := astDefaultValue(field.TypeDef, val)
			if err != nil {
				return nil, fmt.Errorf("failed to get default value for object field %q: %w", name, err)
			}
			astVal.Children = append(astVal.Children, &ast.ChildValue{
				Name:  name,
				Value: fieldASTVal,
			})
		}
		return astVal, nil
	default:
		return nil, fmt.Errorf("unsupported type kind %q", typeDef.Kind)
	}
}

func gqlObjectName(name string) string {
	// gql object name is capitalized camel case
	return strcase.ToCamel(name)
}

func namespaceObject(objName, namespace string) string {
	gqlObjName := gqlObjectName(objName)
	if rest := strings.TrimPrefix(gqlObjName, gqlObjectName(namespace)); rest != gqlObjName {
		if len(rest) == 0 {
			// objName equals namespace, don't namespace this
			return gqlObjName
		}
		// we have this case check here to check for a boundary
		// e.g. if objName="Postman" and namespace="Post", then we should still namespace
		// this to "PostPostman" instead of just going for "Postman" (but we should do that
		// if objName="PostMan")
		if 'A' <= rest[0] && rest[0] <= 'Z' {
			// objName has namespace prefixed, don't namespace this
			return gqlObjName
		}
	}

	return gqlObjectName(namespace + "_" + objName)
}

func gqlFieldName(name string) string {
	// gql field name is uncapitalized camel case
	return strcase.ToLowerCamel(name)
}

func gqlArgName(name string) string {
	// gql arg name is uncapitalized camel case
	return strcase.ToLowerCamel(name)
}
