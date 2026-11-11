package dangv2

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/Khan/genqlient/graphql"
	"github.com/stretchr/testify/require"
	"github.com/vito/dang/v2/pkg/dang"
	"github.com/vito/dang/v2/pkg/dang/analysis"
	"github.com/vito/dang/v2/pkg/introspection"
)

const dangToolsTestSDL = `
interface Node {
  id: ID!
}

type Query {
  node(id: ID!): Node
  """
  Creates a scratch container.
  """
  container: Container!
  """
  Reads a secret.
  """
  secret(name: String!): Secret!
}

"""
An OCI-compatible container.
"""
type Container implements Node {
  id: ID!
  """
  Execute a command in the container.
  """
  withExec(args: [String!]!): Container!
  """
  The output stream of the last executed command.
  """
  stdout: String!
  """
  Forces evaluation of the pipeline.
  """
  sync: Container!
}

type Directory implements Node {
  id: ID!
  entries: [String!]!
}

type Workspace implements Node {
  id: ID!
  """
  Returns a Directory from the workspace.
  """
  directory(path: String!): Directory!
}

type Secret implements Node {
  id: ID!
  plaintext: String!
}
`

func dangToolsTestSchema(t *testing.T) *introspection.Schema {
	t.Helper()
	schema, err := dang.SchemaFromSDL(dangToolsTestSDL, "test.graphqls")
	require.NoError(t, err)
	return schema
}

func workspaceTestScope(t *testing.T) *dangScope {
	return &dangScope{
		fsys:   fstest.MapFS{},
		dir:    ".",
		label:  "workspace scope",
		wsDecl: workspaceDeclaration("ws-id"),
		schema: dangToolsTestSchema(t),
	}
}

// moduleTestScope is a module at mod/ of a workspace.
func moduleTestScope(t *testing.T, files map[string]string) *dangScope {
	fsys := fstest.MapFS{}
	for name, text := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(text)}
	}
	return &dangScope{
		fsys:   subdirFS{prefix: "mod", fsys: fsys},
		dir:    "mod",
		module: true,
		label:  "module mod (mod)",
		schema: dangToolsTestSchema(t),
	}
}

func checkTest(t *testing.T, sc *dangScope, source, search string) string {
	t.Helper()
	out, err := checkDang(context.Background(), sc, source, search)
	require.NoError(t, err)
	return out
}

func TestDangCheckWorkspaceScope(t *testing.T) {
	t.Parallel()
	sc := workspaceTestScope(t)

	t.Run("a value's type and members", func(t *testing.T) {
		out := checkTest(t, sc, `["a"]`, "")
		require.Contains(t, out, "[String!]! has ")
		require.Contains(t, out, "\n  reduce(initial: b) { acc: b, item: a => b }: b — ")
		require.True(t, strings.HasSuffix(out, "ok · workspace scope · nothing ran\n"), out)
	})

	t.Run("a member reference", func(t *testing.T) {
		out := checkTest(t, sc, `[1].reduce`, "")
		require.True(t, strings.HasPrefix(out, "List.reduce(initial: b) { acc: b, item: a => b }: b\n"), out)
		require.Contains(t, out, "\n  example: ")
		require.NotContains(t, out, "error")
	})

	t.Run("a static member", func(t *testing.T) {
		out := checkTest(t, sc, `JSON.encode`, "")
		require.Contains(t, out, "JSON.encode(")
	})

	t.Run("a schema type", func(t *testing.T) {
		out := checkTest(t, sc, `Container`, "")
		require.True(t, strings.HasPrefix(out, "type Container\n  An OCI-compatible container.\n"), out)
		require.Contains(t, out, "\n  withExec(args: [String!]!): Dagger.Container! — Execute a command in the container.\n")
	})

	t.Run("a member of a schema type", func(t *testing.T) {
		out := checkTest(t, sc, `Container.withExec`, "")
		require.True(t, strings.HasPrefix(out, "Container.withExec(args: [String!]!): Dagger.Container!\n  Execute a command in the container.\n"), out)
	})

	t.Run("a member selected from a value", func(t *testing.T) {
		out := checkTest(t, sc, `container.withExec`, "")
		require.True(t, strings.HasPrefix(out, "Dagger.Container.withExec(args: [String!]!): Dagger.Container!\n"), out)
	})

	t.Run("ws is bound", func(t *testing.T) {
		out := checkTest(t, sc, `ws`, "")
		require.True(t, strings.HasPrefix(out, "ws: Dagger.Workspace!\n"), out)
		require.Contains(t, out, "Dagger.Workspace! has 2 members:")
		require.Contains(t, out, "directory(path: String!): Dagger.Directory! — Returns a Directory from the workspace.")
		require.NotContains(t, out, "declared at")
	})

	t.Run("a type error", func(t *testing.T) {
		out := checkTest(t, sc, "let xs = [\"b\"]\nxs.size", "")
		require.Contains(t, out, "<check>:2:1: error: ")
		require.Contains(t, out, `"size"`)
		require.True(t, strings.HasSuffix(out, "1 error · workspace scope · nothing ran\n"), out)
	})

	t.Run("a deprecation", func(t *testing.T) {
		out := checkTest(t, sc, `toJSON([1])`, "")
		require.Contains(t, out, "<check>:1:1: warning: toJSON is deprecated: use JSON.encode instead (quick fix: JSON.encode)")
	})

	t.Run("search", func(t *testing.T) {
		out := checkTest(t, sc, "", "accumulator")
		require.True(t, strings.HasPrefix(out, "search \"accumulator\":\n  List.reduce(initial: b)"), out)
		out = checkTest(t, sc, "", "execute command")
		require.Contains(t, out, "  Container.withExec(args: [String!]!): Dagger.Container! — Execute a command in the container.\n")
		out = checkTest(t, sc, "", "zzznothing")
		require.Contains(t, out, `search "zzznothing": no matches`)
	})

	t.Run("nothing to check", func(t *testing.T) {
		_, err := checkDang(context.Background(), sc, "", "")
		require.ErrorContains(t, err, "nothing to check")
	})
}

