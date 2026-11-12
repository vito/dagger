package core

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/dagger/dagger/dagql"
	"github.com/dagger/dagger/dagql/call"
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

	// Write applies what a successful DangEval snippet wrote, by reassigning
	// `ws` or a bound tool object, and summarizes it. It is called while
	// the snippet's client still holds the written values.
	Write func(context.Context, []DangWrite) (string, error)
}

// DangWrite is a value of the default scope a DangEval snippet reassigned:
// its final value, to apply.
type DangWrite struct {
	// Name is the binding: "ws", or a bound tool object's name.
	Name string
	// TypeName is its GraphQL type.
	TypeName string
	// ID is the encoded ID of its final value.
	ID string
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

	// BoundMods are the modules Bindings need beside an API: their own and
	// their dependencies. A module scope's nested client serves them beside
	// its module (see engine.ClientMetadata.ServedDeps).
	BoundMods []Mod

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
	dangCheckDescription = "Type-check Dang without running anything. Scope: the core API and installed modules (as `Dagger`), or with `at` a Dang module of the workspace, one of its files, or a path:line:col in one (seeing the locals there). Either way `ws: Workspace!` and your tool objects (by type name: `editor`) are bound." + "\n" +
		"Without `source`: the module's or file's errors, warnings and deprecations, as path:line:col." + "\n" +
		"With `source`: its errors, else what it is. A member reference (`xs.reduce`, `JSON.encode`, `Container.withExec`) gives the member's signature, docs and example; a type name, the type and its members; any other expression, its type and members. Probe before writing a helper (`[\"a\"]` lists every list method), and check after each edit, before running a @check." + "\n" +
		"`search` ranks builtins, API fields and the module's declarations by name and docs."
	dangCheckSourceDescription = "Dang to check as one more file of the scope; its last expression is described."
	dangCheckAtDescription     = "Scope: a Dang module of the workspace (name or directory), a .dang file of one, or path:line:col in one. Default: the workspace scope."
	dangCheckSearchDescription = "Words to find in names and docs, e.g. \"sort\" or \"escape html\"."

	dangEvalDescription = "Run Dang: printed output, then the result as `TYPE = value`, then its type's member names. Scope as for DangCheck, without positions: `at` a module runs the snippet as one more file of it, after its top level. A type error runs nothing." + "\n" +
		"Engine calls go through the scope's API (with `at`, the module's dependencies and own API), with no access to the host, and are counted. Assignment is the write: if the snippet succeeds, a reassigned `ws` (`ws = ws.withChanges(editor.edit(source: ws, …))`) is applied to the workspace and a reassigned tool object (`editor = editor.todoWrite(…)`) becomes its state; nothing else is. Objects stay lazy (shown as their query) until you select a scalar such as `.stdout` or `.sync`. Each call is standalone; wrap long runs in Timeout."
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
		start := m.workspace
		return DangToolEnv{
			Server:    srv,
			Workspace: start,
			Scope:     m.dangScope,
			Write: func(ctx context.Context, writes []DangWrite) (string, error) {
				return m.applyDangWrites(ctx, srv, start, writes)
			},
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
// methods type-check and run. A call on a binding from a snippet only returns
// its result; reassigning the binding is what rebinds the object (see
// applyDangWrites).
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
		boundMods := []Mod{NewUserMod(mod)}
		if deps := mod.Self().Deps; deps != nil {
			for _, dep := range deps.Mods() {
				if dep.Name() != ModuleName {
					boundMods = append(boundMods, dep)
				}
			}
		}
		for _, bm := range boundMods {
			scope.Deps = scope.Deps.With(bm, InstallOpts{SkipConstructor: true})
		}
		scope.BoundMods = append(scope.BoundMods, boundMods...)
	}

	scope.SchemaFile, err = scope.Deps.SchemaIntrospectionJSONFileForModule(ctx)
	if err != nil && len(scope.Bindings) > 0 {
		// The bound modules don't fit beside the API: go without them.
		for _, b := range scope.Bindings {
			scope.Skipped = append(scope.Skipped, fmt.Sprintf("%s: %v", b.TypeName, err))
		}
		scope.Bindings, scope.BoundMods, scope.Deps = nil, nil, base
		scope.SchemaFile, err = base.SchemaIntrospectionJSONFileForModule(ctx)
	}
	if err != nil {
		return nil, err
	}
	return scope, nil
}

