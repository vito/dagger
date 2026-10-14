package core

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dagger/dagger/dagql"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/dagger/dagger/engine/slog"
)

func (llm *LLM) WarnToolNameCollisions(ctx context.Context) {
	if llm == nil || llm.mcp == nil {
		return
	}
	collisions, err := llm.mcp.ToolNameCollisions(ctx)
	if err != nil || len(collisions) == 0 {
		return
	}
	logger := slog.SpanLogger(ctx, InstrumentationLibrary)
	for name, contributors := range collisions {
		served := make([]string, len(contributors))
		for i, typeName := range contributors {
			served[i] = namespacedToolName(typeName, name)
		}
		logger.Warn("agent tool name collision: serving the conflicting toolsets under namespaced names",
			"tool", name, "contributors", contributors, "served", served)
	}
}

// Expertise retains a conversation-transforming function without evaluating it.
// The conversation and, when requested, its owning Agent are supplied at run time.
type Expertise struct{ Artifact *Artifact }

func (*Expertise) Type() *ast.Type         { return &ast.Type{NamedType: "Expertise", NonNull: true} }
func (*Expertise) TypeDescription() string { return "A function that can modify a conversation." }

func expertiseFunction(artifact *Artifact) *Function {
	if artifact.Node != nil && artifact.Node.Parent != nil {
		if obj := artifact.Node.Parent.ObjectType(); obj != nil {
			if fn, ok := obj.FunctionByName(artifact.Node.Name); ok {
				return fn
			}
		}
	}
	return nil
}

func NewExpertise(artifact *Artifact) (*Expertise, error) {
	fn := expertiseFunction(artifact)
	if fn == nil || !isExpertiseFunction(fn) || artifact.Node.OriginalModule.Self() == nil {
		uri, err := artifact.URI(ArtifactURIOpts{DimensionKeys: true})
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s is not a source of expertise: expected one LLM! argument, only auto-filled or optional additional arguments, and an LLM! return", uri)
	}
	return &Expertise{Artifact: artifact.Clone()}, nil
}
func (a *Expertise) Clone() *Expertise       { return &Expertise{Artifact: a.Artifact.Clone()} }
func (a *Expertise) Name() string            { return a.Artifact.Node.CommandName() }
func (a *Expertise) Description() string     { return a.Artifact.Node.Description }
func (a *Expertise) Path() []string          { return a.Artifact.Node.Path() }
func (a *Expertise) OriginalModule() *Module { return a.Artifact.Node.OriginalModule.Self() }

// Identity intentionally excludes the implementation and workspace revision.
func (a *Expertise) Identity() (string, error) {
	return a.Artifact.URI(ArtifactURIOpts{DimensionKeys: true})
}

func (a *Expertise) Run(ctx context.Context, base dagql.ObjectResult[*LLM]) (dagql.ObjectResult[*LLM], error) {
	var result dagql.ObjectResult[*LLM]
	key, err := a.Identity()
	if err != nil {
		return result, err
	}
	ctx = WithExpertiseOwner(ctx, key)
	ctx, err = a.Artifact.WorkspaceContext(ctx)
	if err != nil {
		return result, err
	}
	id, err := base.ID()
	if err != nil {
		return result, err
	}
	fn := expertiseFunction(a.Artifact)
	if fn == nil || !isExpertiseFunction(fn) {
		return result, fmt.Errorf("invalid expertise %q", a.Name())
	}
	var inputs []dagql.NamedInput
	for _, arg := range fn.Args {
		if isCoreLLMArg(arg.Self()) {
			inputs = append(inputs, dagql.NamedInput{Name: arg.Self().Name, Value: dagql.NewID[*LLM](id)})
		}
		if isCoreAgentArg(arg.Self()) && !arg.Self().TypeDef.Self().Optional {
			// The conversation owner is distinct from the caller during spawn.
			// A handle-only reference avoids embedding the seed in its own recipe.
			if base.Self().OwnerAgentHandle == "" {
				return result, fmt.Errorf("expertise %q requires an owning agent for argument %q; use LLM.spawn(expertise:) or an agent's snapshot", a.Name(), arg.Self().Name)
			}
			owner, err := AgentReference(ctx, base.Self().OwnerAgentHandle)
			if err != nil {
				return result, fmt.Errorf("expertise %q owning agent: %w", a.Name(), err)
			}
			ownerID, err := owner.ID()
			if err != nil {
				return result, err
			}
			inputs = append(inputs, dagql.NamedInput{Name: arg.Self().Name, Value: dagql.NewID[*Agent](ownerID)})
		}
	}
	if err := a.Artifact.Evaluate(ctx, &result, inputs...); err != nil {
		return result, err
	}
	// An entry may build a fresh conversation. Keep the owner before the next
	// entry runs, not just when spawn finalizes the seed.
	return WithLLMOwnerAgent(ctx, result, base.Self().OwnerAgentHandle)
}

