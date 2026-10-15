package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"dagger.io/dagger/core"

	"dagger.io/dagger"
	"github.com/dagger/testctx"
	"github.com/stretchr/testify/require"
)

const recomposeModulePath = ".dagger/modules/swapper/main.dang"

// Private fields deliberately cannot be recovered through public GraphQL
// selections. Recomposition must carry their serialized state to the new code.
const recomposeSource = `
type Swapper {
  let state: Int! = 0
  let notes: Map[String!]! = ["seed": "constructor"]

  agent(base: LLM!): LLM! @agent {
    base.withTools(currentNode)
  }

  advance: Swapper! {
    state += 1
    if (state == 1) {
      self.notes = notes.with("memo", "kept")
    }
    self
  }

  """Read the original private state."""
  readState: String! {
    "private counter: " + toString(state) + "; notes: " + JSON.encode(notes)
  }

  reload(llm: LLM!, me: Agent!): LLM! {
    me.recompose(conversation: llm)
  }
%s
}
`

func recomposeFixture(c *dagger.Client, source string) *core.Directory {
	return core.NewQuery(c).Directory().
		WithNewFile("dagger.toml", "[modules.swapper]\nsource = \".dagger/modules/swapper\"\n").
		WithNewFile(".dagger/modules/swapper/dagger.json", `{"name":"swapper","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile(recomposeModulePath, source)
}

// Composition runs as an agent, which every entry's Agent arguments receive.
// Use the live schema so these regressions do not depend on SDK regeneration.

// discoverExpertise converts ws's nominated (@agent) entries, optionally
// limited to include patterns.
func discoverExpertise(ctx context.Context, c *dagger.Client, ws *core.Workspace, include ...string) ([]core.ID, error) {
	wsID, err := ws.ID(ctx)
	if err != nil {
		return nil, err
	}
	var selected struct {
		Node struct {
			Artifacts struct {
				FilterTypes struct{ AsExpertise []struct{ ID core.ID } }
			}
		}
	}
	err = c.Do(ctx, &dagger.Request{
		Query: `query($workspace: ID!, $include: [String!]) {
   node(id: $workspace) { ... on Workspace { artifacts(include: $include) { filterTypes(types: ["Expertise"]) { asExpertise { id } } } } }
  }`,
		Variables: map[string]any{"workspace": wsID, "include": include},
	}, &dagger.Response{Data: &selected})
	if err != nil {
		return nil, err
	}
	ids := make([]core.ID, 0, len(selected.Node.Artifacts.FilterTypes.AsExpertise))
	for _, entry := range selected.Node.Artifacts.FilterTypes.AsExpertise {
		ids = append(ids, entry.ID)
	}
	return ids, nil
}

// spawnComposed spawns an agent from base, composed with ids, and returns
// the agent with its composed seed.
func spawnComposed(ctx context.Context, t *testctx.T, c *dagger.Client, base *core.LLM, ids []core.ID) (*agentHandle, *core.LLM, error) {
	t.Helper()
	baseID, err := base.ID(ctx)
	if err != nil {
		return nil, nil, err
	}
	if ids == nil {
		ids = []core.ID{}
	}
	var result struct{ Node struct{ Spawn string } }
	if err := c.Do(ctx, &dagger.Request{
		Query:     `query($base: ID!, $expertise: [ID!]!) { node(id: $base) { ... on LLM { spawn(expertise: $expertise) } } }`,
		Variables: map[string]any{"base": baseID, "expertise": ids},
	}, &dagger.Response{Data: &result}); err != nil {
		return nil, nil, err
	}
	agent := &agentHandle{c: c, agentID: result.Node.Spawn}
	out, err := agent.run(ctx, t, "snapshot { id }")
	if err != nil {
		return nil, nil, err
	}
	return agent, core.Ref[*core.LLM](core.NewQuery(c), core.ID(out.Get("snapshot.id").String())), nil
}

// composeRecomposeFixture spawns an agent composed with ws's nominated
// entries, from base or a fresh conversation bound to ws.
func composeRecomposeFixture(ctx context.Context, t *testctx.T, c *dagger.Client, ws *core.Workspace, base *core.LLM, include ...string) (*agentHandle, *core.LLM) {
	t.Helper()
	if base == nil {
		base = core.NewQuery(c).LLM().WithWorkspace(ws)
	}
	ids, err := discoverExpertise(ctx, c, ws, include...)
	require.NoError(t, err)
	agent, llm, err := spawnComposed(ctx, t, c, base, ids)
	require.NoError(t, err)
	return agent, llm
}

// composeAs runs Agent.compose: the entries are recorded, or refreshed in
// place when already recorded.
func composeAs(ctx context.Context, agent *agentHandle, conversation *core.LLM, ids []core.ID) (*core.LLM, error) {
	if ids == nil {
		ids = []core.ID{}
	}
	return agentComposition(ctx, agent, conversation, `compose(expertise: $expertise, conversation: $llm)`, map[string]any{"expertise": ids})
}

// recomposeAs runs Agent.recompose: every recorded entry is refreshed.
func recomposeAs(ctx context.Context, agent *agentHandle, conversation *core.LLM) (*core.LLM, error) {
	return agentComposition(ctx, agent, conversation, `recompose(conversation: $llm)`, nil)
}

func agentComposition(ctx context.Context, agent *agentHandle, conversation *core.LLM, selection string, vars map[string]any) (*core.LLM, error) {
	llmID, err := conversation.ID(ctx)
	if err != nil {
		return nil, err
	}
	decls := "$agent: ID!, $llm: ID!"
	if vars == nil {
		vars = map[string]any{}
	} else {
		decls += ", $expertise: [ID!]!"
	}
	vars["agent"] = agent.agentID
	vars["llm"] = llmID
	var result struct {
		Node struct{ Result struct{ ID core.ID } }
	}
	if err := agent.c.Do(ctx, &dagger.Request{
		Query:     fmt.Sprintf(`query(%s) { node(id: $agent) { ... on Agent { result: %s { id } } } }`, decls, selection),
		Variables: vars,
	}, &dagger.Response{Data: &result}); err != nil {
		return nil, err
	}
	return core.Ref[*core.LLM](core.NewQuery(agent.c), result.Node.Result.ID), nil
}

// recomposeLLM binds ws, refreshes base's record there, then composes the
// entries discovered in ws, refreshing recorded ones in place: what an
// install does.
func recomposeLLM(ctx context.Context, t *testctx.T, agent *agentHandle, ws *core.Workspace, base *core.LLM, include ...string) (*core.LLM, error) {
	t.Helper()
	llm := base.WithWorkspace(ws)
	if len(recordedExpertise(ctx, t, agent.c, llm)) > 0 {
		var err error
		llm, err = recomposeAs(ctx, agent, llm)
		if err != nil {
			return nil, err
		}
	}
	ids, err := discoverExpertise(ctx, agent.c, ws, include...)
	if err != nil {
		return nil, err
	}
	return composeAs(ctx, agent, llm, ids)
}

// composeLLM binds ws and composes the entries discovered there, without
// refreshing the rest of the record.
func composeLLM(ctx context.Context, agent *agentHandle, ws *core.Workspace, base *core.LLM, include ...string) (*core.LLM, error) {
	ids, err := discoverExpertise(ctx, agent.c, ws, include...)
	if err != nil {
		return nil, err
	}
	return composeAs(ctx, agent, base.WithWorkspace(ws), ids)
}

// refreshLLM binds ws and recomposes: every recorded entry is found again in
// ws, or reruns from its recorded source.
func refreshLLM(ctx context.Context, agent *agentHandle, ws *core.Workspace, base *core.LLM) (*core.LLM, error) {
	return recomposeAs(ctx, agent, base.WithWorkspace(ws))
}

// composeURI binds ws and composes the entries a URI selects there.
func composeURI(ctx context.Context, agent *agentHandle, ws *core.Workspace, base *core.LLM, uri string) (*core.LLM, error) {
	ids, err := uriExpertiseIDs(ctx, agent.c, ws, uri)
	if err != nil {
		return nil, err
	}
	return composeAs(ctx, agent, base.WithWorkspace(ws), ids)
}

// spawnURI spawns an agent from base bound to ws, composed with the entries
// a URI selects there.
func spawnURI(ctx context.Context, t *testctx.T, c *dagger.Client, ws *core.Workspace, base *core.LLM, uri string) (*agentHandle, *core.LLM) {
	t.Helper()
	ids, err := uriExpertiseIDs(ctx, c, ws, uri)
	require.NoError(t, err)
	agent, llm, err := spawnComposed(ctx, t, c, base.WithWorkspace(ws), ids)
	require.NoError(t, err)
	return agent, llm
}

// restoreComposer restores agent's runtime in another client's session, under
// its handle and name, from a conversation recipe captured from it, and
// returns the restored agent with its seed: the conversation, owned by the
// restored agent. An agent must be present in a session to compose there.
func restoreComposer(ctx context.Context, t *testctx.T, target *dagger.Client, agent *agentHandle, recipe core.ID) (*agentHandle, *core.LLM) {
	t.Helper()
	info := agent.mustRun(ctx, t, "handle name")
	restored, err := rehydrateAgent(ctx, target, string(recipe), info.Get("handle").String(), info.Get("name").String(), "IDLE", "")
	require.NoError(t, err)
	seed := restored.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()
	return restored, core.Ref[*core.LLM](core.NewQuery(target), core.ID(seed))
}

func uriExpertiseIDs(ctx context.Context, c *dagger.Client, ws *core.Workspace, uri string) ([]core.ID, error) {
	wsID, err := ws.ID(ctx)
	if err != nil {
		return nil, err
	}
	var res struct {
		Node struct {
			Artifacts struct {
				FilterURI struct{ AsExpertise []struct{ ID core.ID } }
			}
		}
	}
	err = c.Do(ctx, &dagger.Request{
		Query: `query($ws: ID!, $uri: String!) {
			node(id: $ws) { ... on Workspace { artifacts { filterUri(uri: $uri) { asExpertise { id } } } } }
		}`,
		Variables: map[string]any{"ws": wsID, "uri": uri},
	}, &dagger.Response{Data: &res})
	if err != nil {
		return nil, err
	}
	var ids []core.ID
	for _, entry := range res.Node.Artifacts.FilterURI.AsExpertise {
		ids = append(ids, entry.ID)
	}
	return ids, nil
}

type recordedExpertiseEntry struct {
	Artifact struct{ URI string }
}

func recordedExpertise(ctx context.Context, t *testctx.T, c *dagger.Client, llm *core.LLM) []recordedExpertiseEntry {
	t.Helper()
	id, err := llm.ID(ctx)
	require.NoError(t, err)
	var res struct {
		Node struct{ Expertise []recordedExpertiseEntry }
	}
	require.NoError(t, c.Do(ctx, &dagger.Request{
		Query:     `query($id: ID!) { node(id: $id) { ... on LLM { expertise { artifact { uri } } } } }`,
		Variables: map[string]any{"id": id},
	}, &dagger.Response{Data: &res}))
	return res.Node.Expertise
}

// Refresh the composition record in ws with recompose's own URI resolution,
// not old Expertise IDs or compose-all discovery (which would install the chief).
func recomposeRecordedExpertise(ctx context.Context, t *testctx.T, agent *agentHandle, ws *core.Workspace, llm *core.LLM) *core.LLM {
	t.Helper()
	result, err := refreshLLM(ctx, agent, ws, llm)
	require.NoError(t, err)
	return result
}

func recomposeTool(id, name string) core.LLMContentBlockInput {
	return core.LLMContentBlockInput{Kind: core.LLMContentBlockKindToolCall, CallID: id, ToolName: name}
}

func recomposeRecordingTurn(llm *core.LLM, prompt string, tools ...string) *core.LLM {
	llm = llm.WithPrompt(prompt)
	for i, tool := range tools {
		call := recomposeTool(fmt.Sprintf("%s_%d", prompt, i), tool)
		llm = llm.WithResponse([]core.LLMContentBlockInput{call}).WithToolResult(call.CallID, "", false)
	}
	return llm.WithResponse([]core.LLMContentBlockInput{{Kind: core.LLMContentBlockKindText, Text: prompt + " done"}})
}

func (LLMSuite) TestRecomposePrivateStateAndReplay(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	initial := fmt.Sprintf(recomposeSource, "")
	updated := strings.ReplaceAll(fmt.Sprintf(recomposeSource, `
  let addedDefault: String! = "new field default"

  """A newly installed method with new documentation."""
  added: String! {
    addedDefault + "; updated counter: " + toString(state)
  }
`), "Read the original private state.", "Read the updated private state.")
	fixture := recomposeFixture(c, initial)
	ws := fixture.AsWorkspace()
	script := recomposeRecordingTurn(core.NewQuery(c).LLM(), "mutate", "advance", "readState")
	script = recomposeRecordingTurn(script, "unchanged", "readState")
	script = recomposeRecordingTurn(script, "edited", "added", "advance", "added", "readState")
	script = recomposeRecordingTurn(script, "restored", "advance", "added", "readState")
	script = recomposeRecordingTurn(script, "installed", "added", "extraTool", "readState")
	model := cannedRecordingModel(ctx, t, c, script)
	mapPattern := regexp.MustCompile(`"memo"\s*:\s*"kept"`)
	agent, llm := composeRecomposeFixture(ctx, t, c, ws, core.NewQuery(c).LLM(core.LLMOpts{Model: model}).WithWorkspace(ws))
	llm = llm.WithPrompt("mutate").Loop()
	transcript, err := llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "private counter: 1")
	require.Regexp(t, `"memo"\s*:\s*"kept"`, transcript)

	llm, err = recomposeLLM(ctx, t, agent, ws, llm)
	require.NoError(t, err)
	llm = llm.WithPrompt("unchanged").Loop()
	transcript, err = llm.Transcript(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(transcript, "private counter: 1"))
	require.Len(t, mapPattern.FindAllString(transcript, -1), 2)

	// Edit the existing workspace overlay, as the editor does: rebuilding a
	// Directory-backed workspace would give the module a different origin.
	// Agent.recompose finds the recorded entry again there.
	ws = ws.WithNewFile(recomposeModulePath, updated)
	llm, err = refreshLLM(ctx, agent, ws, llm)
	require.NoError(t, err)
	tools, err := llm.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "## added\n")
	require.Contains(t, tools, "A newly installed method with new documentation.")
	require.Contains(t, tools, "Read the updated private state.")
	require.NotContains(t, tools, "Read the original private state.")
	llm = llm.WithPrompt("edited").Loop()
	transcript, err = llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "new field default; updated counter: 1")
	require.Contains(t, transcript, "new field default; updated counter: 2")
	require.Contains(t, transcript, "private counter: 2")
	require.Len(t, mapPattern.FindAllString(transcript, -1), 3)

	// Recompose twice, then replay the portable recipe in a fresh client with
	// no module schema served. The next same-type return must retain NEW code.
	for range 2 {
		llm, err = refreshLLM(ctx, agent, ws, llm)
		require.NoError(t, err)
	}
	portable, err := sink.captureLLMRecipe(ctx, t, c, llm)
	require.NoError(t, err)
	target := connect(ctx, t)
	restoredAgent, restored := restoreComposer(ctx, t, target, agent, portable)
	llm = restored.WithPrompt("restored").Loop()
	transcript, err = llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "new field default; updated counter: 3")
	require.Contains(t, transcript, "private counter: 3")
	require.Len(t, mapPattern.FindAllString(transcript, -1), 4)
	require.NotContains(t, transcript, "is not available")

	// Adding expertise is the install path: initialize only the new module,
	// not the existing module's counter/map or its previously added field.
	installed := llm.Workspace().
		WithNewFile("dagger.toml", "[modules.swapper]\nsource = \".dagger/modules/swapper\"\n[modules.extra]\nsource = \"modules/extra\"\n").
		WithNewFile("modules/extra/dagger.json", `{"name":"extra","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile("modules/extra/main.dang", `
type Extra {
  let marker: String! = "installed default"
  agent(base: LLM!): LLM! @agent { base.withTools(currentNode) }
  extraTool: String! { marker }
}
`)
	llm, err = recomposeLLM(ctx, t, restoredAgent, installed, llm)
	require.NoError(t, err)
	transcript, err = llm.WithPrompt("installed").Loop().Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "installed default")
	require.Equal(t, 2, strings.Count(transcript, "new field default; updated counter: 3"))
	require.Equal(t, 2, strings.Count(transcript, "private counter: 3"))
	require.Len(t, mapPattern.FindAllString(transcript, -1), 5)
}

func (LLMSuite) TestRecomposeRemoteToLocalStateAndReplay(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	const remotePrompt = "The remote swapper prompt."
	const localPrompt = "The locally installed swapper prompt."
	initial := strings.ReplaceAll(fmt.Sprintf(recomposeSource, `
  remoteOnly: String! { "remote implementation" }
`), "base.withTools(currentNode)", fmt.Sprintf("base.withSystemPrompt(%q).withTools(currentNode)", remotePrompt))
	updated := strings.ReplaceAll(fmt.Sprintf(recomposeSource, `
  let addedDefault: String! = "local field default"

  """A tool from the local installation."""
  added: String! { addedDefault + "; local counter: " + toString(state) }
`), "base.withTools(currentNode)", fmt.Sprintf("base.withSystemPrompt(%q).withTools(currentNode)", localPrompt))
	updated = strings.ReplaceAll(updated, "Read the original private state.", "Read the locally installed private state.")

	// Serve a real remote module without depending on an external repository.
	// Replacing its installation must not make source identity part of state
	// compatibility or ownership of the expertise's tools and prompts.
	remoteRef := workspaceSelectionRemoteRef(ctx, t, c, recomposeFixture(c, initial).Directory(".dagger/modules/swapper"))
	ws := core.NewQuery(c).Directory().WithNewFile("dagger.toml", fmt.Sprintf("[modules.swapper]\nsource = %q\n", remoteRef)).AsWorkspace()
	agent, llm := composeRecomposeFixture(ctx, t, c, ws, core.NewQuery(c).LLM().WithWorkspace(ws).WithSystemPrompt(remotePrompt))
	require.ElementsMatch(t, []string{remotePrompt, remotePrompt}, recomposeSystemPrompts(ctx, t, c, llm))
	model := cannedRecordingModel(ctx, t, c, recomposeRecordingTurn(llm, "mutate remote", "advance", "readState"))
	llm = llm.WithModel(model).WithPrompt("mutate remote").Loop()
	transcript, err := llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "private counter: 1")
	mapPattern := regexp.MustCompile(`"memo"\s*:\s*"kept"`)
	require.Len(t, mapPattern.FindAllString(transcript, -1), 1)

	// Keep the installed name and object type, but switch from a git source
	// to local code. This is not just an edit within the original origin.
	ws = ws.WithNewFile("dagger.toml", "[modules.swapper]\nsource = \".dagger/modules/swapper\"\n").
		WithNewFile(".dagger/modules/swapper/dagger.json", `{"name":"swapper","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile(recomposeModulePath, updated)
	llm, err = recomposeLLM(ctx, t, agent, ws, llm, "swapper")
	require.NoError(t, err)
	// The caller's identical prompt survives, while the remote-owned prompt
	// is replaced rather than left behind or duplicated.
	expectedPrompts := []string{remotePrompt, localPrompt}
	require.ElementsMatch(t, expectedPrompts, recomposeSystemPrompts(ctx, t, c, llm))
	tools, err := llm.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "## added\n")
	require.Contains(t, tools, "A tool from the local installation.")
	require.Contains(t, tools, "Read the locally installed private state.")
	require.NotContains(t, tools, "Read the original private state.")
	require.NotContains(t, tools, "## remoteOnly\n")

	model = cannedRecordingModel(ctx, t, c, recomposeRecordingTurn(llm, "use local", "added", "advance", "added", "readState"))
	llm = llm.WithModel(model).WithPrompt("use local").Loop()
	transcript, err = llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "local field default; local counter: 1")
	require.Contains(t, transcript, "local field default; local counter: 2")
	require.Contains(t, transcript, "private counter: 2")
	require.Len(t, mapPattern.FindAllString(transcript, -1), 2)

	// A same-type state return and a repeated recompose must keep local code
	// and ownership, including when the portable recipe is loaded elsewhere.
	llm, err = recomposeLLM(ctx, t, agent, ws, llm, "swapper")
	require.NoError(t, err)
	require.ElementsMatch(t, expectedPrompts, recomposeSystemPrompts(ctx, t, c, llm))
	model = cannedRecordingModel(ctx, t, c, recomposeRecordingTurn(llm, "replay local", "advance", "added", "readState"))
	portable, err := sink.captureLLMRecipe(ctx, t, c, llm.WithModel(model))
	require.NoError(t, err)
	target := connect(ctx, t)
	llm = core.Ref[*core.LLM](core.NewQuery(target), portable)
	require.ElementsMatch(t, expectedPrompts, recomposeSystemPrompts(ctx, t, target, llm))
	transcript, err = llm.WithPrompt("replay local").Loop().Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "local field default; local counter: 3")
	require.Contains(t, transcript, "private counter: 3")
	require.Len(t, mapPattern.FindAllString(transcript, -1), 3)
	require.NotContains(t, transcript, "is not available")
}

