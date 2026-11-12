package dangv2

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing/fstest"

	"github.com/Khan/genqlient/graphql"
	telemetry "github.com/dagger/otel-go"
	"github.com/vito/dang/v2/pkg/dang/analysis"
	"github.com/vito/dang/v2/pkg/introspection"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/core/modules"
	dangshared "github.com/dagger/dagger/core/sdk/dang/shared"
	"github.com/dagger/dagger/core/workspace"
	"github.com/dagger/dagger/dagql"
)

// This file connects the DangCheck and DangEval builtins (see
// core/llm_dang_tools.go) to the engine: it resolves a call's scope in the
// conversation's workspace, reads the source as the workspace has it now
// (pending edits included), and runs snippets with the nested client a
// module function gets. tools_check.go and tools_eval.go do the Dang work.

func init() {
	core.RegisterDangTools(dangTools{})
}

type dangTools struct{}

func (dangTools) Check(ctx context.Context, env core.DangToolEnv, args core.DangCheckArgs) (string, error) {
	ctx = withDangSyntaxCache(ctx)
	var out string
	err := withDangScope(ctx, env, args.At, true, func(ctx context.Context, sc *dangScope, _ dagql.ObjectResult[*core.ModuleSource]) error {
		var err error
		out, err = checkDang(ctx, sc, args.Source, args.Search)
		return err
	})
	return out, err
}

func (dangTools) Eval(ctx context.Context, env core.DangToolEnv, args core.DangEvalArgs) (string, error) {
	if strings.TrimSpace(args.Source) == "" {
		return "", errors.New("source is required")
	}
	ctx = withDangSyntaxCache(ctx)
	query, err := core.CurrentQuery(ctx)
	if err != nil {
		return "", err
	}
	var out string
	err = withDangScope(ctx, env, args.At, false, func(ctx context.Context, sc *dangScope, src dagql.ObjectResult[*core.ModuleSource]) error {
		nestedClientMetadata, err := dangshared.NewNestedClientMetadata(ctx)
		if err != nil {
			return err
		}
		// The nested client serves the API the snippet was checked against:
		// a module scope's module, as for its own functions, or else the
		// conversation's, installed modules included.
		modCtx := dangModuleContext(ctx, env.Server, src)
		if src.Self() == nil {
			deps, err := env.Deps(ctx)
			if err != nil {
				return fmt.Errorf("load the workspace schema: %w", err)
			}
			nestedClientMetadata.ServedDeps = deps
		}
		// Printed output goes to the tool call's span, as a module
		// function's goes to its call's.
		stdio := telemetry.SpanStdio(ctx, core.InstrumentationLibrary)
		defer stdio.Close()
		_, err = dangshared.WithNestedClientServer(ctx, query, nestedClientMetadata, true /* inert attachables */, nil, modCtx, func(ctx context.Context, client graphql.Client) ([]byte, error) {
			var err error
			out, err = evalDang(ctx, sc, args.Source, client, stdio.Stdout)
			return nil, err
		})
		return err
	})
	return out, err
}

// dangModuleContext loads a module scope's module, best effort, so the nested
// client serves what its code calls at runtime: its dependencies and its own
// API. A module that doesn't load leaves the client the core API.
func dangModuleContext(ctx context.Context, srv *dagql.Server, src dagql.ObjectResult[*core.ModuleSource]) dagql.ObjectResult[*core.Module] {
	var mod dagql.ObjectResult[*core.Module]
	if src.Self() == nil {
		return mod
	}
	if err := srv.Select(ctx, src, &mod, dagql.Selector{View: srv.View, Field: "asModule"}); err != nil {
		return dagql.ObjectResult[*core.Module]{}
	}
	return mod
}

// dangAtPosition matches `at` naming a position: path:line or path:line:col.
var dangAtPosition = regexp.MustCompile(`^(.+\.dang):(\d+)(?::(\d+))?$`)

