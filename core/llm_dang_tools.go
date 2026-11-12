package core

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/dagger/dagger/dagql"
)

// The Dang builtins: a workbench for agents writing Dang, the language
// modules of the dang SDK are written in. DangCheck answers what an editor's
// language server would (diagnostics, types, members, docs) without running
// anything; DangEval runs a snippet. Both are always registered: their
// descriptions carry the workflow, since nothing else points at them.
const (
	dangCheckToolName = "DangCheck"
	dangEvalToolName  = "DangEval"
)

// DangTools is the language service behind DangCheck and DangEval. Only the
// dang SDK's implementation package (core/sdk/dang/v2) may import Dang (see
// core/sdk/dang/README.md), so it implements this and registers it with
// RegisterDangTools; core only describes the tools and dispatches to it.
type DangTools interface {
	// Check type-checks without running anything and returns the report.
	Check(ctx context.Context, env DangToolEnv, args DangCheckArgs) (string, error)
	// Eval runs a snippet and returns its report: printed output and the
	// result, or the diagnostics or failure that stopped it. An error is
	// for what keeps the tool itself from running (e.g. an unknown `at`).
	Eval(ctx context.Context, env DangToolEnv, args DangEvalArgs) (string, error)
}

// DangToolEnv is what a Dang tool call sees of its conversation.
type DangToolEnv struct {
	// Server is the conversation's schema.
	Server *dagql.Server

	// Workspace is the conversation's workspace, if it is bound to one: the
	// default scope binds it as `ws`, and `at` names modules and files in it.
	Workspace dagql.ObjectResult[*Workspace]

	// Scope returns the default scope, as of the call.
	Scope func(context.Context) (*DangScope, error)
}

// DangScope is the default scope of a Dang tool call: the conversation's API
// and the values bound in it besides `ws`.
type DangScope struct {
	// Deps is the API: the core API, the modules the conversation's schema
	// serves, and the modules of its bound tool objects. A nested client
	// running code in the scope serves it (see
	// engine.ClientMetadata.ServedDeps).
	Deps *SchemaBuilder

	// SchemaFile is Deps' introspection JSON, as module code sees it.
	SchemaFile dagql.Result[*File]

	// Bindings are the conversation's bound tool objects, as of the call.
	Bindings []DangBinding

	// Skipped names the bound tool objects left out of Bindings, and why.
	Skipped []string
}

// DangBinding is a bound tool object in the default scope: a value named
// after its type, loaded from its ID, its state when the call began.
type DangBinding struct {
	// Name is what code calls it: its type name in lowerCamel case.
	Name string
	// TypeName is its GraphQL type.
	TypeName string
	// ID is its encoded ID.
	ID string
}

// DangCheckArgs are DangCheck's arguments.
type DangCheckArgs struct {
	Source string
	At     string
	Search string
}

// DangEvalArgs are DangEval's arguments.
type DangEvalArgs struct {
	Source string
	At     string
}

var (
	dangToolsMu sync.RWMutex
	dangTools   DangTools
)

// RegisterDangTools installs the implementation of the Dang builtins.
func RegisterDangTools(impl DangTools) {
	dangToolsMu.Lock()
	defer dangToolsMu.Unlock()
	dangTools = impl
}

func registeredDangTools() DangTools {
	dangToolsMu.RLock()
	defer dangToolsMu.RUnlock()
	return dangTools
}

// Descriptions of the Dang builtins. Every conversation pays for them, so
// they stay terse; they are the only guidance pointing at the tools.
const (
	dangCheckDescription = "Type-check Dang without running anything. Scope: the core API and installed modules (as `Dagger`) with `ws: Workspace!` and your tool objects bound (by type name: `editor`), or with `at` a Dang module of the workspace, one of its files, or a path:line:col in one (seeing the locals there)." + "\n" +
		"Without `source`: the module's or file's errors, warnings and deprecations, as path:line:col." + "\n" +
		"With `source`: its errors, else what it is. A member reference (`xs.reduce`, `JSON.encode`, `Container.withExec`) gives the member's signature, docs and example; a type name, the type and its members; any other expression, its type and members. Probe before writing a helper (`[\"a\"]` lists every list method), and check after each edit, before running a @check." + "\n" +
		"`search` ranks builtins, API fields and the module's declarations by name and docs."
	dangCheckSourceDescription = "Dang to check as one more file of the scope; its last expression is described."
	dangCheckAtDescription     = "Scope: a Dang module of the workspace (name or directory), a .dang file of one, or path:line:col in one. Default: the workspace scope."
	dangCheckSearchDescription = "Words to find in names and docs, e.g. \"sort\" or \"escape html\"."

	dangEvalDescription = "Run Dang: printed output, then the result as `TYPE = value`, then its type's member names. Scope as for DangCheck, without positions: `at` a module runs the snippet as one more file of it, after its top level. A type error runs nothing." + "\n" +
		"Engine calls go through the scope's API (with `at`, the module's dependencies and own API), with no access to the host; they are counted, and nothing is applied to the workspace or your tools' state. Objects stay lazy (shown as their query) until you select a scalar such as `.stdout` or `.sync`. Each call is standalone; wrap long runs in Timeout."
	dangEvalSourceDescription = "Dang statements to run; the last expression is the result."
	dangEvalAtDescription     = "A Dang module of the workspace (name or directory), or a .dang file of one. Default: the workspace scope."
)

