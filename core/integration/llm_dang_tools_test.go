package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"dagger.io/dagger"
	"dagger.io/dagger/core"

	"github.com/dagger/testctx"
	"github.com/stretchr/testify/require"
)

// dangToolCall is one call a canned model makes.
type dangToolCall struct {
	tool string
	args map[string]string
}

// runDangToolCalls has a canned model make calls, one per turn, in a
// conversation the shell pipeline setup prepares, and returns its transcript.
func runDangToolCalls(ctx context.Context, t *testctx.T, c *dagger.Client, base *core.Container, shell func(string) core.WithContainerFunc, setup string, calls []dangToolCall) string {
	t.Helper()
	const prompt = "probe some dang"
	script := core.NewQuery(c).LLM().WithPrompt(prompt)
	for i, call := range calls {
		args, err := json.Marshal(call.args)
		require.NoError(t, err)
		id := fmt.Sprintf("dang_%d", i)
		script = script.
			WithResponse([]core.LLMContentBlockInput{
				{Kind: core.LLMContentBlockKindToolCall, CallID: id, ToolName: call.tool, Arguments: core.JSON(args)},
			}).
			WithToolResult(id, "", false)
	}
	model := cannedRecordingModel(ctx, t, c, script.WithResponse([]core.LLMContentBlockInput{
		{Kind: core.LLMContentBlockKindText, Text: "done"},
	}))
	out, err := base.With(shell(fmt.Sprintf(
		`llm --model="%s" | %s | with-prompt "%s" | loop | transcript`, model, setup, prompt,
	))).Stdout(ctx)
	require.NoError(t, err)
	require.Contains(t, out, "done")
	return out
}

// TestDangTools drives the DangCheck and DangEval builtins end to end over a
// bound workspace with a Dang module: the workspace scope (with `ws` bound to
// the conversation's workspace), a module of it, a position in one of its
// files, search, type errors, laziness and engine calls.
func (LLMSuite) TestDangTools(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	base := workspaceFixture(t, c, "dang-tools")
	const bind = `with-workspace --workspace $(current-workspace)`

	t.Run("always registered", func(ctx context.Context, t *testctx.T) {
		tools, err := base.With(daggerShellNoMod("llm | tools")).Stdout(ctx)
		require.NoError(t, err)
		require.Contains(t, tools, "## DangCheck\n")
		require.Contains(t, tools, "## DangEval\n")
	})

	t.Run("workspace and module scopes", func(ctx context.Context, t *testctx.T) {
		out := runDangToolCalls(ctx, t, c, base, daggerShellNoMod, bind, []dangToolCall{
			{"DangCheck", map[string]string{"source": `["a"].reduce`}},
			{"DangCheck", map[string]string{"source": "nope"}},
			{"DangCheck", map[string]string{"search": "accumulator"}},
			{"DangCheck", map[string]string{"at": "notes"}},
			{"DangCheck", map[string]string{"at": "notes", "source": "shout"}},
			{"DangCheck", map[string]string{"at": ".dagger/modules/notes/main.dang:12:5", "source": "prefix + name"}},
			{"DangEval", map[string]string{"source": "print(\"reading\")\nws.file(\"notes.txt\").contents"}},
			{"DangEval", map[string]string{"source": `container.from("alpine")`}},
			{"DangEval", map[string]string{"at": "notes", "source": `shout("dang")`}},
			{"DangEval", map[string]string{"source": "print(\"never\")\nnope"}},
			// The workspace scope runs against the conversation's API, the
			// installed module included, as DangCheck checks against it.
			{"DangCheck", map[string]string{"source": `notes.greet(name: "x")`}},
			{"DangEval", map[string]string{"source": `notes.greet(name: "x")`}},
			// Loading the workspace's modules from a snippet, which has no
			// host to find an outer .env on.
			{"DangEval", map[string]string{"source": `ws.artifacts.items.{{moduleName}}`}},
			{"DangEval", map[string]string{"source": `ws.moduleSource(path: "/.dagger/modules/helper").asModule.name`}},
		})
		for _, want := range []string{
			// A member reference describes the member.
			"List.reduce(initial: b) { acc: b, item: a => b }: b",
			// A type error, at its position in the snippet.
			"<check>:1:1: error: ",
			"1 error · workspace scope · nothing ran",
			// Search.
			`search "accumulator":`,
			// A module's diagnostics, and its private helper with its
			// declaration site, workspace-relative.
			"ok · module notes (.dagger/modules/notes) · 1 file · nothing ran",
			"shout(name: String!): String!",
			"declared at .dagger/modules/notes/main.dang:34:5",
			// A snippet checked inside a function body sees its locals.
			"String! has ",
			// ws is the conversation's workspace; the read is an engine call.
			"reading\n",
			`String! = "hi from the workspace\n"`,
			// Objects stay lazy.
			"(not evaluated; select a field such as .sync",
			// A module's private helper runs.
			`String! = "DANG"`,
			// A type error runs nothing, and is reported as the tool's
			// result, not a tool error.
			"[Tool result]: <check>:2:1: error: \"nope\" not found",
			// An installed module is callable without `at`.
			`String! = "hello, X"`,
			// The workspace's modules load from a snippet.
			`moduleName: "notes"`,
			`String! = "helper"`,
		} {
			require.Contains(t, out, want)
		}
		require.NotContains(t, out, `"notes" not found`)
		require.NotContains(t, out, "PermissionDenied")
		require.Regexp(t, `ok · \d+ms · 0 engine calls · workspace scope`, out)
		require.Regexp(t, `ok · [\d.]+m?s · [1-9]\d* engine calls? · workspace scope`, out)
		require.NotContains(t, out, "never\n")
		require.False(t, strings.Contains(out, "/tmp/"), "paths are workspace-relative:\n%s", out)
	})

	t.Run("bound tool objects", func(ctx context.Context, t *testctx.T) {
		// The bound Notes has a greeting its constructor doesn't give it:
		// `notes` in a snippet is the bound object, as of the call, not a
		// fresh one.
		out := runDangToolCalls(ctx, t, c, base, daggerShell,
			bind+` | with-tools $(notes | with-greeting "yo, ")`,
			[]dangToolCall{
				{"DangCheck", map[string]string{"source": "notes"}},
				{"DangEval", map[string]string{"source": `notes.greet(name: "x")`}},
				// A call that returns new state returns it; the binding
				// keeps its own.
				{"DangEval", map[string]string{"source": `notes.withGreeting(greeting: "hi, ").greeting`}},
				{"DangEval", map[string]string{"source": `notes.greeting`}},
			})
		for _, want := range []string{
			"notes: Dagger.Notes!",
			`String! = "yo, X"`,
			`String! = "hi, "`,
			`String! = "yo, "`,
		} {
			require.Contains(t, out, want)
		}
	})
}