// parseDangAt splits `at` into what it names and an optional position.
func parseDangAt(at string) (string, *analysis.Pos, error) {
	at = strings.TrimSpace(at)
	m := dangAtPosition.FindStringSubmatch(at)
	if m == nil {
		return at, nil, nil
	}
	line, err := strconv.Atoi(m[2])
	if err != nil || line < 1 {
		return "", nil, fmt.Errorf("at %q: invalid line", at)
	}
	col := 1
	if m[3] != "" {
		col, err = strconv.Atoi(m[3])
		if err != nil || col < 1 {
			return "", nil, fmt.Errorf("at %q: invalid column", at)
		}
	}
	return m[1], &analysis.Pos{Line: line, Column: col}, nil
}

// withDangScope resolves `at` into a scope and calls fn with it, and with
// the module source of a module scope, if the directory is one's. Only
// DangCheck takes positions.
func withDangScope(
	ctx context.Context,
	env core.DangToolEnv,
	at string,
	allowPosition bool,
	fn func(context.Context, *dangScope, dagql.ObjectResult[*core.ModuleSource]) error,
) error {
	var noSource dagql.ObjectResult[*core.ModuleSource]
	if strings.TrimSpace(at) == "" {
		schema, err := workspaceDangSchema(ctx, env)
		if err != nil {
			return err
		}
		sc := &dangScope{fsys: fstest.MapFS{}, dir: ".", label: "workspace scope", schema: schema}
		if ws := env.Workspace; ws.Self() != nil {
			id, err := ws.ID()
			if err != nil {
				return err
			}
			encoded, err := id.Encode()
			if err != nil {
				return err
			}
			sc.wsDecl = workspaceDeclaration(encoded)
		}
		return fn(ctx, sc, noSource)
	}

	ws := env.Workspace
	if ws.Self() == nil {
		return fmt.Errorf("at %q: this conversation has no workspace to find it in", at)
	}
	target, pos, err := parseDangAt(at)
	if err != nil {
		return err
	}
	if pos != nil && !allowPosition {
		return fmt.Errorf("at %q: give a module or a .dang file, not a position", at)
	}
	loc, err := resolveDangLocation(ctx, env.Server, ws, target)
	if err != nil {
		return fmt.Errorf("at %q: %w", at, err)
	}
	if pos != nil && loc.file == "" {
		return fmt.Errorf("at %q: a position needs a .dang file", at)
	}

	cwd := cleanWorkspacePath(strings.TrimPrefix(ws.Self().Cwd, "/"))
	label := displayWorkspacePath(cwd, loc.dir)
	var selfSource *core.ModuleSource
	if src := loc.src.Self(); src != nil {
		name := src.ModuleOriginalName
		if name == "" {
			name = src.ModuleName
		}
		if name != "" {
			label = fmt.Sprintf("module %s (%s)", name, label)
		}
		selfSource = &core.ModuleSource{ModuleName: name, ModuleOriginalName: name, EngineVersion: src.EngineVersion}
		ctx, err = ContextWithDangVersion(ctx, moduleDangVersion(src))
		if err != nil {
			return err
		}
	}

	var schema *introspection.Schema
	if loc.src.Self() != nil {
		var file dagql.ObjectResult[*core.File]
		if err := env.Server.Select(ctx, loc.src, &file, dagql.Selector{View: env.Server.View, Field: "introspectionSchemaJson"}); err != nil {
			return fmt.Errorf("load the schema of %s: %w", label, err)
		}
		schema, err = loadDangSchema(ctx, file.Result)
	} else {
		schema, err = workspaceDangSchema(ctx, env)
	}
	if err != nil {
		return err
	}

	var dir dagql.ObjectResult[*core.Directory]
	if err := env.Server.Select(ctx, ws, &dir, dagql.Selector{
		View:  env.Server.View,
		Field: "directory",
		Args:  []dagql.NamedInput{{Name: "path", Value: dagql.NewString(workspaceAPIPath(loc.dir))}},
	}); err != nil {
		return fmt.Errorf("at %q: %w", at, err)
	}
	return dir.Self().Mount(ctx, dir, func(mnt string) error {
		if selfSource != nil {
			schema = ensureModuleSelfTypes(ctx, schema, selfSource, mnt)
		}
		sc := &dangScope{
			fsys:   subdirFS{prefix: loc.dir, fsys: os.DirFS(mnt)},
			dir:    loc.dir,
			cwd:    cwd,
			module: true,
			file:   loc.file,
			pos:    pos,
			label:  label,
			schema: schema,
		}
		if loc.file != "" {
			if _, err := fs.Stat(sc.fsys, loc.file); err != nil {
				return fmt.Errorf("at %q: no file %s in the workspace", at, sc.display(loc.file))
			}
		}
		return fn(ctx, sc, loc.src)
	})
}

