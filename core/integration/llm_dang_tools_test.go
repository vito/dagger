package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"dagger.io/dagger/core"

	"github.com/dagger/testctx"
	"github.com/stretchr/testify/require"
)

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

	calls := []struct {
		tool string
		args map[string]string
	}{
		{"DangCheck", map[string]string{"source": `["a"].reduce`}},
		{"DangCheck", map[string]string{"source": "nope"}},
		{"DangCheck", map[string]string{"search": "accumulator"}},
		{"DangCheck", map[string]string{"at": "notes"}},
		{"DangCheck", map[string]string{"at": "notes", "source": "shout"}},
		{"DangCheck", map[string]string{"at": ".dagger/modules/notes/main.dang:10:5", "source": "greeting + name"}},
		{"DangEval", map[string]string{"source": "print(\"reading\")\nws.file(\"notes.txt\").contents"}},
		{"DangEval", map[string]string{"source": `container.from("alpine")`}},
		{"DangEval", map[string]string{"at": "notes", "source": `shout("dang")`}},
		{"DangEval", map[string]string{"source": "print(\"never\")\nnope"}},
		// The workspace scope runs against the core API: an installed
		// module is a type error, not a runtime failure.
		{"DangEval", map[string]string{"source": `notes.greet(name: "x")`}},
	}
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
	out, err := base.With(daggerShellNoMod(fmt.Sprintf(
		`llm --model="%s" | %s | with-prompt "%s" | loop | transcript`, model, bind, prompt,
	))).Stdout(ctx)
	require.NoError(t, err)

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
		"declared at .dagger/modules/notes/main.dang:17:5",
		// A snippet checked inside a function body sees its locals.
		"String! has ",
		// ws is the conversation's workspace; the read is an engine call.
		"reading\n",
		`String! = "hi from the workspace\n"`,
		// Objects stay lazy.
		"(not evaluated; select a field such as .sync",
		// A module's private helper runs.
		`String! = "DANG"`,
		// A type error runs nothing.
		"nothing ran",
		`<check>:1:1: error: "notes" not found`,
		"done",
	} {
		require.Contains(t, out, want)
	}
	require.Regexp(t, `ok · \d+ms · 0 engine calls · workspace scope`, out)
	require.Regexp(t, `ok · [\d.]+m?s · [1-9]\d* engine calls? · workspace scope`, out)
	require.NotContains(t, out, "never\n")
	require.False(t, strings.Contains(out, "/tmp/"), "paths are workspace-relative:\n%s", out)
}
