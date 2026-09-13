package server

import (
	"context"
	"strconv"
	"sync"
)

type sessionApproval struct {
	done    chan struct{}
	allowed bool
	err     error
}

// Lives only in daggerSession, never in a result, recipe, or client prompt key.
// Remember denials too, so a tool retry cannot badger the user into approving.
type sessionApprovals[K comparable] struct {
	mu        sync.Mutex
	decisions map[K]*sessionApproval
}

func (a *sessionApprovals[K]) check(ctx context.Context, key K, ask func(context.Context) (bool, error)) (bool, error) {
	a.mu.Lock()
	if decision, ok := a.decisions[key]; ok {
		a.mu.Unlock()
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-decision.done:
			return decision.allowed, decision.err
		}
	}
	decision := &sessionApproval{done: make(chan struct{})}
	if a.decisions == nil {
		a.decisions = make(map[K]*sessionApproval)
	}
	a.decisions[key] = decision
	a.mu.Unlock()
	decision.allowed, decision.err = ask(ctx)
	a.mu.Lock()
	// A canceled or unavailable prompt is not a user decision.
	if decision.err != nil {
		delete(a.decisions, key)
	}
	close(decision.done)
	a.mu.Unlock()
	return decision.allowed, decision.err
}

// promptLiteral renders untrusted text for an approval prompt: literal,
// escaped for terminals (including bidi/control characters), not Markdown.
// Neither credentials nor module-supplied prose belong in a prompt.
func promptLiteral(s string) string {
	quoted := strconv.QuoteToASCII(s)
	return quoted[1 : len(quoted)-1]
}