// Bumping a remote module's pin in a live workspace's pending dagger.lock and
// reloading must load the newly pinned commit, not the session's served one.
func (LLMSuite) TestRecomposeOverlayLockRepinsRemoteModule(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	initial := fmt.Sprintf(recomposeSource, "")
	bumped := strings.ReplaceAll(initial, "Read the original private state.", "Read the bumped private state.")

	// A remote module with two commits on main, without depending on an
	// external repository: A, then B at the branch head.
	repo := core.NewQuery(c).Container().From(alpineImage).
		WithExec([]string{"apk", "add", "git"}).
		WithDirectory("/repo", recomposeFixture(c, initial).Directory(".dagger/modules/swapper")).
		WithWorkdir("/repo").
		WithExec([]string{"sh", "-c", `set -e
git init -q -b main
git -c user.email=root@localhost -c user.name=Test add -A
git -c user.email=root@localhost -c user.name=Test commit -q -m A`}).
		WithNewFile("/repo/main.dang", bumped).
		WithExec([]string{"sh", "-c", `git -c user.email=root@localhost -c user.name=Test commit -q -am B`})
	commits, err := repo.WithExec([]string{"git", "rev-parse", "HEAD~1", "HEAD"}).Stdout(ctx)
	require.NoError(t, err)
	shas := strings.Fields(commits)
	require.Len(t, shas, 2)
	commitA, commitB := shas[0], shas[1]
	remoteRef := workspaceSelectionRemoteRef(ctx, t, c, repo.Directory("/repo"))

	config := fmt.Sprintf("[modules.swapper]\nsource = %q\n", remoteRef)
	// A live, host-backed workspace: overlays on it re-resolve only what they
	// touch, on top of the modules the session serves from disk.
	base := workspaceBase(t, c).WithNewFile("dagger.toml", config)
	const selectCommit = `artifacts(include: ["swapper"]) { filterTypes(types: ["Expertise"]) { asExpertise { originalModule { source { commit } } } } }`
	commitOf := func(ctx context.Context, t *testctx.T, ctr *core.Container, query string, args ...any) string {
		t.Helper()
		out, err := ctr.With(daggerQuery(query, args...)).Stdout(ctx)
		require.NoError(t, err)
		switch {
		case strings.Contains(out, commitA) && !strings.Contains(out, commitB):
			return commitA
		case strings.Contains(out, commitB) && !strings.Contains(out, commitA):
			return commitB
		}
		t.Fatalf("expected exactly one of %s and %s: %s", commitA, commitB, out)
		return ""
	}

	// Let the engine write the lock for the branch head (B), then pin A on
	// disk: the served module follows the on-disk pin.
	locked := base.With(daggerQuery(`{ currentWorkspace { %s } }`, selectCommit))
	lockB, err := locked.File("dagger.lock").Contents(ctx)
	require.NoError(t, err)
	require.Contains(t, lockB, commitB)
	lockA := strings.ReplaceAll(lockB, commitB, commitA)
	base = base.WithNewFile("dagger.lock", lockA)
	require.Equal(t, commitA, commitOf(ctx, t, base, `{ currentWorkspace { %s } }`, selectCommit))

	t.Run("lock edit", func(ctx context.Context, t *testctx.T) {
		require.Equal(t, commitB, commitOf(ctx, t, base,
			`{ currentWorkspace { withNewFile(path: "dagger.lock", contents: %q) { %s } } }`, lockB, selectCommit))
	})

	t.Run("config and lock edit", func(ctx context.Context, t *testctx.T) {
		require.Equal(t, commitB, commitOf(ctx, t, base,
			`{ currentWorkspace { withNewFile(path: "dagger.toml", contents: %q) { withNewFile(path: "dagger.lock", contents: %q) { %s } } } }`,
			config+"# touched\n", lockB, selectCommit))
	})

	t.Run("config edit keeps the on-disk pin", func(ctx context.Context, t *testctx.T) {
		require.Equal(t, commitA, commitOf(ctx, t, base,
			`{ currentWorkspace { withNewFile(path: "dagger.toml", contents: %q) { %s } } }`,
			config+"# touched\n", selectCommit))
	})
}

