package main

import (
	"context"
	"fmt"

	"dagger.io/dagger"
	"github.com/Khan/genqlient/graphql"
	"github.com/dagger/dagger/engine/slog"
	"github.com/vito/dang/pkg/dang"
	"github.com/vito/dang/pkg/hm"
	"github.com/vito/dang/pkg/introspection"
	dangqb "github.com/vito/dang/pkg/querybuilder"
)

// dangShellHandler implements LLMVarSyncer for the Dang language.

var _ LLMVarSyncer = (*dangShellHandler)(nil)

func (h *dangShellHandler) LLMVars(ctx context.Context) ([]LLMVar, error) {
	h.mu.RLock()
	bindings := h.evalEnv.Bindings(dang.PublicVisibility)
	h.mu.RUnlock()

	var vars []LLMVar
	for _, binding := range bindings {
		name := binding.Key
		val := binding.Value

		if name == agentVar || name == lastValueVar {
			continue
		}

		// Check if it's a GraphQL value (Dagger object)
		if gqlVal, ok := val.(dang.GraphQLValue); ok {
			var id string
			err := gqlVal.QueryChain.Select("id").
				Client(gqlVal.Client).
				Bind(&id).
				Execute(ctx)
			if err != nil {
				slog.Debug("failed to get ID for LLM sync", "name", name, "error", err)
				continue
			}
			vars = append(vars, LLMVar{
				Name:     name,
				TypeName: gqlVal.TypeName,
				Value:    id,
			})
			continue
		}

		// Check if it's a string
		if strVal, ok := val.(dang.StringValue); ok {
			vars = append(vars, LLMVar{
				Name:  name,
				Value: strVal.Val,
			})
			continue
		}

		// Skip non-syncable types (functions, modules, etc.)
	}
	return vars, nil
}

func (h *dangShellHandler) SetLLMVar(ctx context.Context, name string, v LLMVar) error {
	if v.IsObject() {
		client := h.findGraphQLClient()
		if client == nil {
			slog.Debug("no GraphQL client available to set object var", "name", name)
			return nil
		}

		schema := h.findSchema()

		// Build the query chain using Dang's querybuilder
		qb := dangqb.Query().
			Select("load" + v.TypeName + "FromID").
			Arg("id", v.Value)

		gqlVal := dang.GraphQLValue{
			Name:       name,
			TypeName:   v.TypeName,
			Client:     client,
			Schema:     schema,
			QueryChain: qb,
		}

		// Try to resolve the type from the schema
		if schema != nil {
			typeEnv := dang.NewEnv(v.TypeName, schema)
			if namedEnv, exists := typeEnv.NamedType(v.TypeName); exists {
				gqlVal.ValType = dang.NonNull(namedEnv)
			}
		}
		if gqlVal.ValType == nil {
			gqlVal.ValType = hm.NewSimpleFresher().Fresh()
		}

		h.mu.Lock()
		h.evalEnv.Set(name, gqlVal)
		h.mu.Unlock()
		return nil
	}

	// String value
	h.mu.Lock()
	h.evalEnv.Set(name, dang.StringValue{Val: v.Value})
	h.mu.Unlock()
	return nil
}

func (h *dangShellHandler) GetAgentLLM(ctx context.Context, dag *dagger.Client) *dagger.LLM {
	h.mu.RLock()
	val, ok := h.evalEnv.Get(agentVar)
	h.mu.RUnlock()
	if !ok {
		return nil
	}
	gqlVal, ok := val.(dang.GraphQLValue)
	if !ok {
		return nil
	}
	// Get the ID from the Dang GraphQL value
	var id string
	err := gqlVal.QueryChain.Select("id").
		Client(gqlVal.Client).
		Bind(&id).
		Execute(ctx)
	if err != nil {
		slog.Debug("failed to get agent LLM ID", "error", err)
		return nil
	}
	// Use Dagger's querybuilder to construct the LLM
	return dag.LLM().WithGraphQLQuery(
		dag.QueryBuilder().Select("loadLLMFromID").Arg("id", id),
	)
}

func (h *dangShellHandler) SetAgentLLM(ctx context.Context, llm *dagger.LLM) error {
	objID, err := llm.XXX_GraphQLID(ctx)
	if err != nil {
		return err
	}

	client := h.findGraphQLClient()
	if client == nil {
		return fmt.Errorf("no GraphQL client available")
	}

	qb := dangqb.Query().
		Select("loadLLMFromID").
		Arg("id", objID)

	gqlVal := dang.GraphQLValue{
		Name:       agentVar,
		TypeName:   "LLM",
		Client:     client,
		Schema:     h.findSchema(),
		QueryChain: qb,
		ValType:    hm.NewSimpleFresher().Fresh(),
	}

	h.mu.Lock()
	h.evalEnv.Set(agentVar, gqlVal)
	h.mu.Unlock()
	return nil
}

// findGraphQLClient returns a GraphQL client from the handler's imports.
func (h *dangShellHandler) findGraphQLClient() graphql.Client {
	for _, ic := range h.importConfigs {
		if ic.Client != nil {
			return ic.Client
		}
	}
	return nil
}

// findSchema returns the first available schema from imports.
func (h *dangShellHandler) findSchema() *introspection.Schema {
	for _, ic := range h.importConfigs {
		if ic.Schema != nil {
			return ic.Schema
		}
	}
	return nil
}