// loadDangTools registers DangCheck and DangEval, whenever the engine has a
// Dang implementation (always, outside of tests).
func (m *MCP) loadDangTools(srv *dagql.Server, allTools *LLMToolSet) {
	impl := registeredDangTools()
	if impl == nil {
		return
	}
	env := func() DangToolEnv {
		return DangToolEnv{
			Server:    srv,
			Workspace: m.workspace,
			Scope:     m.dangScope,
		}
	}
	allTools.Add(LLMTool{
		Name:        dangCheckToolName,
		Description: dangCheckDescription,
		ReadOnly:    true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source": map[string]any{"type": "string", "description": dangCheckSourceDescription},
				"at":     map[string]any{"type": "string", "description": dangCheckAtDescription},
				"search": map[string]any{"type": "string", "description": dangCheckSearchDescription},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
		Call: ToolFunc(srv, func(ctx context.Context, args struct {
			Source string `default:""`
			At     string `default:""`
			Search string `default:""`
		}) (any, error) {
			return impl.Check(ctx, env(), DangCheckArgs(args))
		}),
	})
	allTools.Add(LLMTool{
		Name:        dangEvalToolName,
		Description: dangEvalDescription,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source": map[string]any{"type": "string", "description": dangEvalSourceDescription},
				"at":     map[string]any{"type": "string", "description": dangEvalAtDescription},
			},
			"required":             []string{"source"},
			"additionalProperties": false,
		},
		Call: ToolFunc(srv, func(ctx context.Context, args struct {
			Source string
			At     string `default:""`
		}) (any, error) {
			return impl.Eval(ctx, env(), DangEvalArgs(args))
		}),
	})
}

// dangScope is the default scope of a Dang tool call: the conversation's
// API, plus each bound tool object, under its type's lowerCamel name, with
// its module (and that module's dependencies) served beside the API so its
// methods type-check and run. Bindings are values only: a call on one from a
// snippet returns its result, and never rebinds the object or applies a
// changeset (see MCP.routeObjectMethodResult for what a tool call does).
//
// Only module objects are bound: a core type's name would shadow a core
// field (`container`), and `ws` is the workspace already. An object whose
// module can't join the API is skipped rather than failing the call.
func (m *MCP) dangScope(ctx context.Context) (*DangScope, error) {
	ctx, base, err := m.baseDeps(ctx)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	bound := slices.Clone(m.boundTools)
	m.mu.Unlock()

	scope := &DangScope{Deps: base}
	seen := map[string]bool{"ws": true}
	for _, b := range bound {
		typeName := b.typeName()
		objType := b.objType
		if b.object != nil {
			objType = b.object.ObjectType()
		}
		mod, ok := ModuleObjectTypeModule(objType)
		if !ok {
			scope.Skipped = append(scope.Skipped, typeName+": not a module object")
			continue
		}
		name := LatestNamer.FieldName(typeName)
		if seen[name] {
			scope.Skipped = append(scope.Skipped, fmt.Sprintf("%s: %q is taken", typeName, name))
			continue
		}
		id := b.id
		if id == nil && b.object != nil {
			if id, err = b.object.ID(); err != nil {
				return nil, err
			}
		}
		if id == nil {
			scope.Skipped = append(scope.Skipped, typeName+": no ID")
			continue
		}
		encoded, err := id.Encode()
		if err != nil {
			return nil, fmt.Errorf("encode bound %s: %w", typeName, err)
		}
		seen[name] = true
		scope.Bindings = append(scope.Bindings, DangBinding{Name: name, TypeName: typeName, ID: encoded})
		// Its types, not its constructor: the name is the bound value's.
		scope.Deps = scope.Deps.With(NewUserMod(mod), InstallOpts{SkipConstructor: true})
		if deps := mod.Self().Deps; deps != nil {
			for _, dep := range deps.Mods() {
				if dep.Name() != ModuleName {
					scope.Deps = scope.Deps.With(dep, InstallOpts{SkipConstructor: true})
				}
			}
		}
	}

	scope.SchemaFile, err = scope.Deps.SchemaIntrospectionJSONFileForModule(ctx)
	if err != nil && len(scope.Bindings) > 0 {
		// The bound modules don't fit beside the API: go without them.
		for _, b := range scope.Bindings {
			scope.Skipped = append(scope.Skipped, fmt.Sprintf("%s: %v", b.TypeName, err))
		}
		scope.Bindings, scope.Deps = nil, base
		scope.SchemaFile, err = base.SchemaIntrospectionJSONFileForModule(ctx)
	}
	if err != nil {
		return nil, err
	}
	return scope, nil
}
