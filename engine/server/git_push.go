package server

import (
	"context"
	"fmt"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/engine"
	"github.com/dagger/dagger/engine/session/prompt"
)

type gitPushApprovalKey struct {
	owner, remote, ref string
	force              bool
}

func (srv *Server) AuthorizeGitPush(ctx context.Context, remote, ref string, force bool) (*engine.ClientMetadata, error) {
	client, err := srv.clientFromContext(ctx)
	if err != nil {
		return nil, err
	}
	owner, delegated, err := gitPushOwner(client)
	if err != nil {
		return nil, err
	}
	// Calling push directly is implicit authorization for this exact operation.
	// Tool execution in the owner's context is not a direct user API call.
	if !delegated && !core.IsAgentToolCall(ctx) {
		return owner.clientMetadata, nil
	}
	key := gitPushApprovalKey{owner: owner.clientID, remote: remote, ref: ref, force: force}
	allowed, err := client.daggerSession.gitPushApprovals.check(ctx, key, func(ctx context.Context) (bool, error) {
		conn, available, err := srv.SpecificClientAttachableConn(ctx, owner.clientID, core.SpecificClientAttachableConnOpts{IfAvailable: true})
		if err != nil {
			return false, err
		}
		if !available {
			return false, fmt.Errorf("owning client is not available to approve the push")
		}
		action := "pushing"
		if force {
			action = "force pushing"
		}
		response, err := prompt.NewPromptClient(conn).PromptBool(ctx, &prompt.BoolRequest{
			Prompt: fmt.Sprintf("Allow %s to %s @ %s?", action, promptLiteral(remote), promptLiteral(ref)),
		})
		if err != nil {
			return false, err
		}
		return response.Response, nil
	})
	if err != nil {
		return nil, fmt.Errorf("git push requires approval: %w", err)
	}
	if !allowed {
		action := "push"
		if force {
			action = "force push"
		}
		return nil, fmt.Errorf("git %s permission denied by the owning client for %s for this session", action, ref)
	}
	return owner.clientMetadata, nil
}

// A module cannot regain implicit authorization by spawning a non-module
// nested client. Use the trusted caller immediately before the first module
// boundary, not the nearest non-module client after that boundary.
func gitPushOwner(client *daggerClient) (*daggerClient, bool, error) {
	var owner *daggerClient
	for _, parent := range client.parents {
		if parent.mod.Self() != nil {
			if owner == nil {
				return nil, true, fmt.Errorf("no owning client for git push")
			}
			return owner, true, nil
		}
		owner = parent
	}
	if client.mod.Self() != nil {
		if owner == nil {
			return nil, true, fmt.Errorf("no owning client for git push")
		}
		return owner, true, nil
	}
	return client, false, nil
}