const testModuleMain = `"""
Sorts names, uppercase first.
"""
let stableOrder(items: [String!]!): [String!]! {
  items.sort
}

pub greet(name: String!): String! {
  let greeting = "hello, "
  greeting + name
}
`

func TestDangCheckModuleScope(t *testing.T) {
	t.Parallel()

	t.Run("diagnostics with workspace paths", func(t *testing.T) {
		sc := moduleTestScope(t, map[string]string{
			"main.dang":   testModuleMain,
			"broken.dang": "let s = toJSON([1])\nlet n: Int! = \"nope\"\n",
		})
		out := checkTest(t, sc, "", "")
		require.Contains(t, out, "mod/broken.dang:1:9: warning: toJSON is deprecated")
		require.Contains(t, out, "mod/broken.dang:2:")
		require.True(t, strings.HasSuffix(out, "1 error, 1 warning · module mod (mod) · 2 files · nothing ran\n"), out)
	})

	t.Run("a clean module", func(t *testing.T) {
		sc := moduleTestScope(t, map[string]string{"main.dang": testModuleMain})
		out := checkTest(t, sc, "", "")
		require.Equal(t, "ok · module mod (mod) · 1 file · nothing ran\n", out)
	})

	t.Run("one file", func(t *testing.T) {
		sc := moduleTestScope(t, map[string]string{
			"main.dang":   testModuleMain,
			"broken.dang": "let n: Int! = \"nope\"\n",
		})
		sc.file = "mod/main.dang"
		out := checkTest(t, sc, "", "")
		require.NotContains(t, out, "broken.dang")
		require.True(t, strings.HasPrefix(out, "ok · "), out)
	})

	t.Run("a private helper and its declaration", func(t *testing.T) {
		sc := moduleTestScope(t, map[string]string{"main.dang": testModuleMain})
		out := checkTest(t, sc, "stableOrder", "")
		require.True(t, strings.HasPrefix(out, "stableOrder(items: [String!]!): [String!]!\n  Sorts names, uppercase first.\n  declared at mod/main.dang:4:5\n"), out)
	})

	t.Run("search finds declarations", func(t *testing.T) {
		sc := moduleTestScope(t, map[string]string{"main.dang": testModuleMain})
		out := checkTest(t, sc, "", "uppercase")
		require.Contains(t, out, "  stableOrder(items: [String!]!): [String!]! — Sorts names, uppercase first.\n")
	})

	t.Run("at a position", func(t *testing.T) {
		sc := moduleTestScope(t, map[string]string{"main.dang": testModuleMain})
		sc.file = "mod/main.dang"
		sc.pos = &analysis.Pos{Line: 10, Column: 3}
		out := checkTest(t, sc, "greeting + name", "")
		require.True(t, strings.HasPrefix(out, "String! has "), out)
		out = checkTest(t, sc, "name.size", "")
		require.Contains(t, out, "<check>:1:1: error: ")
		require.Contains(t, out, "1 error · module mod (mod) · 1 file · nothing ran\n")
		// Outside the function, its locals aren't in scope.
		sc.pos = &analysis.Pos{Line: 1, Column: 1}
		out = checkTest(t, sc, "greeting", "")
		require.Contains(t, out, "error: ")
	})
}

// fakeAPI answers the queries a snippet sends with canned data.
type fakeAPI struct {
	mu      sync.Mutex
	queries []string
	answer  func(query string) string
}

func (f *fakeAPI) MakeRequest(_ context.Context, req *graphql.Request, resp *graphql.Response) error {
	f.mu.Lock()
	f.queries = append(f.queries, req.Query)
	f.mu.Unlock()
	return json.Unmarshal([]byte(f.answer(req.Query)), resp.Data)
}

func evalTest(t *testing.T, sc *dangScope, source string, api *fakeAPI) (string, error) {
	t.Helper()
	if api == nil {
		api = &fakeAPI{answer: func(q string) string { t.Errorf("unexpected query %s", q); return "{}" }}
	}
	var spanLogs strings.Builder
	out, err := evalDang(context.Background(), sc, source, api, &spanLogs)
	if err == nil {
		// Printed output reaches the span's logs too.
		for _, line := range strings.Split(spanLogs.String(), "\n") {
			require.Contains(t, out, line)
		}
	}
	return out, err
}

