package server

import (
	"context"
	"fmt"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/engine/session/prompt"
)

type hostWriteApprovalKey struct {
	owner, action, dest string
}

// AuthorizeHostWrite is AuthorizeGitPush's sibling for exports that write to
// the calling client's host (Workspace/Changeset/Directory/File/Container
// export). Calling export directly is implicit authorization for this exact
// operation; tool execution in the owner's context is not a direct user API
// call, so it requires the owner's per-session approval. Module clients need
// no owner resolution here: their host attachables are inert, so a module's
// export cannot reach any host with or without approval.
func (srv *Server) AuthorizeHostWrite(ctx context.Context, action, dest string) error {
	client, err := srv.clientFromContext(ctx)
	if err != nil {
		return err
	}
	if !core.IsAgentToolCall(ctx) {
		return nil
	}
	key := hostWriteApprovalKey{owner: client.clientID, action: action, dest: dest}
	allowed, err := client.daggerSession.hostWriteApprovals.check(ctx, key, func(ctx context.Context) (bool, error) {
		conn, available, err := srv.SpecificClientAttachableConn(ctx, client.clientID, core.SpecificClientAttachableConnOpts{IfAvailable: true})
		if err != nil {
			return false, err
		}
		if !available {
			return false, fmt.Errorf("owning client is not available to approve the write")
		}
		response, err := prompt.NewPromptClient(conn).PromptBool(ctx, &prompt.BoolRequest{
			Prompt: fmt.Sprintf("Allow %s to %s?", action, promptLiteral(dest)),
		})
		if err != nil {
			return false, err
		}
		return response.Response, nil
	})
	if err != nil {
		return fmt.Errorf("%s requires approval: %w", action, err)
	}
	if !allowed {
		return fmt.Errorf("%s to %s permission denied by the owning client for this session", action, dest)
	}
	return nil
}