// workspaceDangSchema loads the workspace scope's API: the conversation's.
func workspaceDangSchema(ctx context.Context, env core.DangToolEnv) (*introspection.Schema, error) {
	file, err := env.SchemaFile(ctx)
	if err != nil {
		return nil, fmt.Errorf("load the workspace schema: %w", err)
	}
	return loadDangSchema(ctx, file)
}

// dangLocation is where `at` points in the workspace.
type dangLocation struct {
	// dir is the directory of .dang files, relative to the workspace root.
	dir string
	// file is the .dang file `at` names, if it names one.
	file string
	// src is the module whose source dir is, if there is one.
	src dagql.ObjectResult[*core.ModuleSource]
}

// resolveDangLocation finds what `at` (without a position) names: a module of
// the workspace by name, a directory, or a .dang file, with paths resolved as
// Workspace.directory resolves them.
func resolveDangLocation(ctx context.Context, srv *dagql.Server, ws dagql.ObjectResult[*core.Workspace], target string) (dangLocation, error) {
	var loc dangLocation
	switch {
	case strings.HasSuffix(target, ".dang"):
		p, err := dangWorkspacePath(ws.Self(), target)
		if err != nil {
			return loc, err
		}
		loc.file, loc.dir = p, path.Dir(p)
	default:
		if source, ok := installedModuleDir(ctx, srv, ws, target); ok {
			loc.dir = source
		} else {
			p, err := dangWorkspacePath(ws.Self(), target)
			if err != nil {
				return loc, err
			}
			loc.dir = p
		}
	}
	loc.src, loc.dir = findDangModuleSource(ctx, srv, ws, loc.dir, loc.file == "")
	return loc, nil
}

// installedModuleDir returns the root of the local module installed in the
// workspace as name.
func installedModuleDir(ctx context.Context, srv *dagql.Server, ws dagql.ObjectResult[*core.Workspace], name string) (string, bool) {
	if name == "" || strings.ContainsAny(name, "/\\") {
		return "", false
	}
	var mod dagql.ObjectResult[*core.WorkspaceModule]
	if err := srv.Select(ctx, ws, &mod, dagql.Selector{
		View:  srv.View,
		Field: "module",
		Args:  []dagql.NamedInput{{Name: "name", Value: dagql.NewString(name)}},
	}); err != nil {
		return "", false
	}
	source := mod.Self().Source
	if source == "" || !workspace.IsLocalRef(source, "") {
		return "", false
	}
	return cleanWorkspacePath(strings.TrimPrefix(source, "/")), true
}

// findDangModuleSource finds the module whose Dang source is in dir: the
// nearest module at or above it, if dir is its source directory, or, when
// rootOK, its root (dir then becomes its source directory). It returns the
// module source, if any, and the directory of .dang files.
func findDangModuleSource(ctx context.Context, srv *dagql.Server, ws dagql.ObjectResult[*core.Workspace], dir string, rootOK bool) (dagql.ObjectResult[*core.ModuleSource], string) {
	var none dagql.ObjectResult[*core.ModuleSource]
	// The nearest module config, of either format.
	root := ""
	for _, name := range modules.ConfigFilenames() {
		var found dagql.Nullable[dagql.String]
		if err := srv.Select(ctx, ws, &found, dagql.Selector{
			View:  srv.View,
			Field: "findUp",
			Args: []dagql.NamedInput{
				{Name: "name", Value: dagql.NewString(name)},
				{Name: "from", Value: dagql.NewString(workspaceAPIPath(dir))},
			},
		}); err != nil || !found.Valid {
			continue
		}
		candidate := cleanWorkspacePath(path.Dir(strings.TrimPrefix(found.Value.String(), "/")))
		if root == "" || pathDepth(candidate) > pathDepth(root) {
			root = candidate
		}
	}
	if root == "" {
		return none, dir
	}
	var src dagql.ObjectResult[*core.ModuleSource]
	if err := srv.Select(ctx, ws, &src, dagql.Selector{
		View:  srv.View,
		Field: "moduleSource",
		Args:  []dagql.NamedInput{{Name: "path", Value: dagql.NewString(workspaceAPIPath(root))}},
	}); err != nil {
		return none, dir
	}
	sub := "."
	if rel, ok := relPath(src.Self().SourceRootSubpath, src.Self().SourceSubpath); ok {
		sub = rel
	}
	srcDir := cleanWorkspacePath(path.Join(root, sub))
	switch {
	case srcDir == dir:
		return src, dir
	case rootOK && root == dir:
		return src, srcDir
	}
	return none, dir
}

