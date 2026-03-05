package main

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"dagger.io/dagger"
	"github.com/dagger/dagger/engine/slog"
	"github.com/dagger/dagger/util/hashutil"
	telemetry "github.com/dagger/otel-go"
	"github.com/opencontainers/go-digest"
)

// LLMVarSyncer is the interface for syncing variables between a shell handler
// and the LLM session. Both the sh-based shell handler and the Dang handler
// implement this to allow the LLM to read and write their bindings.
type LLMVarSyncer interface {
	// LLMVars returns all user-defined variables suitable for syncing
	// to the LLM environment. Each var is either a string or a Dagger
	// object (identified by type name and GraphQL ID).
	LLMVars(ctx context.Context) ([]LLMVar, error)

	// SetLLMVar assigns a variable received from the LLM back into the
	// handler's environment.
	SetLLMVar(ctx context.Context, name string, v LLMVar) error

	// GetAgentLLM returns the LLM object stored in the handler's "agent"
	// variable, if any. This allows the LLM session to resume from a
	// previously stored state. Returns nil if no agent is set.
	GetAgentLLM(ctx context.Context, dag *dagger.Client) *dagger.LLM

	// SetAgentLLM stores the LLM object in the handler's "agent" variable.
	SetAgentLLM(ctx context.Context, llm *dagger.LLM) error
}

// LLMVar represents a variable that can be synced between a handler and the LLM.
type LLMVar struct {
	Name     string
	TypeName string // Dagger type name (e.g. "Container"), or "" for strings
	Value    string // String value, or GraphQL ID for Dagger objects
}

// IsObject returns true if this var represents a Dagger object (not a string).
func (v LLMVar) IsObject() bool {
	return v.TypeName != ""
}

// Digest returns a content hash for change detection.
func (v LLMVar) Digest() digest.Digest {
	return hashutil.HashStrings(v.TypeName, v.Value)
}