func TestDangEval(t *testing.T) {
	t.Parallel()
	sc := workspaceTestScope(t)

	t.Run("output and result", func(t *testing.T) {
		out, err := evalTest(t, sc, "print(\"hello\")\n[1, 2].map { x => x * 2 }", nil)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(out, "output:\nhello\n[Int!]! = [2, 4]\nmembers: "), out)
		require.Regexp(t, `\nok · \d+ms · 0 engine calls · workspace scope\n$`, out)
	})

	t.Run("objects stay lazy", func(t *testing.T) {
		out, err := evalTest(t, sc, `container.withExec(["echo", "hi"])`, nil)
		require.NoError(t, err)
		require.Contains(t, out, "Container! = <Container: container.withExec(args: …)>\n(not evaluated; select a field such as .sync or .stdout to run it)\n")
		require.Contains(t, out, "members: id stdout sync withExec\n")
		require.Contains(t, out, " · 0 engine calls · ")
	})

	t.Run("engine calls are counted", func(t *testing.T) {
		api := &fakeAPI{answer: func(string) string {
			return `{"container":{"withExec":{"stdout":"hi\n"}}}`
		}}
		out, err := evalTest(t, sc, `container.withExec(["echo", "hi"]).stdout`, api)
		require.NoError(t, err)
		require.Contains(t, out, "String! = \"hi\\n\"\n")
		require.Contains(t, out, " · 1 engine call · ")
		require.Len(t, api.queries, 1)
	})

	t.Run("a type error runs nothing", func(t *testing.T) {
		_, err := evalTest(t, sc, "print(\"never\")\nnope", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "<check>:2:1: error: ")
		require.Contains(t, err.Error(), "1 error · workspace scope · nothing ran")
		require.NotContains(t, err.Error(), "never\n")
	})

	t.Run("a failure keeps its output", func(t *testing.T) {
		_, err := evalTest(t, sc, "print(\"before\")\nraise \"boom\"", nil)
		require.Error(t, err)
		require.True(t, strings.HasPrefix(err.Error(), "output:\nbefore\n"), err.Error())
		require.Contains(t, err.Error(), "boom")
		require.Contains(t, err.Error(), "failed · ")
	})

	t.Run("secrets are scrubbed", func(t *testing.T) {
		api := &fakeAPI{answer: func(string) string {
			return `{"secret":{"plaintext":"hunter2"}}`
		}}
		out, err := evalTest(t, sc, "let s = secret(name: \"pw\").plaintext\nprint(\"pw is \" + s)\ns", api)
		require.NoError(t, err)
		require.NotContains(t, out, "hunter2")
		require.Contains(t, out, "pw is ***")
		require.Contains(t, out, `String! = "***"`)
	})

	t.Run("a module's top level runs first", func(t *testing.T) {
		sc := moduleTestScope(t, map[string]string{
			"main.dang": testModuleMain + "\nprint(\"module loaded\")\n",
		})
		out, err := evalTest(t, sc, `greet("dang")`, nil)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(out, "output:\nmodule loaded\nString! = \"hello, dang\"\n"), out)
		require.Contains(t, out, " · module mod (mod)\n")
	})
}

func TestParseDangAt(t *testing.T) {
	t.Parallel()
	target, pos, err := parseDangAt("mod/main.dang:10:3")
	require.NoError(t, err)
	require.Equal(t, "mod/main.dang", target)
	require.Equal(t, &analysis.Pos{Line: 10, Column: 3}, pos)

	target, pos, err = parseDangAt("mod/main.dang:7")
	require.NoError(t, err)
	require.Equal(t, "mod/main.dang", target)
	require.Equal(t, &analysis.Pos{Line: 7, Column: 1}, pos)

	target, pos, err = parseDangAt("my-module")
	require.NoError(t, err)
	require.Equal(t, "my-module", target)
	require.Nil(t, pos)

	_, _, err = parseDangAt("main.dang:0:1")
	require.Error(t, err)
}

func TestDangDisplayPaths(t *testing.T) {
	t.Parallel()
	sc := moduleTestScope(t, map[string]string{"main.dang": "let n: Int! = \"nope\"\n"})
	sc.cwd = "mod"
	out := checkTest(t, sc, "", "")
	require.True(t, strings.HasPrefix(out, "main.dang:1:"), "relative to the cwd, as `at` resolves it: %s", out)
	sc.cwd = "elsewhere"
	out = checkTest(t, sc, "", "")
	require.True(t, strings.HasPrefix(out, "/mod/main.dang:1:"), "anchored at the root outside the cwd: %s", out)
}

func TestRelPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ base, target, want string }{
		{".", ".", "."},
		{".", "src", "src"},
		{"mod", "mod", "."},
		{"mod", "mod/src", "src"},
	} {
		got, ok := relPath(tc.base, tc.target)
		require.True(t, ok, tc)
		require.Equal(t, tc.want, got, tc)
	}
	_, ok := relPath("mod", "other")
	require.False(t, ok)
}
