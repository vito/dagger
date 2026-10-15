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
		if isCoreAgentArg(arg.Self()) {
			// Every Agent argument receives the agent composing the
			// conversation, never the caller. A handle-only reference avoids
			// embedding the seed in its own recipe.
			optional := arg.Self().TypeDef.Self().Optional
			if base.Self().OwnerAgentHandle == "" {
				if optional {
					continue
				}
				return result, fmt.Errorf("expertise %q requires an owning agent for argument %q; compose it with LLM.spawn(expertise:) or Agent.compose", a.Name(), arg.Self().Name)
			}
			owner, err := AgentReference(ctx, base.Self().OwnerAgentHandle)
			if err != nil {
				return result, fmt.Errorf("expertise %q owning agent: %w", a.Name(), err)
			}
			ownerID, err := owner.ID()
			if err != nil {
				return result, err
			}
			var value dagql.Input = dagql.NewID[*Agent](ownerID)
			if optional {
				value = dagql.Opt(dagql.NewID[*Agent](ownerID))
			}
			inputs = append(inputs, dagql.NamedInput{Name: arg.Self().Name, Value: value})
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

// ComposeExpertise runs the given entries in list order. At top level they are
// recorded: an entry already recorded under the same identity is refreshed in
// place, replacing its contributions and preserving compatible tool state,
// while other recorded entries are untouched; new entries are appended. While
// an entry runs, composed entries belong to that outer entry and are not
// recorded: rerunning the outer entry reproduces them.
func ComposeExpertise(ctx context.Context, base dagql.ObjectResult[*LLM], expertise dagql.ObjectResultArray[*Expertise]) (dagql.ObjectResult[*LLM], error) {
	if _, nested := ExpertiseOwner(ctx); nested {
		acc := base
		for _, entry := range expertise {
			next, err := entry.Self().Run(ctx, acc)
			if err != nil {
				return base, fmt.Errorf("compose agent %q: %w", entry.Self().Name(), err)
			}
			acc = next
		}
		acc.Self().WarnToolNameCollisions(ctx)
		return acc, nil
	}
	srv, err := CurrentDagqlServer(ctx)
	if err != nil {
		return base, err
	}
	record := slices.Clone(base.Self().Expertise)
	positions := map[string]int{}
	for i, entry := range record {
		key, err := entry.Self().Identity()
		if err != nil {
			return base, err
		}
		positions[key] = i
	}
	acc := base
	refreshed := false
	cleared := map[string]bool{}
	// Each identity runs once, at its first position in the list, as the last
	// entry given for it.
	var order []string
	run := map[string]dagql.ObjectResult[*Expertise]{}
	for _, entry := range expertise {
		key, err := entry.Self().Identity()
		if err != nil {
			return base, err
		}
		if _, seen := run[key]; !seen {
			order = append(order, key)
		}
		run[key] = entry
		i, recorded := positions[key]
		if !recorded {
			positions[key] = len(record)
			record = append(record, entry)
			continue
		}
		record[i] = entry
		if i < len(base.Self().Expertise) && !cleared[key] {
			// Clear a recorded entry's contributions before any entry runs.
			acc, err = withoutComposition(ctx, srv, acc, key)
			if err != nil {
				return base, err
			}
			cleared[key] = true
			refreshed = true
		}
	}
	for _, key := range order {
		entry := run[key]
		next, err := entry.Self().Run(ctx, acc)
		if err != nil {
			return base, fmt.Errorf("compose agent %q: %w", entry.Self().Name(), err)
		}
		acc = next
	}
	if refreshed {
		acc, err = preserveRecomposedTools(ctx, srv, base, acc)
		if err != nil {
			return base, err
		}
	}
	acc, err = RecordExpertise(ctx, acc, record)
	if err != nil {
		return base, err
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