// relPath is target relative to base, both slash-separated and relative to
// the same directory, when target is within base.
func relPath(base, target string) (string, bool) {
	base, target = path.Clean(base), path.Clean(target)
	switch {
	case base == target:
		return ".", true
	case base == ".":
		return target, !strings.HasPrefix(target, "../")
	case strings.HasPrefix(target, base+"/"):
		return strings.TrimPrefix(target, base+"/"), true
	}
	return "", false
}

// dangWorkspacePath resolves a path as Workspace.directory does: absolute
// from the workspace root, relative from its cwd. The result is relative to
// the root.
func dangWorkspacePath(ws *core.Workspace, p string) (string, error) {
	p = strings.ReplaceAll(p, `\`, "/")
	var resolved string
	if strings.HasPrefix(p, "/") {
		resolved = strings.TrimPrefix(path.Clean(p), "/")
	} else {
		cwd := strings.TrimPrefix(ws.Cwd, "/")
		resolved = path.Join(cwd, p)
	}
	resolved = cleanWorkspacePath(resolved)
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return "", fmt.Errorf("%s is outside the workspace", p)
	}
	return resolved, nil
}

func cleanWorkspacePath(p string) string {
	p = path.Clean(p)
	if p == "" || p == "/" {
		return "."
	}
	return p
}

// workspaceAPIPath is the absolute (root-anchored) path a Workspace API takes
// for a root-relative one.
func workspaceAPIPath(p string) string {
	if p = cleanWorkspacePath(p); p == "." {
		return "/"
	}
	return "/" + p
}

// pathDepth counts the directories of a root-relative path.
func pathDepth(p string) int {
	if p = cleanWorkspacePath(p); p == "." {
		return 0
	}
	return strings.Count(p, "/") + 1
}

// displayWorkspacePath names a root-relative path as `at` resolves it back:
// relative to the workspace cwd when within it, else anchored at the root.
func displayWorkspacePath(cwd, p string) string {
	if rel, ok := relPath(cwd, p); ok {
		return rel
	}
	return "/" + p
}

// subdirFS serves fsys as the directory prefix of an otherwise empty file
// system, so files read from a mounted module directory are named by their
// workspace path. Nothing above prefix exists, so neither does a dang.toml
// above the module directory, which the engine never reads either (see
// dangSourceDir).
type subdirFS struct {
	prefix string
	fsys   fs.FS
}

var (
	_ fs.ReadDirFS  = subdirFS{}
	_ fs.ReadFileFS = subdirFS{}
	_ fs.StatFS     = subdirFS{}
)

func (s subdirFS) rel(op, name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	switch {
	case s.prefix == "." || s.prefix == "":
		return name, nil
	case name == s.prefix:
		return ".", nil
	case strings.HasPrefix(name, s.prefix+"/"):
		return strings.TrimPrefix(name, s.prefix+"/"), nil
	}
	return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
}

func (s subdirFS) Open(name string) (fs.File, error) {
	rel, err := s.rel("open", name)
	if err != nil {
		return nil, err
	}
	return s.fsys.Open(rel)
}

func (s subdirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	rel, err := s.rel("readdir", name)
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(s.fsys, rel)
}

func (s subdirFS) ReadFile(name string) ([]byte, error) {
	rel, err := s.rel("readfile", name)
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(s.fsys, rel)
}

func (s subdirFS) Stat(name string) (fs.FileInfo, error) {
	rel, err := s.rel("stat", name)
	if err != nil {
		return nil, err
	}
	return fs.Stat(s.fsys, rel)
}
