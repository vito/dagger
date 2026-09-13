package core

import (
	"context"
	"os"
	"path/filepath"

	"dagger.io/dagger"
	"github.com/dagger/testctx"
	"github.com/stretchr/testify/require"
)

// TestExportAgentToolRequiresApproval is TestPushModuleRequiresApproval's
// sibling for host writes: an agent tool call runs core APIs in the owner's
// context, so an export-family tool can reach the owner's checkout. Without
// an interactive prompt attachable the approval must fail closed, while the
// same export as a direct user API call is implicitly authorized.
func (WorkspaceSuite) TestExportAgentToolRequiresApproval(ctx context.Context, t *testctx.T) {
	// Subtests run in parallel and the direct-export control advances HEAD,
	// so each gets its own checkout and client.
	setup := func(ctx context.Context, t *testctx.T) (string, func(...string) string, *dagger.Client, *dagger.Workspace) {
		checkout, git := workspaceExportCheckout(ctx, t)
		c := connect(ctx, t, dagger.WithWorkdir(checkout))
		agent := snapshotWorkspace(ctx, t, c, c.CurrentWorkspace()).
			WithNewFile("agent.txt", "from agent").
			WithCommit("agent change", workspaceCommitDate)
		agentID, err := agent.ID(ctx)
		require.NoError(t, err)
		return checkout, git, c, dagger.Ref[*dagger.Workspace](c, agentID)
	}

	exportModel := func(ctx context.Context, t *testctx.T, c *dagger.Client, prompt string, arguments dagger.JSON) string {
		return cannedReplayModel(ctx, t, c, c.LLM().
			WithPrompt(prompt).
			WithResponse([]dagger.LLMContentBlockInput{{
				Kind: dagger.LLMContentBlockKindToolCall, CallID: "call_1", ToolName: "export",
				Arguments: arguments,
			}}).
			// Placeholder result: the real tool runs during replay, and its
			// live refusal flows into the transcript.
			WithToolResult("call_1", "", true).
			WithResponse([]dagger.LLMContentBlockInput{{
				Kind: dagger.LLMContentBlockKindText, Text: "done",
			}}))
	}

	t.Run("an agent workspace export is refused without approval", func(ctx context.Context, t *testctx.T) {
		checkout, git, c, agent := setup(ctx, t)
		head := git("rev-parse", "HEAD")
		model := exportModel(ctx, t, c, "save your work", "")
		transcript, err := c.LLM(dagger.LLMOpts{Model: model}).
			WithTools(c.CurrentWorkspace().WithCommitsFrom(agent)).
			WithPrompt("save your work").
			Loop().
			Transcript(ctx)
		require.NoError(t, err)
		require.Contains(t, transcript, "saving workspace changes requires approval")
		// Neither HEAD nor the worktree moved.
		require.Equal(t, head, git("rev-parse", "HEAD"))
		_, err = os.Stat(filepath.Join(checkout, "agent.txt"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("an agent directory export is refused without approval", func(ctx context.Context, t *testctx.T) {
		checkout, _, c, _ := setup(ctx, t)
		model := exportModel(ctx, t, c, "export it", dagger.JSON(`{"path":"exfiltrated"}`))
		transcript, err := c.LLM(dagger.LLMOpts{Model: model}).
			WithTools(c.Directory().WithNewFile("evil.txt", "boo")).
			WithPrompt("export it").
			Loop().
			Transcript(ctx)
		require.NoError(t, err)
		require.Contains(t, transcript, "exporting a directory requires approval")
		_, err = os.Stat(filepath.Join(checkout, "exfiltrated"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("a direct export needs no approval", func(ctx context.Context, t *testctx.T) {
		checkout, git, c, agent := setup(ctx, t)
		head := git("rev-parse", "HEAD")
		// The same integration saved by the user directly never prompts, so
		// it succeeds without any prompt attachable in the session.
		require.NoError(t, c.CurrentWorkspace().WithCommitsFrom(agent).Export(ctx))
		require.NotEqual(t, head, git("rev-parse", "HEAD"))
		_, err := os.Stat(filepath.Join(checkout, "agent.txt"))
		require.NoError(t, err)
	})
}