func (LLMSuite) TestRecomposeFailureKeepsOldState(ctx context.Context, t *testctx.T) {
	for _, tc := range []struct{ name, source string }{
		{"broken source", "this is not a Dang module"},
		{"incompatible private map", strings.ReplaceAll(strings.ReplaceAll(
			fmt.Sprintf(recomposeSource, ""),
			`let notes: Map[String!]! = ["seed": "constructor"]`, `let notes: Map[Int!]! = ["seed": 42]`),
			`notes.with("memo", "kept")`, `notes.with("memo", 99)`)},
	} {
		t.Run(tc.name, func(ctx context.Context, t *testctx.T) {
			c := connect(ctx, t)
			fixture := recomposeFixture(c, fmt.Sprintf(recomposeSource, ""))
			ws := fixture.AsWorkspace()
			script := recomposeRecordingTurn(core.NewQuery(c).LLM(), "mutate", "advance")
			script = recomposeRecordingTurn(script, "still usable", "advance", "readState")
			model := cannedRecordingModel(ctx, t, c, script)
			agent, llm := composeRecomposeFixture(ctx, t, c, ws, core.NewQuery(c).LLM(core.LLMOpts{Model: model}).WithWorkspace(ws))
			llm = llm.WithPrompt("mutate").Loop()
			_, err := llm.Sync(ctx)
			require.NoError(t, err)
			// A recorded module that fails to load is an error, never a silent
			// fallback to its previously recorded source.
			_, err = refreshLLM(ctx, agent, ws.WithNewFile(recomposeModulePath, tc.source), llm)
			require.Error(t, err, "reload must not silently reset incompatible state")
			if tc.name == "incompatible private map" {
				require.Contains(t, err.Error(), "notes")
			}
			contents, err := llm.Workspace().File(recomposeModulePath).Contents(ctx)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf(recomposeSource, ""), contents)
			transcript, err := llm.WithPrompt("still usable").Loop().Transcript(ctx)
			require.NoError(t, err)
			require.Contains(t, transcript, "private counter: 2")
			require.Regexp(t, `"memo"\s*:\s*"kept"`, transcript)
		})
	}
}

func (LLMSuite) TestRecomposeStateVersion(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	// The same private-map retype that TestRecomposeFailureKeepsOldState rejects,
	// declared incompatible by the author via a state version. Recomposing then
	// resets the object to the new revision's defaults instead of failing.
	retyped := func(source string) string {
		return strings.ReplaceAll(strings.ReplaceAll(source,
			`let notes: Map[String!]! = ["seed": "constructor"]`, `let notes: Map[Int!]! = ["seed": 42]`),
			`notes.with("memo", "kept")`, `notes.with("memo", 99)`)
	}
	withVersion := func(source string) string {
		return strings.ReplaceAll(source, "base.withTools(currentNode)", "base.withTools(currentNode, version: 2)")
	}
	versioned := withVersion(retyped(fmt.Sprintf(recomposeSource, "")))
	extended := withVersion(retyped(fmt.Sprintf(recomposeSource, `
  added: String! { "added; counter: " + toString(state) }
`)))
	fixture := recomposeFixture(c, fmt.Sprintf(recomposeSource, ""))
	ws := fixture.AsWorkspace()
	script := recomposeRecordingTurn(core.NewQuery(c).LLM(), "mutate", "advance", "readState")
	script = recomposeRecordingTurn(script, "reset", "readState", "advance")
	script = recomposeRecordingTurn(script, "carried", "added", "readState")
	model := cannedRecordingModel(ctx, t, c, script)
	agent, llm := composeRecomposeFixture(ctx, t, c, ws, core.NewQuery(c).LLM(core.LLMOpts{Model: model}).WithWorkspace(ws))
	llm = llm.WithPrompt("mutate").Loop()
	transcript, err := llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "private counter: 1")
	require.Regexp(t, `"memo"\s*:\s*"kept"`, transcript)

	// Changing the withTools version from its default of 0 to 2 discards the
	// old state rather than overlaying it onto the retyped map.
	ws = ws.WithNewFile(recomposeModulePath, versioned)
	llm, err = recomposeLLM(ctx, t, agent, ws, llm)
	require.NoError(t, err)
	llm = llm.WithPrompt("reset").Loop()
	transcript, err = llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "private counter: 0")
	require.Regexp(t, `"seed"\s*:\s*42`, transcript)
	require.Len(t, regexp.MustCompile(`"memo"\s*:\s*"kept"`).FindAllString(transcript, -1), 1, "the old memo is gone after the reset")

	// An edit at the same version carries the (now version 2) state over.
	ws = ws.WithNewFile(recomposeModulePath, extended)
	llm, err = recomposeLLM(ctx, t, agent, ws, llm)
	require.NoError(t, err)
	transcript, err = llm.WithPrompt("carried").Loop().Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "added; counter: 1")
	require.Equal(t, 2, strings.Count(transcript, "private counter: 1"))
	require.Regexp(t, `"memo"\s*:\s*99`, transcript)
}

const recomposeGoModulePath = ".dagger/modules/swapper/main.go"

// The same shape as recomposeSource, in a container SDK. Nothing about state
// transfer is SDK-specific: private fields are whatever the SDK serializes.
const recomposeGoSource = `package main

import (
	"encoding/json"
	"fmt"

	"dagger/swapper/internal/dagger"
)

type Swapper struct {
	// An ordinary field, not engine metadata.
	StateVersion string
	// +private
	State int
	// +private
	Notes []string
%s
}

func New() *Swapper {
	return &Swapper{Notes: []string{"seed=constructor"}%s}
}

// +agent
func (s *Swapper) Agent(base *dagger.LLM) *dagger.LLM {
	return base.WithTools(dag.CurrentNode())
}

func (s *Swapper) Advance() *Swapper {
	s.State++
	if s.State == 1 {
		s.Notes = append(s.Notes, "memo=kept")
	}
	return s
}

// Read the private state.
func (s *Swapper) ReadState() string {
	notes, _ := json.Marshal(s.Notes)
	return fmt.Sprintf("private counter: %%d; notes: %%s", s.State, notes)
}
%s
`

func recomposeGoFixture(c *dagger.Client, source string) *core.Directory {
	return core.NewQuery(c).Directory().
		WithNewFile("dagger.toml", "[modules.swapper]\nsource = \".dagger/modules/swapper\"\n").
		WithNewFile(".dagger/modules/swapper/dagger.json", `{"name":"swapper","engineVersion":"v1.0.0-0","sdk":"go"}`).
		WithNewFile(recomposeGoModulePath, source)
}

func (LLMSuite) TestRecomposeGoModuleState(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	initial := fmt.Sprintf(recomposeGoSource, "", "", "")
	extended := fmt.Sprintf(recomposeGoSource, `
	// +private
	Label string`, `, Label: "new field default"`, `
func (s *Swapper) Added() string {
	return fmt.Sprintf("%s; updated counter: %d", s.Label, s.State)
}
`)
	versioned := strings.ReplaceAll(extended, "base.WithTools(dag.CurrentNode())", "base.WithTools(dag.CurrentNode(), dagger.LLMWithToolsOpts{Version: 2})")
	fixture := recomposeGoFixture(c, initial)
	ws := fixture.AsWorkspace()
	script := recomposeRecordingTurn(core.NewQuery(c).LLM(), "mutate", "advance", "readState")
	script = recomposeRecordingTurn(script, "edited", "added", "advance", "readState")
	script = recomposeRecordingTurn(script, "reset", "readState")
	model := cannedRecordingModel(ctx, t, c, script)
	agent, llm := composeRecomposeFixture(ctx, t, c, ws, core.NewQuery(c).LLM(core.LLMOpts{Model: model}).WithWorkspace(ws))
	llm = llm.WithPrompt("mutate").Loop()
	transcript, err := llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "private counter: 1")
	require.Contains(t, transcript, "memo=kept")

	// A new revision with an added private field and method: existing state
	// carries over, the new field takes the new constructor's default.
	ws = ws.WithNewFile(recomposeGoModulePath, extended)
	llm, err = recomposeLLM(ctx, t, agent, ws, llm)
	require.NoError(t, err)
	llm = llm.WithPrompt("edited").Loop()
	transcript, err = llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "new field default; updated counter: 1")
	require.Contains(t, transcript, "private counter: 2")
	require.Equal(t, 2, strings.Count(transcript, "memo=kept"))

	// Changing the withTools version resets the object to the new defaults.
	ws = ws.WithNewFile(recomposeGoModulePath, versioned)
	llm, err = refreshLLM(ctx, agent, ws, llm)
	require.NoError(t, err)
	tools, err := llm.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "## stateVersion\n", "a field named stateVersion is an ordinary tool")
	transcript, err = llm.WithPrompt("reset").Loop().Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "private counter: 0")
	require.Equal(t, 2, strings.Count(transcript, "memo=kept"))
}

func (LLMSuite) TestRecomposeSameBatchStateReturn(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	const editSource = `
  updateSource(ws: Workspace!): Workspace! {
    ws.withNewFile("` + recomposeModulePath + `", ws.file("next-source.txt").contents)
  }
`
	initial := fmt.Sprintf(recomposeSource, editSource)
	updated := fmt.Sprintf(recomposeSource, editSource+`
  added: String! { "updated source; counter: " + toString(state) }
`)
	ws := recomposeFixture(c, initial).WithNewFile("next-source.txt", updated).AsWorkspace()
	// Continuations run in their written position, on the state the calls
	// before them produced: reload must see both the source edit and the
	// same-type receiver returned by the calls emitted before it. Reload
	// recomposes as the agent running the loop, its me.
	script := core.NewQuery(c).LLM().WithPrompt("update and reload").WithResponse([]core.LLMContentBlockInput{
		recomposeTool("advance", "advance"), recomposeTool("edit", "updateSource"), recomposeTool("reload", "reload"),
	}).WithToolResult("advance", "", false).WithToolResult("edit", "", false).WithToolResult("reload", "", false).
		WithResponse([]core.LLMContentBlockInput{recomposeTool("added", "added")}).WithToolResult("added", "", false).
		WithResponse([]core.LLMContentBlockInput{recomposeTool("read", "readState")}).WithToolResult("read", "", false).
		WithResponse([]core.LLMContentBlockInput{{Kind: core.LLMContentBlockKindText, Text: "done"}})
	model := cannedRecordingModel(ctx, t, c, script)
	ids, err := discoverExpertise(ctx, c, ws)
	require.NoError(t, err)
	agent := spawnWithExpertise(ctx, t, c, core.NewQuery(c).LLM(core.LLMOpts{Model: model}).WithWorkspace(ws), "same-batch", ids)
	_, _, err = agent.sendAndWait(ctx, t, "update and reload")
	require.NoError(t, err)
	out, _ := agent.snapshot(ctx, t)
	require.Contains(t, out, "private counter: 1")
	require.Contains(t, out, "updated source; counter: 1")
	require.Regexp(t, `"memo"\s*:\s*"kept"`, out)
	require.NotContains(t, out, "already ran this turn")
}

