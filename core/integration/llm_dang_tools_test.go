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
	return runDangToolCallsThen(ctx, t, c, base, shell, setup, calls, "")
}

// runDangToolCallsThen is runDangToolCalls, then runs the shell statement
// then in the same session, its output following the transcript's.
func runDangToolCallsThen(ctx context.Context, t *testctx.T, c *dagger.Client, base *core.Container, shell func(string) core.WithContainerFunc, setup string, calls []dangToolCall, then string) string {
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
	statements := fmt.Sprintf(`llm --model="%s" | %s | with-prompt "%s" | loop | transcript`, model, setup, prompt)
	if then != "" {
		statements += "\n" + then
	}
	out, err := base.With(shell(statements)).Stdout(ctx)
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
			// Assignment is the write.
			{"DangEval", map[string]string{"source": `ws = ws.withNewFile(path: "written.txt", contents: "w")`}},
			{"DangEval", map[string]string{"source": `ws.file("written.txt").contents`}},
			// A failure writes nothing.
			{"DangEval", map[string]string{"source": "ws = ws.withNewFile(path: \"nope.txt\", contents: \"x\")\nraise \"boom\""}},
			{"DangEval", map[string]string{"source": `ws.directory("/").entries.contains("nope.txt")`}},
			// A workspace only returned is not written.
			{"DangEval", map[string]string{"source": `ws.withNewFile(path: "draft.txt", contents: "d")`}},
			// ws is bound, and written, in a module's scope too.
			{"DangEval", map[string]string{"at": "notes", "source": `ws = ws.withNewFile(path: "from-module.txt", contents: shout("m"))`}},
			{"DangEval", map[string]string{"source": `ws.file("from-module.txt").contents`}},
			// A value read before ws is reassigned keeps the one it read.
			{"DangEval", map[string]string{"source": "let before = ws\nws = ws.withNewFile(path: \"after.txt\", contents: \"a\")\n[before.directory(\"/\").entries.contains(\"after.txt\"), ws.directory(\"/\").entries.contains(\"after.txt\")]"}},
			{"DangEval", map[string]string{"source": `ws.file("after.txt").contents`}},
		})
		for _, want := range []string{
			// A member reference describes the member.
			"List.reduce(initial: b) { acc: b, item: a => b }: b",
			// A type error, at its position in the snippet, with what exists.
			"<check>:1:1: error: ",
			"  closest: node(id: ID!)",
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
			// Writes.
			"ws written (1 file)",
			`String! = "w"`,
			"(not applied; to write it, assign it: ws = …)",
			"0 writes",
			`String! = "M"`,
			// before and ws, side by side; ws is written.
			"[Boolean!]! = [false, true]",
			`String! = "a"`,
		} {
			require.Contains(t, out, want)
		}
		require.Regexp(t, `contains\(\\"nope.txt\\"\)"\}\)\s+\[Tool result\]: Boolean! = false`, out)
		require.NotContains(t, out, `"notes" not found`)
		require.NotContains(t, out, "PermissionDenied")
		require.Regexp(t, `ok · \d+ms · 0 engine calls · workspace scope`, out)
		require.Regexp(t, `ok · [\d.]+m?s · [1-9]\d* engine calls? · workspace scope`, out)
		require.NotContains(t, out, "never\n")
		require.False(t, strings.Contains(out, "/tmp/"), "paths are workspace-relative:\n%s", out)
	})

	t.Run("long results and the skill nudge", func(ctx context.Context, t *testctx.T) {
		const nudge = "See the dang-language skill (ReadSkill) for Dang's syntax and semantics."
		out := runDangToolCalls(ctx, t, c, base, daggerShellNoMod, bind, []dangToolCall{
			// Source that doesn't parse or check points at the skill.
			{"DangEval", map[string]string{"source": "nope"}},
			{"DangCheck", map[string]string{"source": "[1,"}},
			// A failure at runtime doesn't.
			{"DangEval", map[string]string{"source": `raise "boom"`}},
			// A long result is bounded as a module tool's output is, its
			// middle in the tool call's logs.
			{"DangEval", map[string]string{"source": `container.from("alpine").withExec(["seq", "1", "20000"]).stdout`}},
			// Once the skill is read, the nudge stops.
			{"ReadSkill", map[string]string{"name": "dang-language"}},
			{"DangEval", map[string]string{"source": "nope"}},
		})
		require.Equal(t, 2, strings.Count(out, nudge), out)
		require.Contains(t, out, "boom")
		require.Contains(t, out, "String! = \"\"\"\n1\n2\n3\n")
		require.Contains(t, out, "\n20000\n\"\"\"\n")
		require.Regexp(t, `\.\.\. \d+ lines omitted \(use ReadLogs\(span: [0-9a-f]+\) to read more\) \.\.\.`, out)
		require.NotContains(t, out, "chars omitted")
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
				// Assigning it is what rebinds it, for later calls.
				{"DangEval", map[string]string{"source": `notes = notes.withGreeting(greeting: "hey, ")`}},
				{"DangEval", map[string]string{"source": `notes.greet(name: "x")`}},
				// In a module's scope too.
				{"DangEval", map[string]string{"at": "notes", "source": `notes = notes.withGreeting(greeting: shout("o, "))`}},
				{"DangEval", map[string]string{"source": `notes.greeting`}},
			})
		for _, want := range []string{
			"notes: Dagger.Notes!",
			`String! = "yo, X"`,
			`String! = "hi, "`,
			`String! = "yo, "`,
			"notes state updated",
			`String! = "hey, X"`,
			`String! = "O, "`,
		} {
			require.Contains(t, out, want)
		}
	})

	t.Run("a bound tool object from a git module", func(ctx context.Context, t *testctx.T) {
		// Agent tools are usually remote modules. A snippet that loads one
		// loads its git module source in a client that has no host to look
		// for the caller's .env on: from its ID, when the conversation was
		// resumed and it isn't cached, or by asking for it.
		helper := base.Directory(".dagger/modules/helper")
		svc, _ := gitSmartHTTPServiceDirAuth(ctx, t, c, "", makeGitDir(c, helper, "main"), "", nil)
		repo := "http://" + startedServiceIP(ctx, t, c, svc) + "/repo.git"
		remote := base.WithNewFile("dagger.toml", fmt.Sprintf(
			"[modules.notes]\nsource = %q\n\n[modules.helper]\nsource = %q\n", ".dagger/modules/notes", repo+"@main"))
		out := runDangToolCalls(ctx, t, c, remote, daggerShell, bind+` | with-tools $(helper)`,
			[]dangToolCall{
				{"DangEval", map[string]string{"source": `helper.ping`}},
				// Not loaded before: this snippet's client loads it.
				{"DangEval", map[string]string{"source": fmt.Sprintf(`moduleSource(refString: %q).asModule.name`, repo)}},
			})
		require.Contains(t, out, `String! = "pong"`)
		require.Contains(t, out, `String! = "helper"`)
		require.NotContains(t, out, "PermissionDenied")
		require.NotContains(t, out, "failed to load")
	})

	t.Run("a host secret in a git tool module", func(ctx context.Context, t *testctx.T) {
		// A git tool module holding a secret its settings read from the
		// caller's host, as agent tools (a GitHub token) do. The agent's
		// tool calls and a snippet reading the bound object both read it,
		// and a snippet's own secret, made in a client with no host, doesn't
		// keep the conversation's client from reading the same one after.
		vault := core.NewQuery(c).Directory().
			WithNewFile("dagger-module.toml", "name = \"vault\"\nengineVersion = \"latest\"\n\n[runtime]\n  source = \"dang\"\n").
			WithNewFile("main.dang", `type Vault {
  pub token: Secret!

  new(token: Secret!) {
    self.token = token
    self
  }

  pub reveal: String! {
    token.plaintext
  }
}
`)
		svc, _ := gitSmartHTTPServiceDirAuth(ctx, t, c, "", makeGitDir(c, vault, "main"), "", nil)
		repo := "http://" + startedServiceIP(ctx, t, c, svc) + "/repo.git"
		withVault := base.
			WithEnvVariable("VAULT_TOKEN", "from-the-host").
			WithNewFile("dagger.toml", fmt.Sprintf(
				"[modules.notes]\nsource = %q\n\n[modules.vault]\nsource = %q\n\n[modules.vault.settings]\ntoken = \"env://VAULT_TOKEN\"\n",
				".dagger/modules/notes", repo+"@main"))
		out := runDangToolCallsThen(ctx, t, c, withVault, daggerShell, bind+` | with-tools $(vault)`,
			[]dangToolCall{
				{"reveal", map[string]string{}},
				{"DangEval", map[string]string{"source": `vault.reveal`}},
				{"DangEval", map[string]string{"source": `secret(uri: "env://VAULT_TOKEN").uri`}},
				{"reveal", map[string]string{}},
			},
			// The conversation's own client, after the snippets.
			"vault | reveal")
		require.Equal(t, 2, strings.Count(out, "[Tool result]: from-the-host"), out)
		require.Contains(t, out, `String! = "from-the-host"`)
		require.Contains(t, out, `String! = "env://VAULT_TOKEN"`)
		require.True(t, strings.HasSuffix(strings.TrimSpace(out), "from-the-host"), out)
	})

	t.Run("installed modules of a snapshot workspace", func(ctx context.Context, t *testctx.T) {
		// `dagger agent` binds a snapshot of the workspace: a value, whose
		// conversation schema is core only. The Dang tools still see the
		// modules it installs, loaded from its own tree, a git module
		// included, and `at` names one whose source isn't in the tree.
		helper := base.Directory(".dagger/modules/helper")
		svc, _ := gitSmartHTTPServiceDirAuth(ctx, t, c, "", makeGitDir(c, helper, "main"), "", nil)
		repo := "http://" + startedServiceIP(ctx, t, c, svc) + "/repo.git"
		remote := base.
			WithNewFile("dagger.toml", fmt.Sprintf(
				"[modules.notes]\nsource = %q\n\n[modules.helper]\nsource = %q\n", ".dagger/modules/notes", repo+"@main")).
			WithExec([]string{"git", "add", "."}).
			WithExec([]string{"git", "commit", "-m", "install helper"})
		out := runDangToolCalls(ctx, t, c, remote, daggerShellNoMod,
			`with-workspace --workspace $(current-workspace | snapshot)`,
			[]dangToolCall{
				{"DangCheck", map[string]string{"source": `notes.greet(name: "x")`}},
				{"DangEval", map[string]string{"source": `notes.greet(name: "x")`}},
				{"DangCheck", map[string]string{"source": `helper.ping`}},
				{"DangCheck", map[string]string{"search": "only snippets load"}},
				{"DangEval", map[string]string{"source": `helper.ping`}},
				{"DangCheck", map[string]string{"at": "helper"}},
				{"DangCheck", map[string]string{"at": "helper", "source": "Helper"}},
				{"DangEval", map[string]string{"at": "helper", "source": `Helper.ping + "!"`}},
				{"DangCheck", map[string]string{"at": "nope", "source": "1"}},
			})
		for _, want := range []string{
			`String! = "hello, X"`,
			"String! has ",
			"Dagger.Helper.ping: String!",
			"  type Helper — A module the workspace doesn't install",
			`String! = "pong"`,
			fmt.Sprintf("ok · module helper (%s@main) · 1 file · nothing ran", repo),
			"type Helper\n",
			`String! = "pong!"`,
			fmt.Sprintf("module helper (%s@main)", repo),
			`error: at "nope": nope is neither a module installed in the workspace nor one of its directories (installed: helper, notes)`,
		} {
			require.Contains(t, out, want)
		}
		require.NotContains(t, out, `"notes" not found`)
		require.NotContains(t, out, `"helper" not found`)
		require.NotContains(t, out, "no such file or directory")
	})

	t.Run("a module installed by a snippet", func(ctx context.Context, t *testctx.T) {
		// A module a snippet installs (`ws = ws.withModule(…)`) is in the
		// next call's scope, and one it uninstalls isn't, in a live workspace
		// and in a snapshot.
		helper := base.Directory(".dagger/modules/helper")
		svc, _ := gitSmartHTTPServiceDirAuth(ctx, t, c, "", makeGitDir(c, helper, "main"), "", nil)
		repo := "http://" + startedServiceIP(ctx, t, c, svc) + "/repo.git"
		committed := base.
			WithExec([]string{"git", "add", "."}).
			WithExec([]string{"git", "commit", "-m", "initial"})
		calls := []dangToolCall{
			{"DangCheck", map[string]string{"source": `helper.ping`}},
			{"DangEval", map[string]string{"source": fmt.Sprintf(`ws = ws.withModule(ref: %q)`, repo+"@main")}},
			{"DangCheck", map[string]string{"source": `helper.ping`}},
			{"DangEval", map[string]string{"source": `helper.ping`}},
			{"DangEval", map[string]string{"at": "helper", "source": `Helper.ping + "?"`}},
			// One it uninstalls leaves it, even if the session serves it.
			{"DangCheck", map[string]string{"source": `notes.greet(name: "x")`}},
			{"DangEval", map[string]string{"source": `ws = ws.withoutModule(name: "notes")`}},
			{"DangCheck", map[string]string{"source": `notes.greet(name: "x")`}},
		}
		for name, bind := range map[string]string{
			"live":     bind,
			"snapshot": `with-workspace --workspace $(current-workspace | snapshot)`,
		} {
			out := runDangToolCalls(ctx, t, c, committed, daggerShellNoMod, bind, calls)
			for _, want := range []string{
				// Not installed yet.
				`<check>:1:1: error: "helper" not found`,
				"ws written",
				"Dagger.Helper.ping: String!",
				`String! = "pong"`,
				`String! = "pong?"`,
				"String! has ",
				`<check>:1:1: error: "notes" not found`,
			} {
				require.Contains(t, out, want, name)
			}
		}
	})

	t.Run("a module broken by a snippet", func(ctx context.Context, t *testctx.T) {
		// An edit that breaks an installed module's source takes it out of
		// the scope, rather than leaving the copy the session served, and
		// says why when a snippet names it.
		out := runDangToolCalls(ctx, t, c, base, daggerShellNoMod, bind, []dangToolCall{
			{"DangEval", map[string]string{"source": `notes.greet(name: "x")`}},
			{"DangEval", map[string]string{"source": `ws = ws.withNewFile(path: ".dagger/modules/notes/main.dang", contents: "type Notes {\n")`}},
			{"DangCheck", map[string]string{"source": `notes.greet(name: "x")`}},
		})
		require.Contains(t, out, `String! = "hello, X"`)
		require.Contains(t, out, `<check>:1:1: error: "notes" not found`)
		require.Contains(t, out, "note: installed modules that failed to load aren't in scope: notes (")
	})

	t.Run("a commit through ws is written", func(ctx context.Context, t *testctx.T) {
		// A commit moves HEAD without leaving a file delta: writing ws makes
		// the committed workspace the conversation's, as a tool returning it
		// would, rather than diffing files. The snippet's client has no host,
		// so the workspace is a snapshot and the author is explicit.
		committed := base.
			WithExec([]string{"git", "add", "."}).
			WithExec([]string{"git", "commit", "-m", "initial"})
		out := runDangToolCalls(ctx, t, c, committed, daggerShellNoMod,
			`with-workspace --workspace $(current-workspace | snapshot)`,
			[]dangToolCall{
				{"DangEval", map[string]string{"source": `ws = ws.withCommit(changes: ws.withNewFile(path: "committed.txt", contents: "c").changes, message: "commit from dang", date: "2026-01-01T00:00:00Z", authorName: "Dang", authorEmail: "dang@example.com")`}},
				{"DangEval", map[string]string{"source": `ws.git.head.log(limit: 1).{{message}}`}},
				{"DangEval", map[string]string{"source": `ws.file("committed.txt").contents`}},
			})
		require.Contains(t, out, "ws written")
		require.NotContains(t, out, "ws unchanged")
		require.Contains(t, out, `message: "commit from dang`)
		require.Contains(t, out, `String! = "c"`)
	})
}
