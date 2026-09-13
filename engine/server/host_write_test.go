package server

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/engine"
	"github.com/dagger/dagger/engine/session/prompt"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type hostWritePromptServer struct {
	prompt.UnimplementedPromptServer
	requests []*prompt.BoolRequest
}

func (p *hostWritePromptServer) PromptBool(_ context.Context, req *prompt.BoolRequest) (*prompt.BoolResponse, error) {
	p.requests = append(p.requests, req)
	return &prompt.BoolResponse{Response: !strings.Contains(req.Prompt, "/denied")}, nil
}

func TestHostWriteApprovalAgentGate(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	grpcServer := grpc.NewServer()
	questions := &hostWritePromptServer{}
	prompt.RegisterPromptServer(grpcServer, questions)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)
	conn, err := grpc.NewClient("passthrough:///prompt", grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	sess := &daggerSession{sessionID: "session", attachables: newSessionAttachableManager()}
	sess.state.Store(sessionStateInitialized)
	owner := &daggerClient{clientID: "owner", daggerSession: sess, clientMetadata: &engine.ClientMetadata{ClientID: "owner", SessionID: "session"}}
	sess.clients = map[string]*daggerClient{"owner": owner}
	sess.attachables.callers["owner"] = &sessionAttachableCaller{ctx: t.Context(), conn: conn}
	srv := &Server{daggerSessions: map[string]*daggerSession{"session": sess}}
	ownerCtx := engine.ContextWithClientMetadata(t.Context(), owner.clientMetadata)
	agentCtx := core.WithAgentToolCall(ownerCtx)

	// Calling export directly is implicit authorization: no prompt.
	require.NoError(t, srv.AuthorizeHostWrite(ownerCtx, "saving workspace changes", "/checkout"))
	require.Empty(t, questions.requests)

	// An agent tool call prompts once per action and destination; the grant
	// is remembered for the session.
	for range 2 {
		require.NoError(t, srv.AuthorizeHostWrite(agentCtx, "saving workspace changes", "/checkout"))
	}
	require.Len(t, questions.requests, 1)
	require.Equal(t, "Allow saving workspace changes to /checkout?", questions.requests[0].Prompt)
	require.Empty(t, questions.requests[0].PersistentKey)

	// A denial is remembered too, so a tool retry cannot badger the user.
	for range 2 {
		err := srv.AuthorizeHostWrite(agentCtx, "saving workspace changes", "/denied")
		require.ErrorContains(t, err, "permission denied")
	}
	require.Len(t, questions.requests, 2)

	// A different destination or action is a separate decision.
	require.NoError(t, srv.AuthorizeHostWrite(agentCtx, "exporting a directory", "/checkout"))
	require.Len(t, questions.requests, 3)

	// Destinations render literally, with terminal control sequences escaped.
	require.NoError(t, srv.AuthorizeHostWrite(agentCtx, "exporting a file", "/evil\x1b[2J"))
	require.Equal(t, `Allow exporting a file to /evil\x1b[2J?`, questions.requests[3].Prompt)

	// Without a prompt channel the write is refused rather than allowed.
	headless := &daggerClient{clientID: "headless", daggerSession: sess, clientMetadata: &engine.ClientMetadata{ClientID: "headless", SessionID: "session"}}
	sess.clients["headless"] = headless
	headlessCtx := core.WithAgentToolCall(engine.ContextWithClientMetadata(t.Context(), headless.clientMetadata))
	err = srv.AuthorizeHostWrite(headlessCtx, "saving workspace changes", "/checkout")
	require.ErrorContains(t, err, "saving workspace changes requires approval")
	require.Len(t, questions.requests, 4)
}