func (LLMSuite) TestRecomposeLivePrivateRoster(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	workerScript := core.NewQuery(c).LLM().WithPrompt("opening").
		WithResponse([]core.LLMContentBlockInput{{Kind: core.LLMContentBlockKindText, Text: "worker opened"}}).
		WithPrompt("after reload").
		WithResponse([]core.LLMContentBlockInput{{Kind: core.LLMContentBlockKindText, Text: "same worker answered"}})
	workerModel := cannedRecordingModel(ctx, t, c, workerScript)
	roster := fmt.Sprintf(`
  let workers: Map[Agent!]! = [:]

  hire: Dagger.Swapper! @cache(policy: FunctionCachePolicy.Never) {
    let worker = llm(model: %q).spawn(name: "helper")
    worker.send("opening").response
    currentNode.{{... on Dagger.Swapper!}}.withWorker(worker)
  }

  withWorker(worker: Agent!): Swapper! {
    self.workers = workers.with("helper", worker)
    self
  }

  let member: Agent! {
    let worker = workers["helper"]
    if (worker == null) { raise "lost private roster" }
    worker
  }

  workerHandle: String! { "worker handle: " + member.handle }

  askWorker: String! @cache(policy: FunctionCachePolicy.Never) {
    member.send("after reload").response
  }

  harvest: String! @cache(policy: FunctionCachePolicy.Never) {
    "harvested: " + member.snapshot.lastReply
  }
`, workerModel)
	initial := fmt.Sprintf(recomposeSource, roster)
	updated := strings.ReplaceAll(initial, "Read the original private state.", "Read the updated private state.")
	fixture := recomposeFixture(c, initial)
	script := recomposeRecordingTurn(core.NewQuery(c).LLM(), "hire once", "hire", "workerHandle")
	script = recomposeRecordingTurn(script, "use roster", "workerHandle", "askWorker", "harvest")
	model := cannedRecordingModel(ctx, t, c, script)
	ws := fixture.AsWorkspace()
	agent, llm := composeRecomposeFixture(ctx, t, c, ws, core.NewQuery(c).LLM(core.LLMOpts{Model: model}).WithWorkspace(ws))
	llm = llm.WithPrompt("hire once").Loop()
	before, err := llm.Transcript(ctx)
	require.NoError(t, err)
	handlePattern := regexp.MustCompile(`worker handle: ([a-zA-Z0-9_-]+)`)
	beforeHandles := handlePattern.FindAllStringSubmatch(before, -1)
	require.Len(t, beforeHandles, 1)
	llm, err = refreshLLM(ctx, agent, ws.WithNewFile(recomposeModulePath, updated), llm)
	require.NoError(t, err)
	after, err := llm.WithPrompt("use roster").Loop().Transcript(ctx)
	require.NoError(t, err)
	afterHandles := handlePattern.FindAllStringSubmatch(after, -1)
	require.Len(t, afterHandles, 2)
	require.Equal(t, beforeHandles[0][1], afterHandles[1][1], "reload must not recreate the worker")
	require.Contains(t, after, "same worker answered")
	require.Contains(t, after, "harvested: same worker answered")
	require.NotContains(t, after, "lost private roster")
}

// Read stored system-message blocks, not Transcript: text shared with a user
// message or a tool result must not accidentally satisfy prompt assertions.
func recomposeSystemPrompts(ctx context.Context, t *testctx.T, c *dagger.Client, llm *core.LLM) []string {
	t.Helper()
	id, err := llm.ID(ctx)
	require.NoError(t, err)
	var res struct {
		Node struct {
			Messages []struct {
				Role    string
				Content []struct{ Text string }
			}
		}
	}
	require.NoError(t, c.Do(ctx, &dagger.Request{
		Query:     `query($id: ID!) { node(id: $id) { ... on LLM { messages { role content { text } } } } }`,
		Variables: map[string]any{"id": id},
	}, &dagger.Response{Data: &res}))
	var prompts []string
	for _, message := range res.Node.Messages {
		if message.Role == "SYSTEM" {
			for _, block := range message.Content {
				prompts = append(prompts, block.Text)
			}
		}
	}
	return prompts
}

func (LLMSuite) TestRecomposeOwnedPrompts(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	const shared = "Caller and module deliberately use identical prompt text."
	const replacement = "The edited swapper prompt."
	const callerTail = "Caller prompt appended after composition."
	source := strings.ReplaceAll(fmt.Sprintf(recomposeSource, ""),
		"base.withTools(currentNode)", fmt.Sprintf("base.withSystemPrompt(%q).withTools(currentNode)", shared))
	fixture := recomposeFixture(c, source).
		WithNewFile("dagger.toml", "[modules.swapper]\nsource = \".dagger/modules/swapper\"\n[modules.other]\nsource = \"modules/other\"\n").
		WithNewFile("modules/other/dagger.json", `{"name":"other","engineVersion":"v1.0.0-0","sdk":"dang"}`)
	const otherSource = `
type Other {
  agent(base: LLM!): LLM! @agent {
    base.withSystemPrompt("The unrelated expertise prompt.").withTools(currentNode)
  }
  oldOtherTool: String! { "other original tool" }
}
`
	fixture = fixture.WithNewFile("modules/other/main.dang", otherSource)
	ws := fixture.AsWorkspace()
	agent, llm := composeRecomposeFixture(ctx, t, c, ws, core.NewQuery(c).LLM().WithWorkspace(ws).WithSystemPrompt(shared))
	llm = llm.WithSystemPrompt(callerTail)
	require.ElementsMatch(t, []string{shared, shared, callerTail, "The unrelated expertise prompt."},
		recomposeSystemPrompts(ctx, t, c, llm))

	// Record against the actual composed prompts so the replay model checks
	// exactly the history it will receive before the source edit.
	model := cannedRecordingModel(ctx, t, c, recomposeRecordingTurn(llm, "mutate owned state", "advance", "readState"))
	llm = llm.WithModel(model).WithPrompt("mutate owned state").Loop()
	transcript, err := llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "private counter: 1")

	// Both modules change on disk. Agent.recompose refreshes every recorded
	// entry, replacing only each owner's old prompt and tools: identical
	// caller text is not ownership.
	updated := strings.ReplaceAll(source, shared, replacement)
	updated = strings.ReplaceAll(updated, "Read the original private state.", "Read the owner's updated state.")
	changedOther := strings.ReplaceAll(strings.ReplaceAll(otherSource,
		"The unrelated expertise prompt.", "The edited unrelated prompt."), "oldOtherTool", "newOtherTool")
	ws = ws.WithNewFile(recomposeModulePath, updated).
		WithNewFile("modules/other/main.dang", changedOther)
	llm, err = refreshLLM(ctx, agent, ws, llm)
	require.NoError(t, err)
	expected := []string{shared, replacement, callerTail, "The edited unrelated prompt."}
	require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, c, llm))
	tools, err := llm.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "Read the owner's updated state.")
	require.Contains(t, tools, "## newOtherTool\n")
	require.NotContains(t, tools, "## oldOtherTool\n")

	// Ownership must survive a new same-type state result too, not just the
	// first composition. This recording now uses the refreshed system blocks.
	model = cannedRecordingModel(ctx, t, c, recomposeRecordingTurn(llm, "mutate reloaded state", "advance", "readState"))
	llm = llm.WithModel(model).WithPrompt("mutate reloaded state").Loop()
	transcript, err = llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "private counter: 2")
	portable, err := sink.captureLLMRecipe(ctx, t, c, llm)
	require.NoError(t, err)
	target := connect(ctx, t)
	targetAgent, llm := restoreComposer(ctx, t, target, agent, portable)
	require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, target, llm))
	for range 2 {
		llm, err = recomposeLLM(ctx, t, targetAgent, llm.Workspace(), llm, "swapper")
		require.NoError(t, err)
		require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, target, llm))
		tools, err = llm.Tools(ctx)
		require.NoError(t, err)
		require.Contains(t, tools, "## newOtherTool\n")
		require.NotContains(t, tools, "## oldOtherTool\n")
	}
}