func (a *Expertise) EncodePersistedObject(ctx context.Context, enc *dagql.PersistEncodeContext) (dagql.PersistedObjectEncoding, error) {
	return a.Artifact.EncodePersistedObject(ctx, enc)
}
func (*Expertise) DecodePersistedObject(ctx context.Context, dec *dagql.PersistDecodeContext, raw json.RawMessage) (dagql.Typed, error) {
	artifact, err := (*Artifact)(nil).DecodePersistedObject(ctx, dec, raw)
	if err != nil {
		return nil, err
	}
	return &Expertise{Artifact: artifact.(*Artifact)}, nil
}
func (a *Expertise) AttachDependencyResults(ctx context.Context, owner dagql.AnyResult, attach func(dagql.AnyResult) (dagql.AnyResult, error)) ([]dagql.AnyResult, error) {
	return a.Artifact.AttachDependencyResults(ctx, owner, attach)
}

// ComposeExpertise records only the outermost composition. Its nested entries
// are reproduced by rerunning that outer entry, rather than independently.
func ComposeExpertise(ctx context.Context, base dagql.ObjectResult[*LLM], expertise dagql.ObjectResultArray[*Expertise]) (dagql.ObjectResult[*LLM], error) {
	_, nested := ExpertiseOwner(ctx)
	record := slices.Clone(base.Self().Expertise)
	if !nested {
		seen := map[string]bool{}
		for _, entry := range record {
			key, err := entry.Self().Identity()
			if err != nil {
				return base, err
			}
			seen[key] = true
		}
		for _, entry := range expertise {
			key, err := entry.Self().Identity()
			if err != nil {
				return base, err
			}
			if seen[key] {
				return base, fmt.Errorf("expertise %q is already composed", entry.Self().Name())
			}
			seen[key] = true
			record = append(record, entry)
		}
	}
	acc := base
	for _, entry := range expertise {
		next, err := entry.Self().Run(ctx, acc)
		if err != nil {
			return base, fmt.Errorf("compose agent %q: %w", entry.Self().Name(), err)
		}
		acc = next
	}
	if !nested {
		var err error
		acc, err = RecordExpertise(ctx, acc, record)
		if err != nil {
			return base, err
		}
	}
	acc.Self().WarnToolNameCollisions(ctx)
	return acc, nil
}

// RecordExpertise uses a selector so checkpoint/portable replay retains the
// composition record even when the entry returned a separately built LLM.
func RecordExpertise(ctx context.Context, llm dagql.ObjectResult[*LLM], entries dagql.ObjectResultArray[*Expertise]) (dagql.ObjectResult[*LLM], error) {
	srv, err := CurrentDagqlServer(ctx)
	if err != nil {
		return llm, err
	}
	ids := make(dagql.ArrayInput[dagql.ID[*Expertise]], len(entries))
	for i, entry := range entries {
		id, err := entry.ID()
		if err != nil {
			return llm, err
		}
		ids[i] = dagql.NewID[*Expertise](id)
	}
	err = srv.Select(ctx, llm, &llm, dagql.Selector{Field: "__withExpertise", Args: []dagql.NamedInput{{Name: "expertise", Value: ids}}})
	return llm, err
}
