package core

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dagger/dagger/dagql"
	"github.com/dagger/dagger/engine/slog"
)

// RecomposeExpertise refreshes the whole composition record. Every recorded
// entry is found again by its address in the bound workspace, or reruns from
// its recorded source when that workspace does not address it. All recorded
// entries' owned contributions are replaced, preserving the state of their
// tool objects while loading new implementations. Unowned prompts and
// bindings are retained. Every replacement is recorded as a real selector,
// including the module-state rebind, rather than an in-memory change to an
// old object's class.
//
// Recomposition is a top-level operation: an entry composes other expertise
// with ComposeExpertise, which makes it part of that entry.
func RecomposeExpertise(ctx context.Context, base dagql.ObjectResult[*LLM]) (dagql.ObjectResult[*LLM], error) {
	if owner, nested := ExpertiseOwner(ctx); nested {
		return base, fmt.Errorf("cannot recompose while expertise %q runs; compose the entries instead", owner)
	}
	recorded := base.Self().Expertise
	if len(recorded) == 0 {
		return base, errors.New("conversation has no recorded expertise to recompose")
	}
	acc := base
	srv, err := CurrentDagqlServer(ctx)
	if err != nil {
		return base, err
	}
	record, err := refreshRecordedExpertise(ctx, srv, base.Self().Workspace(), recorded)
	if err != nil {
		return base, err
	}
	for _, entry := range recorded {
		key, err := entry.Self().Identity()
		if err != nil {
			return base, err
		}
		// Clear every recorded entry's contributions before running any.
		acc, err = withoutComposition(ctx, srv, acc, key)
		if err != nil {
			return base, err
		}
	}
	for _, entry := range record {
		next, err := entry.Self().Run(ctx, acc)
		if err != nil {
			return base, fmt.Errorf("recompose agent %q: %w", entry.Self().Name(), err)
		}
		acc = next
	}
	acc, err = preserveRecomposedTools(ctx, srv, base, acc)
	if err != nil {
		return base, err
	}
	acc, err = RecordExpertise(ctx, acc, record)
	if err != nil {
		return base, err
	}
	acc.Self().WarnToolNameCollisions(ctx)
	return acc, nil
}

// refreshRecordedExpertise finds each recorded entry again by its address,
// with dimension keys, in the conversation's bound workspace. An entry the
// workspace does not address (e.g. after switching to another repository)
// keeps its recorded artifact and source. A module of the entry that fails to
// load in the workspace is an error rather than a silent fallback.
func refreshRecordedExpertise(ctx context.Context, srv *dagql.Server, ws dagql.ObjectResult[*Workspace], record dagql.ObjectResultArray[*Expertise]) (dagql.ObjectResultArray[*Expertise], error) {
	refreshed := slices.Clone(record)
	if len(record) == 0 || ws.Self() == nil {
		return refreshed, nil
	}
	srv = srv.Canonical()
	var scope dagql.ObjectResult[*Artifacts]
	if err := srv.Select(ctx, ws, &scope, dagql.Selector{View: srv.View, Field: "artifacts"}); err != nil {
		return nil, fmt.Errorf("workspace artifacts: %w", err)
	}
	for i, entry := range record {
		next, err := refindExpertise(ctx, srv, scope, entry)
		if err != nil {
			return nil, fmt.Errorf("refresh expertise %q: %w", entry.Self().Name(), err)
		}
		refreshed[i] = next
	}
	return refreshed, nil
}

func refindExpertise(ctx context.Context, srv *dagql.Server, scope dagql.ObjectResult[*Artifacts], entry dagql.ObjectResult[*Expertise]) (dagql.ObjectResult[*Expertise], error) {
	recorded := entry.Self().Artifact
	key, err := entry.Self().Identity()
	if err != nil {
		return entry, err
	}
	// Check the schema path first: filtering by an address whose dimensions
	// this workspace lacks is an error, not an empty selection.
	addressed := false
	for _, candidate := range scope.Self().Entries {
		if candidate.ModuleName != recorded.ModuleName {
			continue
		}
		if candidate.LoadFailure != nil {
			return entry, errors.New(candidate.LoadFailure.Message)
		}
		if slices.Equal(candidate.Path, recorded.Path) {
			addressed = true
		}
	}
	if !addressed {
		return entry, nil
	}
	var found dagql.ObjectResultArray[*Expertise]
	if err := srv.Select(ctx, scope, &found,
		dagql.Selector{View: srv.View, Field: "filterUri", Args: []dagql.NamedInput{{Name: "uri", Value: dagql.String(key)}}},
		dagql.Selector{View: srv.View, Field: "asExpertise"},
	); err != nil {
		return entry, err
	}
	for _, candidate := range found {
		if id, err := candidate.Self().Identity(); err == nil && id == key {
			return candidate, nil
		}
	}
	// The path exists but not this entry's keys, e.g. a removed collection item.
	return entry, nil
}

// withoutComposition removes one entry's owned contributions, as a recorded
// selector so replay reproduces the clearing.
func withoutComposition(ctx context.Context, srv *dagql.Server, llm dagql.ObjectResult[*LLM], key string) (dagql.ObjectResult[*LLM], error) {
	err := srv.Select(ctx, llm, &llm, dagql.Selector{
		Field: "__withoutComposition",
		Args:  []dagql.NamedInput{{Name: "owner", Value: dagql.String(key)}},
	})
	return llm, err
}