func (LLMSuite) TestRecomposeNestedCompositionPrompts(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	fixture := core.NewQuery(c).Directory().
		WithNewFile("dagger.toml", "[modules.outer]\nsource = \"outer\"\n[modules.inner]\nsource = \"inner\"\n").
		WithNewFile("outer/dagger.json", `{"name":"outer","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile("outer/main.dang", `
type Outer {
  agent(base: LLM!, ws: Workspace!, me: Agent!): LLM! @agent {
    me.compose(conversation: base.withSystemPrompt("outer prompt"), expertise: ws.artifacts(include: ["inner"]).filterTypes(types: ["Expertise"]).asExpertise.{{ id }}.map { node(id: _.id).{{ ... on Expertise! }} })
  }
}
`).
		WithNewFile("inner/dagger.json", `{"name":"inner","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile("inner/main.dang", `
type Inner {
  agent(base: LLM!): LLM! @agent { base.withSystemPrompt("inner prompt") }
}
`)
	ws := fixture.AsWorkspace()
	agent, llm := composeRecomposeFixture(ctx, t, c, ws, nil, "outer")
	entries := recordedExpertise(ctx, t, c, llm)
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].Artifact.URI, "outer/agent")
	expected := []string{"outer prompt", "inner prompt"}
	require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, c, llm))
	for range 2 {
		var err error
		llm, err = recomposeLLM(ctx, t, agent, ws, llm, "outer")
		require.NoError(t, err)
		// Nested contributions are replaced with their outer entry, not appended.
		require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, c, llm),
			"nested compose contributes to the outer entry")
	}
	llm, err := composeLLM(ctx, agent, ws, llm, "inner")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"outer prompt", "inner prompt", "inner prompt"}, recomposeSystemPrompts(ctx, t, c, llm),
		"the separately recorded Inner entry does not replace Outer's nested contribution")
	require.Len(t, recordedExpertise(ctx, t, c, llm), 2)
}

func (LLMSuite) TestRecomposePreservesManualContributions(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	const prompt = "An unowned prompt that also happens to be the module prompt."
	source := strings.ReplaceAll(fmt.Sprintf(recomposeSource, ""),
		"base.withTools(currentNode)", fmt.Sprintf("base.withSystemPrompt(%q).withTools(currentNode)", prompt))
	fixture := recomposeFixture(c, source)
	ws := fixture.AsWorkspace()
	file := core.NewQuery(c).Directory().WithNewFile("marker.txt", "manual file contents survived reload").File("marker.txt")
	manual := core.NewQuery(c).LLM().WithWorkspace(ws).WithSystemPrompt(prompt).WithTools(file)
	// Ordinary withTools does not retroactively claim a caller's prompts.
	require.Equal(t, []string{prompt}, recomposeSystemPrompts(ctx, t, c, manual))
	tools, err := manual.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "## contents\n")

	// Each entry is recorded once; composing an identical entry again
	// refreshes it in place rather than duplicating its contributions.
	agent, composed := composeRecomposeFixture(ctx, t, c, ws, manual)
	require.ElementsMatch(t, []string{prompt, prompt}, recomposeSystemPrompts(ctx, t, c, composed))
	again, err := composeLLM(ctx, agent, ws, composed)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{prompt, prompt}, recomposeSystemPrompts(ctx, t, c, again))
	require.Equal(t, recordedExpertise(ctx, t, c, composed), recordedExpertise(ctx, t, c, again))

	const replacement = "Replacement owned prompt, never the caller's."
	ws = ws.WithNewFile(recomposeModulePath, strings.ReplaceAll(source, prompt, replacement))
	for _, seed := range []*core.LLM{manual, composed} {
		// The legacy/manual seed has no ownership metadata to infer. Preserve
		// its prompt and capability while installing the new owned expertise.
		// The agent adopts the unowned conversation.
		llm, err := recomposeLLM(ctx, t, agent, ws, seed)
		require.NoError(t, err)
		for range 2 {
			require.ElementsMatch(t, []string{prompt, replacement}, recomposeSystemPrompts(ctx, t, c, llm))
			tools, err := llm.Tools(ctx)
			require.NoError(t, err)
			require.Contains(t, tools, "## contents\n")
			require.Contains(t, tools, "## readState\n")
			llm, err = recomposeLLM(ctx, t, agent, ws, llm)
			require.NoError(t, err)
		}
		model := cannedRecordingModel(ctx, t, c, recomposeRecordingTurn(llm, "read manual capability", "contents"))
		transcript, err := llm.WithModel(model).WithPrompt("read manual capability").Loop().Transcript(ctx)
		require.NoError(t, err)
		require.Contains(t, transcript, "manual file contents survived reload")
	}
}

func (LLMSuite) TestRecomposeNestedToolState(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	const outerSource = `
type Outer {
  agent(base: LLM!, ws: Workspace!, me: Agent!): LLM! @agent {
    me.compose(conversation: base, expertise: ws.artifacts(include: ["swapper"]).filterTypes(types: ["Expertise"]).asExpertise.{{ id }}.map { node(id: _.id).{{ ... on Expertise! }} })
  }
}
`
	// Keep this module's implementation distinct from the concurrently running
	// root-only Swapper fixtures: implementation cache aliases otherwise retain
	// their different workspace provenance, before nested ownership is tested.
	const fixtureState = `let nestedFixture: String! = "nested tool state"`
	initial := fmt.Sprintf(recomposeSource, fixtureState)
	fixture := recomposeFixture(c, initial).
		WithNewFile("dagger.toml", "[modules.swapper]\nsource = \".dagger/modules/swapper\"\n[modules.outer]\nsource = \"outer\"\n").
		WithNewFile("outer/dagger.json", `{"name":"outer","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile("outer/main.dang", outerSource)
	ws := fixture.AsWorkspace()
	script := recomposeRecordingTurn(core.NewQuery(c).LLM(), "mutate nested", "advance", "readState")
	script = recomposeRecordingTurn(script, "updated nested", "added")
	script = recomposeRecordingTurn(script, "reset nested", "added")
	model := cannedRecordingModel(ctx, t, c, script)
	agent, llm := composeRecomposeFixture(ctx, t, c, ws, core.NewQuery(c).LLM(core.LLMOpts{Model: model}).WithWorkspace(ws), "outer")
	llm = llm.WithPrompt("mutate nested").Loop()
	transcript, err := llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "private counter: 1")

	updated := fmt.Sprintf(recomposeSource, fixtureState+`
  let addedDefault: String! = "nested default"
  added: String! { addedDefault + "; counter: " + toString(state) }
`)
	// Refreshing Outer refreshes all contributions made during its run,
	// including the nested Swapper binding. No explicit nested recompose is needed.
	ws = ws.WithNewFile(recomposeModulePath, updated)
	llm, err = recomposeLLM(ctx, t, agent, ws, llm, "outer")
	require.NoError(t, err)
	llm = llm.WithPrompt("updated nested").Loop()
	transcript, err = llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "nested default; counter: 1", "outer recompose must retain the nested Swapper tool state")

	// A nested binding still owns its explicit version contract after replay.
	portable, err := sink.captureLLMRecipe(ctx, t, c, llm)
	require.NoError(t, err)
	c = connect(ctx, t)
	agent, llm = restoreComposer(ctx, t, c, agent, portable)
	ws = llm.Workspace().WithNewFile(recomposeModulePath,
		strings.ReplaceAll(updated, "base.withTools(currentNode)", "base.withTools(currentNode, version: 2)"))
	llm, err = recomposeLLM(ctx, t, agent, ws, llm, "outer")
	require.NoError(t, err)
	llm = llm.WithPrompt("reset nested").Loop()
	transcript, err = llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "nested default; counter: 0")

	// The nested binding belongs to Outer. Dropping it must honor the same
	// state-loss guard as dropping a binding installed directly by Outer.
	withoutNested := ws.WithNewFile("outer/main.dang", strings.ReplaceAll(outerSource,
		`me.compose(conversation: base, expertise: ws.artifacts(include: ["swapper"]).filterTypes(types: ["Expertise"]).asExpertise.{{ id }}.map { node(id: _.id).{{ ... on Expertise! }} })`, "base"))
	_, err = refreshLLM(ctx, agent, withoutNested, llm)
	require.ErrorContains(t, err, "discard tool state")
	tools, err := llm.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "## added\n", "failed refresh leaves the old binding usable")
}

func (LLMSuite) TestRecomposeNestedOwnershipIsolation(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	const outerSource = `
type Outer {
  agent(base: LLM!, ws: Workspace!, me: Agent!): LLM! @agent {
    me.compose(conversation: base.withSystemPrompt("outer original"), expertise: ws.artifacts(include: ["inner"]).filterTypes(types: ["Expertise"]).asExpertise.{{ id }}.map { node(id: _.id).{{ ... on Expertise! }} })
  }
}
`
	const innerSource = `
type Inner {
  agent(base: LLM!): LLM! @agent { base.withSystemPrompt("inner original") }
}
`
	fixture := core.NewQuery(c).Directory().
		WithNewFile("dagger.toml", "[modules.outer]\nsource = \"outer\"\n[modules.inner]\nsource = \"inner\"\n").
		WithNewFile("outer/dagger.json", `{"name":"outer","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile("outer/main.dang", outerSource).
		WithNewFile("inner/dagger.json", `{"name":"inner","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile("inner/main.dang", innerSource)
	ws := fixture.AsWorkspace()
	// Inner is composed independently and through Outer. The nested run is
	// owned by Outer, even though the prompt text and module are identical.
	agent, llm := composeRecomposeFixture(ctx, t, c, ws, core.NewQuery(c).LLM().WithWorkspace(ws).WithSystemPrompt("inner original"), "inner")
	llm, err := composeLLM(ctx, agent, ws, llm, "outer")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"inner original", "inner original", "inner original", "outer original",
	}, recomposeSystemPrompts(ctx, t, c, llm))

	// Composing Outer again refreshes only that entry, in place: its
	// transitive contributions, including its nested Inner run, are replaced
	// while the independent Inner entry keeps its original code.
	ws = ws.WithNewFile("inner/main.dang", strings.ReplaceAll(innerSource, "inner original", "inner updated")).
		WithNewFile("outer/main.dang", strings.ReplaceAll(outerSource, "outer original", "outer updated"))
	llm, err = composeLLM(ctx, agent, ws, llm, "outer")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"inner original", "inner original", "outer updated", "inner updated"}, recomposeSystemPrompts(ctx, t, c, llm),
		"Outer replaces its transitive contributions without changing the independent Inner entry")

	// Portable recipes retain entry ownership and composition ancestry.
	// Recomposing refreshes the independent Inner entry too.
	expected := []string{"inner original", "inner updated", "outer updated", "inner updated"}
	for range 2 {
		portable, err := sink.captureLLMRecipe(ctx, t, c, llm)
		require.NoError(t, err)
		c, sink = connectWithTrace(ctx, t)
		agent, llm = restoreComposer(ctx, t, c, agent, portable)
		ws = llm.Workspace()
		llm, err = refreshLLM(ctx, agent, ws, llm)
		require.NoError(t, err)
		require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, c, llm))
	}

	// Composing Inner again refreshes only its independent entry, in place,
	// not Outer's nested contribution or the identical unowned prompt.
	llm, err = composeLLM(ctx, agent, ws, llm, "inner")
	require.NoError(t, err)
	require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, c, llm))

	// Removing the nested compose removes only Outer's nested contribution.
	ws = ws.WithNewFile("outer/main.dang", `
type Outer {
  agent(base: LLM!): LLM! @agent { base.withSystemPrompt("outer alone") }
}
`)
	for range 2 {
		llm, err = composeLLM(ctx, agent, ws, llm, "outer")
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"inner original", "inner updated", "outer alone"},
			recomposeSystemPrompts(ctx, t, c, llm))
	}
}

func (LLMSuite) TestRecomposeDirectCallerSkills(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	const shared = "Identical caller prompt."
	const original = `
  contribute(base: LLM!, skills: Directory!): LLM! {
    base.withSystemPrompt("Identical caller prompt.").withSkills(skills)
  }
`
	fixture := core.NewQuery(c).Directory().WithNewFile("dagger.toml", "[modules.owner]\nsource = \"owner\"\n[modules.owner-extra]\nsource = \"owner-extra\"\n")
	for _, module := range []struct{ name, typ string }{{"owner", "Owner"}, {"owner-extra", "OwnerExtra"}} {
		fixture = fixture.
			WithNewFile(module.name+"/dagger.json", fmt.Sprintf(`{"name":%q,"engineVersion":"v1.0.0-0","sdk":"dang"}`, module.name)).
			WithNewFile(module.name+"/main.dang", "type "+module.typ+" {"+original+"}")
	}
	ws := fixture.AsWorkspace()
	for _, name := range []string{"owner", "owner-extra"} {
		require.NoError(t, ws.ModuleSource(name).AsModule().Serve(ctx))
	}
	manual := core.NewQuery(c).Directory().WithNewFile("manual/SKILL.md", "---\ndescription: Main client skill.\n---\nmanual")
	oldSkills := core.NewQuery(c).Directory().WithNewFile("old-owned/SKILL.md", "---\ndescription: Old module skill.\n---\nold")
	base := core.NewQuery(c).LLM().WithWorkspace(ws).WithSystemPrompt(shared).WithSkills(manual)
	baseID, err := base.ID(ctx)
	require.NoError(t, err)
	skillsID, err := oldSkills.ID(ctx)
	require.NoError(t, err)
	seeds := map[string]*core.LLM{}
	for _, caller := range []string{"owner", "ownerExtra"} {
		var res struct {
			Caller struct{ Contribute struct{ ID string } }
		}
		// No @agent entrypoint is involved. Both callers make identical core
		// calls on the same base with the same explicit arguments.
		require.NoError(t, c.Do(ctx, &dagger.Request{
			Query: fmt.Sprintf(`query($base: ID!, $skills: ID!) {
				caller: %s { contribute(base: $base, skills: $skills) { id } }
			}`, caller),
			Variables: map[string]any{"base": baseID, "skills": skillsID},
		}, &dagger.Response{Data: &res}))
		seeds[caller] = core.Ref[*core.LLM](core.NewQuery(c), core.ID(res.Caller.Contribute.ID))
		require.ElementsMatch(t, []string{shared, shared}, recomposeSystemPrompts(ctx, t, c, seeds[caller]))
		require.Contains(t, skillIndex(ctx, t, seeds[caller]), "old-owned")
	}
	seeds["main"] = base.WithSystemPrompt(shared).WithSkills(oldSkills)

	// Introduce two entries in Owner. Each owns only its own contributions;
	// the earlier direct calls remain unowned, regardless of calling module.
	const updated = `
type Owner {
  first(base: LLM!): LLM! @agent {
    base.withSystemPrompt("first replacement").withSkills(directory.withNewFile("current/SKILL.md", "---\ndescription: Current module skill.\n---\ncurrent"))
  }
  second(base: LLM!): LLM! @agent {
    base.withSystemPrompt("second replacement").withSkills(directory.withNewFile("second/SKILL.md", "---\ndescription: Second entrypoint skill.\n---\nsecond")).withTools(currentNode)
  }
  ping: String! { "second entrypoint tool" }
}
`
	ws = ws.WithNewFile("owner/main.dang", updated)
	for _, caller := range []string{"owner", "ownerExtra", "main"} {
		// An agent adopts the unowned seed; composition runs as that agent.
		agent, llm, err := spawnComposed(ctx, t, c, seeds[caller], nil)
		require.NoError(t, err)
		client := c
		captureSink := sink
		refreshedWS := ws
		for round := range 2 {
			llm, err = recomposeLLM(ctx, t, agent, refreshedWS, llm, "owner")
			require.NoError(t, err)
			// On the second round, the first entrypoint has not yet restored
			// Owner's binding. Preservation must wait for the second one.
			tools, err := llm.Tools(ctx)
			require.NoError(t, err)
			require.Contains(t, tools, "## ping\n")
			expected := []string{shared, shared, "first replacement", "second replacement"}
			require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, client, llm), "caller %s, round %d", caller, round)
			skills := skillIndex(ctx, t, llm)
			require.Equal(t, "Main client skill.", skills["manual"])
			if round == 0 {
				require.Equal(t, "Current module skill.", skills["current"])
			} else {
				require.NotContains(t, skills, "current", "removed skills must not accumulate across edits")
				require.Equal(t, "Current module skill.", skills["next"])
			}
			require.Equal(t, "Second entrypoint skill.", skills["second"])
			require.Contains(t, skills, "old-owned", "calls outside entry runs remain unowned")
			portable, err := captureSink.captureLLMRecipe(ctx, t, client, llm)
			require.NoError(t, err)
			client, captureSink = connectWithTrace(ctx, t)
			agent, llm = restoreComposer(ctx, t, client, agent, portable)
			require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, client, llm))
			require.Equal(t, skills, skillIndex(ctx, t, llm))
			refreshedWS = llm.Workspace().WithNewFile("owner/main.dang", strings.ReplaceAll(updated, "current/SKILL.md", "next/SKILL.md"))
		}
	}
}