// applyDangWrites applies what a DangEval snippet wrote: assignment is the
// write. The final `ws` becomes the conversation's workspace, as a tool
// returning a Workspace makes it (commits and resets included); a reassigned
// tool object is rebound as a tool call returning its own type rebinds it.
// start is the workspace the snippet began with. It summarizes what was
// written.
//
// Writes are all or nothing: every final value is loaded and validated, and
// each tool object's new state computed, before anything changes; then they
// are committed together (see commitDangWrites), and a commit that fails
// undoes the ones before it.
func (m *MCP) applyDangWrites(ctx context.Context, srv *dagql.Server, start dagql.ObjectResult[*Workspace], writes []DangWrite) (string, error) {
	if len(writes) == 0 {
		return "0 writes", nil
	}
	var commits []dangCommit
	for _, w := range writes {
		var id call.ID
		if err := id.Decode(w.ID); err != nil {
			return "", fmt.Errorf("decode the final %s: %w", w.Name, err)
		}
		final, err := srv.Load(ctx, &id)
		if err != nil {
			return "", fmt.Errorf("load the final %s: %w", w.Name, err)
		}
		if w.Name == "ws" {
			ws, ok := dagql.UnwrapAs[dagql.ObjectResult[*Workspace]](final)
			if !ok {
				return "", fmt.Errorf("write ws: got a %s, not a Workspace", final.Type().Name())
			}
			if start.Self() == nil {
				return "", fmt.Errorf("write ws: no workspace bound")
			}
			commits = append(commits, dangCommit{name: "ws", apply: func() (string, error) {
				return m.writeDangWorkspace(ctx, srv, ws)
			}})
			continue
		}
		recv, ok, err := m.boundToolObject(ctx, srv, w.TypeName)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("write %s: no bound %s", w.Name, w.TypeName)
		}
		state, changed, err := m.fieldwiseState(ctx, srv, recv, final)
		if err != nil {
			return "", fmt.Errorf("write %s: %w", w.Name, err)
		}
		name, typeName := w.Name, w.TypeName
		commits = append(commits, dangCommit{name: name, apply: func() (string, error) {
			if !changed {
				return name + " state unchanged", nil
			}
			if err := m.rebindBoundTool(typeName, state); err != nil {
				return "", err
			}
			return name + " state updated", nil
		}})
	}
	if err := m.guardStateChange(); err != nil {
		return "", err
	}
	return m.commitDangWrites(commits)
}

// dangCommit is one resolved write of a DangEval snippet, ready to apply.
type dangCommit struct {
	name string
	// apply makes the write and summarizes it: a first line, then any
	// detail (a workspace's patch).
	apply func() (string, error)
}

// commitDangWrites applies resolved writes in order. If one fails, the
// workspace, the bindings and the state-changed mark are restored to what
// they were before the first, so a failed call has written nothing.
func (m *MCP) commitDangWrites(commits []dangCommit) (string, error) {
	m.mu.Lock()
	prevBound := slices.Clone(m.boundTools)
	prevChanged := m.stateChanged
	m.mu.Unlock()
	prevWorkspace := m.workspace

	var lines, details []string
	for _, c := range commits {
		out, err := c.apply()
		if err != nil {
			m.workspace = prevWorkspace
			m.mu.Lock()
			m.boundTools = prevBound
			m.stateChanged = prevChanged
			m.mu.Unlock()
			return "", fmt.Errorf("write %s: %w (nothing was written)", c.name, err)
		}
		line, detail, _ := strings.Cut(out, "\n")
		lines = append(lines, line)
		if detail != "" {
			details = append(details, detail)
		}
	}
	return strings.Join(append([]string{strings.Join(lines, " · ")}, details...), "\n"), nil
}

// writeDangWorkspace makes final, a Workspace a snippet assigned to `ws`,
// the conversation's workspace, as a tool returning it does
// (rebindWorkspace): its commits, resets and file edits all carry over, and
// the summary says what changed.
func (m *MCP) writeDangWorkspace(ctx context.Context, srv *dagql.Server, final dagql.ObjectResult[*Workspace]) (string, error) {
	summary, err := m.rebindWorkspace(ctx, srv, final)
	if err != nil {
		return "", err
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return "ws unchanged", nil
	}
	line := "ws written"
	if n := strings.Count(summary, "diff --git "); n > 0 {
		files := "files"
		if n == 1 {
			files = "file"
		}
		line = fmt.Sprintf("ws written (%d %s)", n, files)
	}
	return line + "\n" + summary, nil
}