// compositionOwnerMatches compares entry identities. Empty owners are always
// unowned, never an entry selected for recomposition.
func compositionOwnerMatches(contribution, owner string) bool {
	return owner != "" && contribution == owner
}

func snapshotBoundTools(m *MCP) []boundTool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.boundTools)
}

func preserveRecomposedTools(ctx context.Context, srv *dagql.Server, previous, candidate dagql.ObjectResult[*LLM]) (dagql.ObjectResult[*LLM], error) {
	oldBindings := snapshotBoundTools(previous.Self().mcp)
	newBindings := snapshotBoundTools(candidate.Self().mcp)
	byType := make(map[string]boundTool, len(newBindings))
	for _, binding := range newBindings {
		byType[binding.typeName()] = binding
	}
	for _, old := range oldBindings {
		next, exists := byType[old.typeName()]
		if !exists {
			return candidate, fmt.Errorf("reload would discard tool state for %q; use a fresh composition to reset it explicitly", old.typeName())
		}
		if old.Owner != next.Owner {
			label := func(owner string) string {
				if owner == "" {
					return "unowned"
				}
				return fmt.Sprintf("expertise %q", owner)
			}
			return candidate, fmt.Errorf("reload would replace tool binding %q owned by other expertise: %s -> %s; use a fresh composition to replace it explicitly", old.typeName(), label(old.Owner), label(next.Owner))
		}
		if stableIDDigest(old.id) == stableIDDigest(next.id) && old.Version == next.Version {
			continue
		}
		rebound, err := recomposeToolReceiver(ctx, srv, old, next)
		if err != nil {
			return candidate, fmt.Errorf("reload tool state %q: %w", old.typeName(), err)
		}
		id, err := rebound.ID()
		if err != nil {
			return candidate, err
		}
		if err := srv.Select(ctx, candidate, &candidate, dagql.Selector{
			Field: "withTools",
			Args: []dagql.NamedInput{
				{Name: "object", Value: dagql.NewAnyID(id)},
				{Name: "except", Value: dagql.ArrayInput[dagql.String](dagql.NewStringArray(next.Except...))},
				{Name: "version", Value: dagql.Int(next.Version)},
				{Name: "owner", Value: dagql.Opt(dagql.String(next.Owner))},
			},
		}); err != nil {
			return candidate, fmt.Errorf("record reloaded tool state %q: %w", old.typeName(), err)
		}
	}
	return candidate, nil
}

func recomposeToolReceiver(ctx context.Context, srv *dagql.Server, previous, candidate boundTool) (dagql.AnyObjectResult, error) {
	oldObj, err := loadRecomposeBinding(ctx, srv, previous)
	if err != nil {
		return nil, err
	}
	newObj, err := loadRecomposeBinding(ctx, srv, candidate)
	if err != nil {
		return nil, err
	}
	oldState, oldIsModule := dagql.UnwrapAs[*ModuleObject](oldObj)
	newState, newIsModule := dagql.UnwrapAs[*ModuleObject](newObj)
	if !oldIsModule || !newIsModule {
		return nil, fmt.Errorf("cannot reload non-module tool objects; use a fresh composition to replace them explicitly")
	}
	if err := sameToolStateIdentity(oldState, newState); err != nil {
		return nil, err
	}
	if previous.Version != candidate.Version {
		slog.SpanLogger(ctx, InstrumentationLibrary).Warn("resetting tool state: its withTools version changed",
			"object", candidate.typeName(),
			"previousVersion", previous.Version,
			"currentVersion", candidate.Version)
		return newObj, nil
	}
	return RebindModuleObjectState(ctx, srv, newObj, oldObj)
}

// Load only bindings actually replaced by composition. In particular, unrelated
// lazy bindings (which may no longer be reproducible) must stay lazy. Do not
// populate the old LLM's lazy receiver cache while preparing a candidate.
func loadRecomposeBinding(ctx context.Context, srv *dagql.Server, binding boundTool) (dagql.AnyObjectResult, error) {
	obj := binding.object
	if obj == nil {
		var err error
		obj, err = srv.Load(ctx, binding.id)
		if err != nil {
			return nil, fmt.Errorf("load current state for %q: %w", binding.typeName(), err)
		}
	}
	return binding.objType.New(obj)
}

// A binding slot is currently a GraphQL type name. Before transferring state,
// also check installation name and intrinsic module/object identity. Source
// location is deliberately not identity: replacing a remote dependency with a
// local clone (or a fork) is how expertise can fix itself without losing state.
// Equivalent cached implementations may also carry a different source's metadata.
// The selected expertise is trusted code with access to the base conversation;
// preserveRecomposedTools separately prevents claiming another owner's bindings.
func sameToolStateIdentity(previous, candidate *ModuleObject) error {
	oldMod, newMod := previous.Module.Self(), candidate.Module.Self()
	if oldMod.Name() != newMod.Name() || oldMod.OriginalName != newMod.OriginalName ||
		previous.TypeDef.OriginalName != candidate.TypeDef.OriginalName {
		return fmt.Errorf("module or object identity changed")
	}
	return nil
}
