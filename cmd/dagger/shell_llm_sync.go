package main

import (
	"context"
	"fmt"

	"dagger.io/dagger"
	"github.com/dagger/dagger/engine/slog"
	"mvdan.cc/sh/v3/syntax"
)

// shellCallHandler implements LLMVarSyncer for the sh-based shell.

var _ LLMVarSyncer = (*shellCallHandler)(nil)

func (h *shellCallHandler) LLMVars(ctx context.Context) ([]LLMVar, error) {
	var vars []LLMVar
	for name, value := range h.runner.Vars {
		if name == agentVar || name == lastValueVar {
			continue
		}
		str := value.String()
		if key := GetStateKey(str); key != "" {
			st, err := h.state.Load(key)
			if err != nil {
				continue
			}
			q := st.QueryBuilder(h.dag)
			modDef := h.GetDef(st)
			typeDef, err := st.GetTypeDef(modDef)
			if err != nil {
				continue
			}
			if typeDef.AsFunctionProvider() != nil {
				var id string
				if err := q.Select("id").Bind(&id).Execute(ctx); err != nil {
					continue
				}
				vars = append(vars, LLMVar{
					Name:     name,
					TypeName: typeDef.Name(),
					Value:    id,
				})
			}
		} else {
			vars = append(vars, LLMVar{
				Name:  name,
				Value: str,
			})
		}
	}
	return vars, nil
}

func (h *shellCallHandler) SetLLMVar(ctx context.Context, name string, v LLMVar) error {
	if v.IsObject() {
		// Store as a shell state token
		st := ShellState{
			Calls: []FunctionCall{
				{
					Object: "Query",
					Name:   "load" + v.TypeName + "FromID",
					Arguments: map[string]any{
						"id": v.Value,
					},
					ReturnObject: v.TypeName,
				},
			},
		}
		key := h.state.Store(st)
		token := newStateToken(key)
		return h.Eval(ctx, fmt.Sprintf("%s=%s", name, token))
	}

	// String value
	if len(v.Value) > 100 {
		slog.Debug("value is too long", "name", name, "value", v.Value)
		return nil
	}
	quot, err := syntax.Quote(v.Value, syntax.LangBash)
	if err != nil {
		return fmt.Errorf("quote %q: %w", v.Value, err)
	}
	return h.Eval(ctx, fmt.Sprintf("%s=%s", name, quot))
}

func (h *shellCallHandler) GetAgentLLM(ctx context.Context, dag *dagger.Client) *dagger.LLM {
	value, ok := h.runner.Vars[agentVar]
	if !ok {
		return nil
	}
	key := GetStateKey(value.String())
	if key == "" {
		return nil
	}
	st, err := h.state.Load(key)
	if err != nil {
		return nil
	}
	return dag.LLM().WithGraphQLQuery(st.QueryBuilder(dag))
}

func (h *shellCallHandler) SetAgentLLM(ctx context.Context, llm *dagger.LLM) error {
	objID, err := llm.XXX_GraphQLID(ctx)
	if err != nil {
		return err
	}
	st := ShellState{
		Calls: []FunctionCall{
			{
				Object: "Query",
				Name:   "loadLLMFromID",
				Arguments: map[string]any{
					"id": objID,
				},
				ReturnObject: "LLM",
			},
		},
	}
	key := h.state.Store(st)
	token := newStateToken(key)
	return h.Eval(ctx, fmt.Sprintf("%s=%s", agentVar, token))
}
