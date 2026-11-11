package core

import (
	"context"
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
	// Eval runs a snippet and returns its output, or an error carrying the
	// diagnostics or the failure (with any output printed before it).
	Eval(ctx context.Context, env DangToolEnv, args DangEvalArgs) (string, error)
}

// DangToolEnv is what a Dang tool call sees of its conversation.
type DangToolEnv struct {
	// Server is the conversation's schema.
	Server *dagql.Server

	// Workspace is the conversation's workspace, if it is bound to one: the
	// default scope binds it as `ws`, and `at` names modules and files in it.
	Workspace dagql.ObjectResult[*Workspace]

	// SchemaFile returns the introspection JSON of the default (workspace)
	// scope's API, as module code sees it: the core API and the modules the
	// conversation's schema serves.
	SchemaFile func(context.Context) (dagql.Result[*File], error)
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
	dangCheckDescription = "Type-check Dang without running anything. Scope: the core API and installed modules (as `Dagger`) with `ws: Workspace!` bound, or with `at` a Dang module of the workspace, one of its files, or a path:line:col in one (seeing the locals there)." + "\n" +
		"Without `source`: the module's or file's errors, warnings and deprecations, as path:line:col." + "\n" +
		"With `source`: its errors, else what it is. A member reference (`xs.reduce`, `JSON.encode`, `Container.withExec`) gives the member's signature, docs and example; a type name, the type and its members; any other expression, its type and members. Probe before writing a helper (`[\"a\"]` lists every list method), and check after each edit, before running a @check." + "\n" +
		"`search` ranks builtins, API fields and the module's declarations by name and docs."
	dangCheckSourceDescription = "Dang to check as one more file of the scope; its last expression is described."
	dangCheckAtDescription     = "Scope: a Dang module of the workspace (name or directory), a .dang file of one, or path:line:col in one. Default: the workspace scope."
	dangCheckSearchDescription = "Words to find in names and docs, e.g. \"sort\" or \"escape html\"."

	dangEvalDescription = "Run Dang: printed output, then the result as `TYPE = value`, then its type's member names. Scope as for DangCheck, without positions: `at` a module runs the snippet as one more file of it, after its top level. A type error runs nothing." + "\n" +
		"Engine calls reach the core API, or with `at` the module's dependencies and own API; they are counted, and nothing is applied to the workspace. Objects stay lazy (shown as their query) until you select a scalar such as `.stdout` or `.sync`. Each call is standalone; wrap long runs in Timeout."
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
			SchemaFile: func(ctx context.Context) (dagql.Result[*File], error) {
				ctx, deps, err := m.baseDeps(ctx)
				if err != nil {
					return dagql.Result[*File]{}, err
				}
				return deps.SchemaIntrospectionJSONFileForModule(ctx)
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
