// A module whose tool messages the agent that called it, exercising Agent!
// argument handling (hack/designs/async-agents.md §3.1): when poke is
// dispatched as a tool from a running agent loop, MCP passes the calling
// agent's handle into the parent argument, which is hidden from the model's
// tool schema but remains required in the module's GraphQL schema.
package main

import (
	"context"

	"dagger/poker/internal/dagger"
)

type Poker struct{}

// Poke fire-and-forgets a note to the calling agent — the child->parent
// channel — and confirms that the send completed. It must never await the
// message's delivery or reply: the note joins the very turn this tool call is
// part of, so awaiting would deadlock (the self-await hazard, design §9).
//
// (The MCP-supplied argument is named caller rather than parent because the Go
// SDK's generated dispatch code reserves `parent` for the receiver.)
func (m *Poker) Poke(ctx context.Context, caller *dagger.Agent, note string) (string, error) {
	if _, err := caller.Send(ctx, note); err != nil {
		return "", err
	}
	return "sent", nil
}

// MaybePoke is Poke with an optional caller: inside an agent loop MCP passes
// the calling agent, and without one (a synchronous loop, dagger mcp) it
// passes null. Either way the argument never reaches the model's schema.
func (m *Poker) MaybePoke(
	ctx context.Context,
	// +optional
	caller *dagger.Agent,
	note string,
) (string, error) {
	if caller == nil {
		return "no caller for " + note, nil
	}
	return m.Poke(ctx, caller, note)
}