func (LLMSuite) TestRecomposeTransitiveHelperOwnership(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	const ownerSource = `
type Owner {
  first(base: LLM!): LLM! @agent { donor.contribute(base) }
  second(base: LLM!): LLM! @agent { donor.contribute(base) }
  manual(base: LLM!): LLM! { donor.contribute(base) }
}
`
	const donorSource = `
type Donor {
  contribute(base: LLM!): LLM! {
    base.withSystemPrompt("shared helper original")
  }
}
`
	ws := core.NewQuery(c).Directory().
		WithNewFile("dagger.toml", "[modules.owner]\nsource = \"owner\"\n").
		WithNewFile("owner/dagger.json", `{"name":"owner","engineVersion":"v1.0.0-0","sdk":"dang","dependencies":[{"name":"donor","source":"donor"}]}`).
		WithNewFile("owner/main.dang", ownerSource).
		WithNewFile("owner/donor/dagger.json", `{"name":"donor","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile("owner/donor/main.dang", donorSource).AsWorkspace()
	require.NoError(t, ws.ModuleSource("owner").AsModule().Serve(ctx))
	base := core.NewQuery(c).LLM().WithWorkspace(ws)
	baseID, err := base.ID(ctx)
	require.NoError(t, err)
	var res struct {
		Owner struct{ Manual struct{ ID core.ID } }
	}
	require.NoError(t, c.Do(ctx, &dagger.Request{
		Query:     `query($base: ID!) { owner { manual(base: $base) { id } } }`,
		Variables: map[string]any{"base": baseID},
	}, &dagger.Response{Data: &res}))
	manualAgent, manual, err := spawnComposed(ctx, t, c, core.Ref[*core.LLM](core.NewQuery(c), res.Owner.Manual.ID), nil)
	require.NoError(t, err)
	agents := map[string]*agentHandle{"manual": manualAgent}
	seeds := map[string]*core.LLM{"manual": manual}
	for _, name := range []string{"first", "second"} {
		// Both entries call precisely the same helper with the same base ID.
		agents[name], seeds[name] = spawnURI(ctx, t, c, ws, base, "owner/"+name)
	}
	agents["both"], seeds["both"] = composeRecomposeFixture(ctx, t, c, ws, nil, "owner")
	updated := ws.WithNewFile("owner/donor/main.dang", strings.ReplaceAll(donorSource, "shared helper original", "shared helper refreshed"))
	for _, name := range []string{"manual", "first", "second", "both"} {
		// Composing first refreshes only that entry, in place, or appends it.
		llm, err := composeURI(ctx, agents[name], updated, seeds[name], "owner/first")
		require.NoError(t, err)
		expected := []string{"shared helper refreshed"}
		if name != "first" {
			expected = append(expected, "shared helper original")
		}
		require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, c, llm), "seed %s", name)
		portable, err := sink.captureLLMRecipe(ctx, t, c, llm)
		require.NoError(t, err)
		target := connect(ctx, t)
		targetAgent, restored := restoreComposer(ctx, t, target, agents[name], portable)
		restored, err = composeURI(ctx, targetAgent, restored.Workspace(), restored, "owner/first")
		require.NoError(t, err)
		require.ElementsMatch(t, expected, recomposeSystemPrompts(ctx, t, target, restored), "replayed seed %s", name)
	}
}

func (LLMSuite) TestRecomposeWorkerIdentity(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	fixture, err := installAgents(t, c, "staff")
	require.NoError(t, err)
	workerScript := recomposeRecordingTurn(core.NewQuery(c).LLM(), "inspect worker", "parentName")
	// Recompose removes the original leading prompt and appends its replacement
	// during the reload tool call. Record that mid-conversation system message.
	reloadCall := recomposeTool("refresh_reload", "reload")
	parentCall := recomposeTool("refresh_parent", "parentName")
	workerScript = workerScript.WithPrompt("refresh worker").
		WithResponse([]core.LLMContentBlockInput{reloadCall}).
		WithSystemPrompt("Updated worker role: helper").
		WithToolResult(reloadCall.CallID, "", false).
		WithResponse([]core.LLMContentBlockInput{parentCall}).
		WithToolResult(parentCall.CallID, "", false).
		WithResponse([]core.LLMContentBlockInput{{Kind: core.LLMContentBlockKindText, Text: "refresh worker done"}})
	workerScript = recomposeRecordingTurn(workerScript, "restored worker", "parentName")
	workerModel := cannedRecordingModel(ctx, t, c, workerScript)
	source, err := fixture.File("../staff/main.dang").Contents(ctx)
	require.NoError(t, err)
	source = strings.ReplaceAll(source, "__WORKER_MODEL__", workerModel)
	ws := agentFixtureWorkspace(fixture.WithNewFile("../staff/main.dang", source))

	// Discovery remains directive-based even though explicit asExpertise is
	// signature-based. The plain worker transform must not become a chief role.
	_, defaultLLM := composeRecomposeFixture(ctx, t, c, ws, nil)
	defaults := recordedExpertise(ctx, t, c, defaultLLM)
	require.Len(t, defaults, 1)
	require.Contains(t, defaults[0].Artifact.URI, "staff/agent")
	chiefModel := cannedRecordingModel(ctx, t, c, recomposeRecordingTurn(core.NewQuery(c).LLM(), "hire child", "hire"))
	chiefEntries, err := uriExpertiseIDs(ctx, c, ws, "staff/agent")
	require.NoError(t, err)
	chief := spawnWithExpertise(ctx, t, c, core.NewQuery(c).LLM(core.LLMOpts{Model: chiefModel}).WithWorkspace(ws), "chief", chiefEntries)
	root := chief.mustRun(ctx, t, "name handle parent { handle }")
	require.Equal(t, "chief", root.Get("name").String())
	require.Nil(t, root.Get("parent").Value(), "root agents have no parent")
	_, _, err = chief.sendAndWait(ctx, t, "hire child")
	require.NoError(t, err)
	transcript := chief.mustRun(ctx, t, "snapshot { transcript }").Get("snapshot.transcript").String()
	matches := regexp.MustCompile(`worker handle: ([a-zA-Z0-9_-]+)`).FindStringSubmatch(transcript)
	require.Len(t, matches, 2, transcript)
	worker := unmintedAgent(ctx, t, c, matches[1], "helper")
	lineage := worker.mustRun(ctx, t, "name parent { name handle }")
	require.Equal(t, "helper", lineage.Get("name").String())
	require.Equal(t, "chief", lineage.Get("parent.name").String())
	require.Equal(t, root.Get("handle").String(), lineage.Get("parent.handle").String())
	llm := core.Ref[*core.LLM](core.NewQuery(c), core.ID(worker.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
	entries := recordedExpertise(ctx, t, c, llm)
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].Artifact.URI, "staff/worker")
	require.Equal(t, []string{"Worker role: helper"}, recomposeSystemPrompts(ctx, t, c, llm))
	_, _, err = worker.sendAndWait(ctx, t, "inspect worker")
	require.NoError(t, err)
	require.Contains(t, worker.mustRun(ctx, t, "snapshot { transcript }").Get("snapshot.transcript").String(), "original parent: chief")

	updated := strings.NewReplacer("Worker role: ", "Updated worker role: ", "Original peer implementation.", "Updated peer implementation.", "original parent: ", "updated parent: ").Replace(source)
	llm = core.Ref[*core.LLM](core.NewQuery(c), core.ID(worker.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
	llmID, err := llm.WithWorkspace(ws.WithNewFile("../staff/main.dang", updated)).ID(ctx)
	require.NoError(t, err)
	require.NoError(t, worker.reseedAgent(ctx, t, string(llmID)))
	// Reload is me.recompose(conversation: llm), executed inside the worker:
	// it refreshes only the recorded worker entry, so the worker gains no
	// chief tools. Its me is the worker, never the chief that originally
	// invoked spawn.
	_, _, err = worker.sendAndWait(ctx, t, "refresh worker")
	require.NoError(t, err)
	llm = core.Ref[*core.LLM](core.NewQuery(c), core.ID(worker.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
	require.Equal(t, entries, recordedExpertise(ctx, t, c, llm))
	require.Equal(t, []string{"Updated worker role: helper"}, recomposeSystemPrompts(ctx, t, c, llm))
	tools, err := llm.Tools(ctx)
	require.NoError(t, err)
	require.NotContains(t, tools, "chiefOnly")
	require.Contains(t, tools, "Updated peer implementation.")
	transcript, err = llm.Transcript(ctx)
	require.NoError(t, err)
	require.Contains(t, transcript, "updated parent: chief")

	// CLI restore evaluates a captured conversation before minting its runtime.
	// Seed-free owner references must remain lazy through that evaluation.
	chiefSnapshot := core.Ref[*core.LLM](core.NewQuery(c), core.ID(chief.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
	chiefRecipe, err := sink.captureLLMRecipe(ctx, t, c, chiefSnapshot)
	require.NoError(t, err)
	workerRecipe, err := sink.captureLLMRecipe(ctx, t, c, llm)
	require.NoError(t, err)
	t.Run("restore worker before recomposition", func(ctx context.Context, t *testctx.T) {
		target := connect(ctx, t)
		restoredChiefSeed, err := core.Ref[*core.LLM](core.NewQuery(target), chiefRecipe).ID(ctx)
		require.NoError(t, err)
		_, err = rehydrateAgent(ctx, target, string(restoredChiefSeed), root.Get("handle").String(), "chief", "IDLE", "")
		require.NoError(t, err)
		restoredWorkerSeed, err := core.Ref[*core.LLM](core.NewQuery(target), workerRecipe).ID(ctx)
		require.NoError(t, err, "load the snapshot before spawning its owning worker")
		restoredWorker, err := rehydrateAgentWithParent(ctx, target, string(restoredWorkerSeed), matches[1], "helper", "IDLE", "", root.Get("handle").String())
		require.NoError(t, err)
		_, _, err = restoredWorker.sendAndWait(ctx, t, "restored worker")
		require.NoError(t, err)
		restoredTranscript := restoredWorker.mustRun(ctx, t, "snapshot { transcript }").Get("snapshot.transcript").String()
		require.Equal(t, 2, strings.Count(restoredTranscript, "updated parent: chief"), "parentName must run again after restore")
		restored := core.Ref[*core.LLM](core.NewQuery(target), core.ID(restoredWorker.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
		require.Equal(t, entries, recordedExpertise(ctx, t, target, restored))
		restored = recomposeRecordedExpertise(ctx, t, restoredWorker, restored.Workspace(), restored)
		require.Equal(t, []string{"Updated worker role: helper"}, recomposeSystemPrompts(ctx, t, target, restored))
		restoredTools, err := restored.Tools(ctx)
		require.NoError(t, err)
		require.NotContains(t, restoredTools, "chiefOnly")
	})

	// Composing the chief entry as the worker appends it: the worker becomes
	// a chief.
	llm, err = composeURI(ctx, worker, llm.Workspace(), llm, "staff/agent")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"Updated worker role: helper", "Chief role."}, recomposeSystemPrompts(ctx, t, c, llm))
	tools, err = llm.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "## chiefOnly\n")
	require.Len(t, recordedExpertise(ctx, t, c, llm), 2)
	llm = recomposeRecordedExpertise(ctx, t, worker, llm.Workspace(), llm)
	require.ElementsMatch(t, []string{"Updated worker role: helper", "Chief role."}, recomposeSystemPrompts(ctx, t, c, llm))

	// A conversation belongs to its agent: the chief cannot compose the
	// worker's conversation, whose entries' me is the worker.
	_, err = recomposeAs(ctx, chief, llm)
	require.ErrorContains(t, err, "belongs to agent")
}

func spawnWithExpertise(ctx context.Context, t *testctx.T, c *dagger.Client, llm *core.LLM, name string, entries []core.ID) *agentHandle {
	t.Helper()
	id, err := llm.ID(ctx)
	require.NoError(t, err)
	var result struct{ Node struct{ Spawn string } }
	require.NoError(t, c.Do(ctx, &dagger.Request{
		Query:     `query($id: ID!, $name: String!, $entries: [ID!]!) { node(id: $id) { ... on LLM { spawn(name: $name, expertise: $entries) } } }`,
		Variables: map[string]any{"id": id, "name": name, "entries": entries},
	}, &dagger.Response{Data: &result}))
	return &agentHandle{c: c, agentID: result.Node.Spawn}
}

func (LLMSuite) TestComposeTransformURIIdentity(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	const source = `
type Swapper {
  first(base: LLM!): LLM! { base.withSystemPrompt("first original") }
  second(base: LLM!): LLM! { base.withSystemPrompt("second original") }
  invalid: Directory! { directory }
}
`
	ws := recomposeFixture(c, source).AsWorkspace()
	// Neither transform carries @agent, so default discovery finds nothing,
	// and recomposing an empty record is an error.
	agent, llm := composeRecomposeFixture(ctx, t, c, ws, nil)
	require.Empty(t, recordedExpertise(ctx, t, c, llm))
	_, err := refreshLLM(ctx, agent, ws, llm)
	require.ErrorContains(t, err, "no recorded expertise")
	// Explicit conversion uses signatures.
	llm, err = composeURI(ctx, agent, ws, llm, "swapper/first")
	require.NoError(t, err)
	entries := recordedExpertise(ctx, t, c, llm)
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].Artifact.URI, "swapper/first")
	// Composing a recorded entry again, by its full URI or twice in one
	// batch, refreshes it in place instead of failing as a duplicate.
	again, err := composeURI(ctx, agent, ws, llm, entries[0].Artifact.URI)
	require.NoError(t, err, "an absolute URI identifies the same entry")
	require.Equal(t, entries, recordedExpertise(ctx, t, c, again))
	require.Equal(t, []string{"first original"}, recomposeSystemPrompts(ctx, t, c, again))
	ids, err := uriExpertiseIDs(ctx, c, ws, "swapper/first")
	require.NoError(t, err)
	require.Len(t, ids, 1)
	twice, err := composeAs(ctx, agent, core.NewQuery(c).LLM().WithWorkspace(ws), []core.ID{ids[0], ids[0]})
	require.NoError(t, err)
	require.Equal(t, entries, recordedExpertise(ctx, t, c, twice))
	require.Equal(t, []string{"first original"}, recomposeSystemPrompts(ctx, t, c, twice))
	_, err = uriExpertiseIDs(ctx, c, ws, "swapper/invalid")
	require.Error(t, err, "asExpertise must reject non-transform artifacts")
	// A new entry is appended after the recorded ones.
	llm, err = composeURI(ctx, agent, ws, llm, "swapper/second")
	require.NoError(t, err)
	entries = recordedExpertise(ctx, t, c, llm)
	require.Len(t, entries, 2)
	require.Contains(t, entries[0].Artifact.URI, "swapper/first")
	require.Contains(t, entries[1].Artifact.URI, "swapper/second")

	// Composing refreshes only the given entry, in place, and runs it as
	// given: here, as converted from the original workspace. Other recorded
	// entries are untouched until a recompose.
	ws = ws.WithNewFile(recomposeModulePath, strings.ReplaceAll(source, "original", "updated"))
	llm, err = composeAs(ctx, agent, llm.WithWorkspace(ws), ids)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"first original", "second original"}, recomposeSystemPrompts(ctx, t, c, llm))
	require.Equal(t, entries, recordedExpertise(ctx, t, c, llm))
	llm, err = composeURI(ctx, agent, ws, llm, "swapper/first")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"first updated", "second original"}, recomposeSystemPrompts(ctx, t, c, llm))
	require.Equal(t, entries, recordedExpertise(ctx, t, c, llm))
	llm, err = refreshLLM(ctx, agent, ws, llm)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"first updated", "second updated"}, recomposeSystemPrompts(ctx, t, c, llm))
	require.Equal(t, entries, recordedExpertise(ctx, t, c, llm))
	t.Run("portable replay", func(ctx context.Context, t *testctx.T) {
		portable, err := sink.captureLLMRecipe(ctx, t, c, llm)
		require.NoError(t, err)
		target := connect(ctx, t)
		targetAgent, restored := restoreComposer(ctx, t, target, agent, portable)
		require.Equal(t, entries, recordedExpertise(ctx, t, target, restored))
		restored = recomposeRecordedExpertise(ctx, t, targetAgent, restored.Workspace(), restored)
		require.Equal(t, entries, recordedExpertise(ctx, t, target, restored))
		require.ElementsMatch(t, []string{"first updated", "second updated"}, recomposeSystemPrompts(ctx, t, target, restored))
	})
}

func (LLMSuite) TestRecomposeRefindsOrFallsBack(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	const source = `
type Swapper {
  first(base: LLM!): LLM! { base.withSystemPrompt("first original") }
}
`
	ws := recomposeFixture(c, source).AsWorkspace()
	agent, llm := spawnURI(ctx, t, c, ws, core.NewQuery(c).LLM(), "swapper/first")
	entries := recordedExpertise(ctx, t, c, llm)
	require.Len(t, entries, 1)

	// The bound workspace addresses the entry: it is found again there.
	edited := ws.WithNewFile(recomposeModulePath, strings.ReplaceAll(source, "original", "edited"))
	llm, err := refreshLLM(ctx, agent, edited, llm)
	require.NoError(t, err)
	require.Equal(t, []string{"first edited"}, recomposeSystemPrompts(ctx, t, c, llm))
	require.Equal(t, entries, recordedExpertise(ctx, t, c, llm))

	// Another repository's workspace does not address the entry, as after
	// switching to its checkout. The recorded entry reruns from its own
	// source rather than being dropped or failing.
	other := core.NewQuery(c).Directory().
		WithNewFile("dagger.toml", "[modules.unrelated]\nsource = \"unrelated\"\n").
		WithNewFile("unrelated/dagger.json", `{"name":"unrelated","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile("unrelated/main.dang", "type Unrelated {\n  ping: String! { \"pong\" }\n}\n").
		AsWorkspace()
	llm, err = refreshLLM(ctx, agent, other, llm)
	require.NoError(t, err)
	require.Equal(t, []string{"first edited"}, recomposeSystemPrompts(ctx, t, c, llm))
	require.Equal(t, entries, recordedExpertise(ctx, t, c, llm))
	_, err = llm.Workspace().File("unrelated/main.dang").Contents(ctx)
	require.NoError(t, err, "recompose keeps the conversation's bound workspace")

	// Bound to a workspace that addresses it again, the entry is found there.
	returned := edited.WithNewFile(recomposeModulePath, strings.ReplaceAll(source, "original", "returned"))
	llm, err = refreshLLM(ctx, agent, returned, llm)
	require.NoError(t, err)
	require.Equal(t, []string{"first returned"}, recomposeSystemPrompts(ctx, t, c, llm))
	require.Equal(t, entries, recordedExpertise(ctx, t, c, llm))
}

func (LLMSuite) TestComposeTransformDimensionIdentity(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	const source = `
type Swapper {
  members: Members! { Members() }
}
type Members @collection {
  names: [String!]! @keys
  new() {
    self.names = ["one", "two"]
    self
  }
  member(name: String!): Member! @get { Member(name) }
}
type Member {
  let name: String!
  new(name: String!) {
    self.name = name
    self
  }
  transform(base: LLM!): LLM! { base.withSystemPrompt("original member: " + name) }
}
`
	ws := recomposeFixture(c, source).AsWorkspace()
	artifacts, err := ws.Artifacts().FilterTypes([]string{"LLM"}).Items(ctx)
	require.NoError(t, err)
	require.Len(t, artifacts, 2)
	firstURI, err := artifacts[0].URI(ctx)
	require.NoError(t, err)
	secondURI, err := artifacts[1].URI(ctx)
	require.NoError(t, err)
	agent, llm := spawnURI(ctx, t, c, ws, core.NewQuery(c).LLM(), firstURI)
	llm, err = composeURI(ctx, agent, ws, llm, secondURI)
	require.NoError(t, err)
	entries := recordedExpertise(ctx, t, c, llm)
	require.Len(t, entries, 2)
	require.Contains(t, entries[0].Artifact.URI, "member=one")
	require.Contains(t, entries[1].Artifact.URI, "member=two")
	require.ElementsMatch(t, []string{"original member: one", "original member: two"}, recomposeSystemPrompts(ctx, t, c, llm))
	// Agent.recompose finds each entry again by its URI, dimension keys included.
	ws = ws.WithNewFile(recomposeModulePath, strings.ReplaceAll(source, "original member", "updated member"))
	llm, err = refreshLLM(ctx, agent, ws, llm)
	require.NoError(t, err)
	require.Equal(t, entries, recordedExpertise(ctx, t, c, llm))
	require.ElementsMatch(t, []string{"updated member: one", "updated member: two"}, recomposeSystemPrompts(ctx, t, c, llm))
	// A collection item the workspace no longer has is not addressed there:
	// it reruns from its recorded source while its sibling is found again.
	ws = ws.WithNewFile(recomposeModulePath, strings.NewReplacer(`["one", "two"]`, `["one"]`, "original member", "renamed member").Replace(source))
	llm, err = refreshLLM(ctx, agent, ws, llm)
	require.NoError(t, err)
	require.Equal(t, entries, recordedExpertise(ctx, t, c, llm))
	require.ElementsMatch(t, []string{"renamed member: one", "updated member: two"}, recomposeSystemPrompts(ctx, t, c, llm))
}

func (AgentRuntimeSuite) TestParentLineageAndRestore(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	root := spawnAgent(ctx, t, c, spawnOpts{model: emptyReplayModel, name: "root"})
	rootInfo := root.mustRun(ctx, t, "handle parent { handle }")
	require.Nil(t, rootInfo.Get("parent").Value())
	rootHandle := rootInfo.Get("handle").String()
	child := spawnAgent(ctx, t, c, spawnOpts{model: emptyReplayModel, name: "child", handle: rootHandle + "-child", parentHandle: rootHandle})
	childInfo := child.mustRun(ctx, t, "handle parent { name handle parent { handle } }")
	require.Equal(t, rootHandle, childInfo.Get("parent.handle").String())
	require.Equal(t, "root", childInfo.Get("parent.name").String())
	require.Nil(t, childInfo.Get("parent.parent").Value())
	// Restore lineage independently of either conversation seed. In the fresh
	// session both runtimes are minted from the captured conversation recipes.
	rootSnapshot := core.Ref[*core.LLM](core.NewQuery(c), core.ID(root.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
	childSnapshot := core.Ref[*core.LLM](core.NewQuery(c), core.ID(child.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
	rootRecipe, err := sink.captureLLMRecipe(ctx, t, c, rootSnapshot)
	require.NoError(t, err)
	childRecipe, err := sink.captureLLMRecipe(ctx, t, c, childSnapshot)
	require.NoError(t, err)
	target := connect(ctx, t)
	restoredRoot, err := rehydrateAgent(ctx, target, string(rootRecipe), rootHandle, "root", "IDLE", "")
	require.NoError(t, err)
	restoredChild, err := rehydrateAgentWithParent(ctx, target, string(childRecipe), childInfo.Get("handle").String(), "child", "IDLE", "", rootHandle)
	require.NoError(t, err)
	require.Nil(t, restoredRoot.mustRun(ctx, t, "parent { handle }").Get("parent").Value())
	restored := restoredChild.mustRun(ctx, t, "parent { name handle parent { handle } }")
	require.Equal(t, rootHandle, restored.Get("parent.handle").String())
	require.Equal(t, "root", restored.Get("parent.name").String())
	require.Nil(t, restored.Get("parent.parent").Value())
}

func (LLMSuite) TestSpawnExpertiseFailureCanRetry(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	const source = `
type Swapper {
  broken(base: LLM!, me: Agent!): LLM! { raise "composition deliberately failed" }
  ready(base: LLM!, me: Agent!): LLM! { base.withSystemPrompt("ready: " + me.name) }
  fresh(base: LLM!): LLM! { llm(model: "recording/W10=") }
  family(base: LLM!): LLM! {
    let child = llm(model: "recording/W10=").withWorkspace(base.workspace).spawn(name: "nested-child", expertise: base.workspace.artifacts.filterUri("swapper/ready").asExpertise.{{ id }}.map { node(id: _.id).{{ ... on Expertise! }} })
    base.withSystemPrompt("nested child: " + child.handle)
  }
}
`
	ws := recomposeFixture(c, source).AsWorkspace()
	base := core.NewQuery(c).LLM(core.LLMOpts{Model: emptyReplayModel}).WithWorkspace(ws)
	id, err := base.ID(ctx)
	require.NoError(t, err)
	broken, err := uriExpertiseIDs(ctx, c, ws, "swapper/broken")
	require.NoError(t, err)
	var result struct{ Node struct{ Spawn string } }
	err = c.Do(ctx, &dagger.Request{
		Query:     `query($id: ID!, $entries: [ID!]!) { node(id: $id) { ... on LLM { spawn(name: "retry", expertise: $entries) } } }`,
		Variables: map[string]any{"id": id, "entries": broken},
	}, &dagger.Response{Data: &result})
	require.ErrorContains(t, err, "composition deliberately failed")
	ready, err := uriExpertiseIDs(ctx, c, ws, "swapper/ready")
	require.NoError(t, err)
	retry := spawnWithExpertise(ctx, t, c, base, "retry", ready)
	require.Equal(t, "retry", retry.mustRun(ctx, t, "name").Get("name").String())
	retrySnapshot := core.Ref[*core.LLM](core.NewQuery(c), core.ID(retry.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
	require.Contains(t, recomposeSystemPrompts(ctx, t, c, retrySnapshot), "ready: retry")

	t.Run("fresh conversation retains spawn owner", func(ctx context.Context, t *testctx.T) {
		fresh, err := uriExpertiseIDs(ctx, c, ws, "swapper/fresh")
		require.NoError(t, err)
		require.Len(t, fresh, 1)
		require.Len(t, ready, 1)
		agent := spawnWithExpertise(ctx, t, c, base, "fresh-agent", []core.ID{fresh[0], ready[0]})
		snapshot := core.Ref[*core.LLM](core.NewQuery(c), core.ID(agent.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
		require.Contains(t, recomposeSystemPrompts(ctx, t, c, snapshot), "ready: fresh-agent")
	})

	t.Run("nested spawn has its own composition record", func(ctx context.Context, t *testctx.T) {
		family, err := uriExpertiseIDs(ctx, c, ws, "swapper/family")
		require.NoError(t, err)
		parent := spawnWithExpertise(ctx, t, c, base, "family", family)
		snapshot := core.Ref[*core.LLM](core.NewQuery(c), core.ID(parent.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
		prompts := recomposeSystemPrompts(ctx, t, c, snapshot)
		require.Len(t, prompts, 1)
		require.True(t, strings.HasPrefix(prompts[0], "nested child: "))
		child := unmintedAgent(ctx, t, c, strings.TrimPrefix(prompts[0], "nested child: "), "nested-child")
		childSnapshot := core.Ref[*core.LLM](core.NewQuery(c), core.ID(child.mustRun(ctx, t, "snapshot { id }").Get("snapshot.id").String()))
		entries := recordedExpertise(ctx, t, c, childSnapshot)
		require.Len(t, entries, 1)
		require.Contains(t, entries[0].Artifact.URI, "swapper/ready")
		require.Contains(t, recomposeSystemPrompts(ctx, t, c, childSnapshot), "ready: nested-child")
		require.Equal(t, "family", child.mustRun(ctx, t, "parent { name }").Get("parent.name").String())
	})
}

func (LLMSuite) TestComposeTransformReceivesAgent(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	const source = `
type Swapper {
  required(base: LLM!, me: Agent!): LLM! { base.withSystemPrompt("required: " + me.name) }
  optional(base: LLM!, me: Agent): LLM! {
    if (me == null) { raise "optional agent was not filled" }
    base.withSystemPrompt("optional: " + me.name)
  }
}
`
	ws := recomposeFixture(c, source).AsWorkspace()
	required, err := uriExpertiseIDs(ctx, c, ws, "swapper/required")
	require.NoError(t, err, "conversion does not require an agent to have been minted yet")
	require.Len(t, required, 1)
	optional, err := uriExpertiseIDs(ctx, c, ws, "swapper/optional")
	require.NoError(t, err)
	require.Len(t, optional, 1)
	receiver := spawnWithExpertise(ctx, t, c, core.NewQuery(c).LLM().WithWorkspace(ws), "receiver", []core.ID{})
	other := spawnWithExpertise(ctx, t, c, core.NewQuery(c).LLM().WithWorkspace(ws), "other", []core.ID{})

	// The receiver adopts an unowned conversation. Every Agent argument,
	// required or optional, receives the receiver.
	llm, err := composeAs(ctx, receiver, core.NewQuery(c).LLM().WithWorkspace(ws), []core.ID{required[0], optional[0]})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"required: receiver", "optional: receiver"}, recomposeSystemPrompts(ctx, t, c, llm))
	llm, err = recomposeAs(ctx, receiver, llm)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"required: receiver", "optional: receiver"}, recomposeSystemPrompts(ctx, t, c, llm))

	// The conversation now belongs to the receiver: another agent can neither
	// compose nor recompose it.
	_, err = composeAs(ctx, other, llm, required)
	require.ErrorContains(t, err, "belongs to agent")
	_, err = recomposeAs(ctx, other, llm)
	require.ErrorContains(t, err, "belongs to agent")

	// Without a conversation, an agent composes its own snapshot.
	ids := map[string]any{"agent": receiver.agentID, "expertise": optional}
	var res struct {
		Node struct{ Compose struct{ ID core.ID } }
	}
	require.NoError(t, c.Do(ctx, &dagger.Request{
		Query:     `query($agent: ID!, $expertise: [ID!]!) { node(id: $agent) { ... on Agent { compose(expertise: $expertise) { id } } } }`,
		Variables: ids,
	}, &dagger.Response{Data: &res}))
	snapshot := core.Ref[*core.LLM](core.NewQuery(c), res.Node.Compose.ID)
	require.Equal(t, []string{"optional: receiver"}, recomposeSystemPrompts(ctx, t, c, snapshot))
}

func (LLMSuite) TestRecomposeDirectCallerLeavesForeignToolsUnowned(ctx context.Context, t *testctx.T) {
	c, sink := connectWithTrace(ctx, t)
	fixture := core.NewQuery(c).Directory().
		WithNewFile("dagger.toml", "[modules.owner]\nsource = \"owner\"\n[modules.donor]\nsource = \"owner/donor\"\n").
		WithNewFile("owner/dagger.json", `{"name":"owner","engineVersion":"v1.0.0-0","sdk":"dang","dependencies":[{"name":"donor","source":"donor"}]}`).
		WithNewFile("owner/main.dang", `
type Owner {
  contribute(base: LLM!): LLM! { base.withTools(donor) }
}
`).
		WithNewFile("owner/donor/dagger.json", `{"name":"donor","engineVersion":"v1.0.0-0","sdk":"dang"}`).
		WithNewFile("owner/donor/main.dang", `
type Donor {
  agent(base: LLM!): LLM! @agent { base }
  ping: String! { "foreign tool" }
}
`)
	ws := fixture.AsWorkspace()
	require.NoError(t, ws.ModuleSource("owner").AsModule().Serve(ctx))
	baseID, err := core.NewQuery(c).LLM().WithWorkspace(ws).ID(ctx)
	require.NoError(t, err)
	var res struct {
		Owner struct{ Contribute struct{ ID string } }
	}
	require.NoError(t, c.Do(ctx, &dagger.Request{
		Query:     `query($base: ID!) { owner { contribute(base: $base) { id } } }`,
		Variables: map[string]any{"base": baseID},
	}, &dagger.Response{Data: &res}))
	llm := core.Ref[*core.LLM](core.NewQuery(c), core.ID(res.Owner.Contribute.ID))
	tools, err := llm.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "## ping\n")
	agent, llm, err := spawnComposed(ctx, t, c, llm, nil)
	require.NoError(t, err)

	// The bound object's defining module is not the caller. Selecting Donor
	// must leave the binding installed by a direct call inside Owner alone.
	llm, err = recomposeLLM(ctx, t, agent, ws, llm, "donor")
	require.NoError(t, err)
	tools, err = llm.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "## ping\n")
	portable, err := sink.captureLLMRecipe(ctx, t, c, llm)
	require.NoError(t, err)
	c = connect(ctx, t)
	agent, llm = restoreComposer(ctx, t, c, agent, portable)

	// Owner installed the binding outside an entry run. Adding its new agent
	// entry must not claim or discard that unowned foreign tool.
	ws = llm.Workspace().WithNewFile("owner/main.dang", `
type Owner {
  agent(base: LLM!): LLM! @agent { base }
}
`)
	llm, err = recomposeLLM(ctx, t, agent, ws, llm, "owner")
	require.NoError(t, err)
	tools, err = llm.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "## ping\n", "unowned foreign binding survives refresh")

	// Merely sharing a type does not authorize an entry to take over a
	// manually installed binding. Ownership cannot change during refresh.
	claiming := ws.WithNewFile("owner/main.dang", `
type Owner {
  agent(base: LLM!): LLM! @agent { base.withTools(donor) }
}
`)
	_, err = composeLLM(ctx, agent, claiming, llm, "owner")
	require.ErrorContains(t, err, "owned by other expertise")
	require.ErrorContains(t, err, "unowned -> expertise")
	tools, err = llm.Tools(ctx)
	require.NoError(t, err)
	require.Contains(t, tools, "## ping\n", "rejected ownership transfer leaves the original binding usable")
}