// truncateValue truncates a string for display in span names.
func truncateValue(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// syncVarsToLLM syncs handler variables to the LLM environment.
func (s *LLMSession) syncVarsToLLM() error {
	if s.varSyncer == nil {
		return nil
	}

	ctx := s.plumbingCtx

	_, outerSpan := Tracer().Start(ctx, "sync vars → LLM", telemetry.Reveal())
	defer outerSpan.End()

	// Check for agent var first
	if agentLLM := s.varSyncer.GetAgentLLM(ctx, s.dag); agentLLM != nil {
		s.llm = agentLLM
	}

	if s.beforeFS == nil {
		s.beforeFS = s.llm.Env().Workspace()
		s.beforeFSTime = time.Now()
	}

	// Get all vars from the handler
	vars, err := s.varSyncer.LLMVars(ctx)
	if err != nil {
		return err
	}

	oldVars := s.syncedVars
	s.syncedVars = make(map[string]digest.Digest)
	for k, v := range oldVars {
		s.syncedVars[k] = v
	}

	syncedEnvQ := s.dag.QueryBuilder().
		Select("loadEnvFromID").
		Arg("id", s.llm.Env())

	var changed bool
	for _, v := range vars {
		if s.syncedVars[v.Name] == v.Digest() {
			continue
		}

		changed = true

		if v.IsObject() {
			_, varSpan := Tracer().Start(ctx,
				fmt.Sprintf("var → LLM: %s (%s)", v.Name, v.TypeName),
				telemetry.Reveal(),
			)
			varSpan.SetAttributes(
				attribute.String("var.name", v.Name),
				attribute.String("var.type", v.TypeName),
				attribute.String("var.id", truncateValue(v.Value, 80)),
			)

			syncedEnvQ = syncedEnvQ.
				Select("with"+v.TypeName+"Input").
				Arg("name", v.Name).
				Arg("description", "").
				Arg("value", v.Value)
			d, err := idDigest(v.Value)
			if err != nil {
				varSpan.End()
				return err
			}
			s.syncedVars[v.Name] = d
			varSpan.End()
		} else {
			_, varSpan := Tracer().Start(ctx,
				fmt.Sprintf("var → LLM: %s = %s", v.Name, truncateValue(v.Value, 40)),
				telemetry.Reveal(),
			)
			varSpan.SetAttributes(
				attribute.String("var.name", v.Name),
				attribute.String("var.value", truncateValue(v.Value, 200)),
			)

			s.syncedVars[v.Name] = v.Digest()
			syncedEnvQ = syncedEnvQ.
				Select("withStringInput").
				Arg("name", v.Name).
				Arg("description", "").
				Arg("value", v.Value)
			varSpan.End()
		}
	}

	if !changed {
		outerSpan.SetAttributes(attribute.Bool("noop", true))
		return nil
	}

	var envID dagger.EnvID
	if err := syncedEnvQ.Select("id").Bind(&envID).Execute(ctx); err != nil {
		return err
	}
	s.updateLLMAndAgentVar(s.llm.WithEnv(s.dag.LoadEnvFromID(envID)))
	return nil
}

// syncVarsFromLLM syncs LLM environment outputs back to the handler.
func (s *LLMSession) syncVarsFromLLM() error {
	if s.varSyncer == nil {
		return nil
	}

	ctx := s.plumbingCtx

	_, outerSpan := Tracer().Start(ctx, "sync vars ← LLM", telemetry.Reveal())
	defer outerSpan.End()

	outputs, err := s.llm.Env().Outputs(ctx)
	if err != nil {
		return err
	}

	outerSpan.SetAttributes(attribute.Int("output.count", len(outputs)))

	assign := func(bnd *dagger.Binding) error {
		name, err := bnd.Name(ctx)
		if err != nil {
			return err
		}
		typeName, err := bnd.TypeName(ctx)
		if err != nil {
			return err
		}
		isNull, err := bnd.IsNull(ctx)
		if err != nil {
			return err
		}
		if isNull {
			return nil
		}
		switch typeName {
		case "", "Query", "Void":
			return nil
		case "String":
			str, err := bnd.AsString(ctx)
			if err != nil {
				return err
			}

			_, varSpan := Tracer().Start(ctx,
				fmt.Sprintf("var ← LLM: %s = %s", name, truncateValue(str, 40)),
				telemetry.Reveal(),
			)
			varSpan.SetAttributes(
				attribute.String("var.name", name),
				attribute.String("var.type", "String"),
				attribute.String("var.value", truncateValue(str, 200)),
			)
			err = s.varSyncer.SetLLMVar(ctx, name, LLMVar{
				Name:  name,
				Value: str,
			})
			varSpan.End()
			return err
		default:
			var objID string
			if err :=
				s.dag.QueryBuilder().
					Select("loadBindingFromID").
					Arg("id", bnd).
					Select("as"+typeName).
					Select("id").
					Bind(&objID).
					Execute(ctx); err != nil {
				return err
			}

			_, varSpan := Tracer().Start(ctx,
				fmt.Sprintf("var ← LLM: %s (%s)", name, typeName),
				telemetry.Reveal(),
			)
			varSpan.SetAttributes(
				attribute.String("var.name", name),
				attribute.String("var.type", typeName),
				attribute.String("var.id", truncateValue(objID, 80)),
			)
			err := s.varSyncer.SetLLMVar(ctx, name, LLMVar{
				Name:     name,
				TypeName: typeName,
				Value:    objID,
			})
			varSpan.End()
			return err
		}
	}

	for _, output := range outputs {
		if err := assign(&output); err != nil {
			return err
		}
	}

	return assign(s.llm.BindResult(lastValueVar))
}

// updateLLMAndAgentVar updates the LLM and syncs it back to the handler's
// agent variable.
func (s *LLMSession) updateLLMAndAgentVar(llm *dagger.LLM) error {
	s.llm = llm

	ctx := s.plumbingCtx

	model, err := s.llm.Model(ctx)
	if err != nil {
		return err
	}
	s.model = model

	if s.varSyncer != nil {
		if err := s.varSyncer.SetAgentLLM(ctx, s.llm); err != nil {
			slog.Error("failed to set agent LLM", "error", err)
		}
	}
	return nil
}
