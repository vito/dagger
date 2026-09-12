package server

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/dagger/dagger/core"
)

// ClientWorkspaceDebugState describes one connected client's workspace state
// for the /debug/client/workspace endpoint. It exists to make "why does my
// session have no modules/agents" answerable with one curl: a client that
// silently fell back to a rootless workspace shows up here with a rootless
// source kind, an empty config file, and zero pending modules.
type ClientWorkspaceDebugState struct {
	SessionID string `json:"sessionID"`
	ClientID  string `json:"clientID"`

	// WorkspaceLoaded reports whether workspace detection has run for this
	// client; Workspace stays nil until it has (or when detection errored).
	WorkspaceLoaded bool   `json:"workspaceLoaded"`
	WorkspaceError  string `json:"workspaceError,omitempty"`

	Workspace *WorkspaceDebugState `json:"workspace,omitempty"`

	PendingModuleCount int      `json:"pendingModuleCount"`
	PendingModules     []string `json:"pendingModules,omitempty"`

	ServedWorkspaceModuleCount int      `json:"servedWorkspaceModuleCount"`
	ServedWorkspaceModules     []string `json:"servedWorkspaceModules,omitempty"`

	FailedModuleCount int               `json:"failedModuleCount"`
	FailedModules     map[string]string `json:"failedModules,omitempty"`
}

// WorkspaceDebugState is the workspace-value half of ClientWorkspaceDebugState.
type WorkspaceDebugState struct {
	// SourceKind and BaseSourceKind are the Go types (%T) of the workspace's
	// source and base source, e.g. *core.WorkspaceSourceClientLocal for a
	// git-rooted host checkout vs *core.WorkspaceSourceRootlessLocal for the
	// no-git fallback stub.
	SourceKind     string `json:"sourceKind"`
	BaseSourceKind string `json:"baseSourceKind"`

	HostPath    string `json:"hostPath,omitempty"`
	Address     string `json:"address,omitempty"`
	Cwd         string `json:"cwd"`
	ConfigFile  string `json:"configFile,omitempty"`
	LockFile    string `json:"lockFile,omitempty"`
	SelectedEnv string `json:"selectedEnv,omitempty"`
}

// ClientWorkspaceDebugStates snapshots the workspace state of every client of
// every initialized session. Like the other observer paths (Clients,
// activeClientIDs) it takes no per-session lifecycle lock: sessions are
// snapshotted under daggerSessionsMu, clients under each session's clientMu,
// and per-client state under that client's own mutexes, so a session stuck
// initializing or tearing down cannot stall the debug endpoint.
func (srv *Server) ClientWorkspaceDebugStates() []ClientWorkspaceDebugState {
	srv.daggerSessionsMu.RLock()
	sessions := make([]*daggerSession, 0, len(srv.daggerSessions))
	for _, sess := range srv.daggerSessions {
		sessions = append(sessions, sess)
	}
	srv.daggerSessionsMu.RUnlock()

	states := []ClientWorkspaceDebugState{}
	for _, sess := range sessions {
		if sess.state.Load() != sessionStateInitialized {
			continue
		}
		sess.clientMu.RLock()
		clients := make([]*daggerClient, 0, len(sess.clients))
		for _, client := range sess.clients {
			clients = append(clients, client)
		}
		sess.clientMu.RUnlock()

		for _, client := range clients {
			states = append(states, clientWorkspaceDebugState(sess.sessionID, client))
		}
	}

	slices.SortFunc(states, func(a, b ClientWorkspaceDebugState) int {
		if c := cmp.Compare(a.SessionID, b.SessionID); c != 0 {
			return c
		}
		return cmp.Compare(a.ClientID, b.ClientID)
	})
	return states
}

func clientWorkspaceDebugState(sessionID string, client *daggerClient) ClientWorkspaceDebugState {
	state := ClientWorkspaceDebugState{
		SessionID: sessionID,
		ClientID:  client.clientID,
	}

	client.workspaceMu.Lock()
	state.WorkspaceLoaded = client.workspaceLoaded
	if client.workspaceErr != nil {
		state.WorkspaceError = client.workspaceErr.Error()
	}
	ws := client.workspace
	client.workspaceMu.Unlock()

	state.Workspace = workspaceDebugState(ws)

	client.modulesMu.Lock()
	for _, mod := range client.pendingModules {
		state.PendingModules = append(state.PendingModules, moduleProgressName(mod))
	}
	for name := range client.servedWorkspaceModuleNames {
		state.ServedWorkspaceModules = append(state.ServedWorkspaceModules, name)
	}
	for name, err := range client.failedModules {
		if state.FailedModules == nil {
			state.FailedModules = map[string]string{}
		}
		state.FailedModules[name] = err.Error()
	}
	client.modulesMu.Unlock()

	slices.Sort(state.ServedWorkspaceModules)
	state.PendingModuleCount = len(state.PendingModules)
	state.ServedWorkspaceModuleCount = len(state.ServedWorkspaceModules)
	state.FailedModuleCount = len(state.FailedModules)
	return state
}

func workspaceDebugState(ws *core.Workspace) *WorkspaceDebugState {
	if ws == nil {
		return nil
	}
	return &WorkspaceDebugState{
		SourceKind:     fmt.Sprintf("%T", ws.Source()),
		BaseSourceKind: fmt.Sprintf("%T", ws.BaseSource()),
		HostPath:       ws.HostPath(),
		Address:        ws.Address,
		Cwd:            ws.Cwd,
		ConfigFile:     ws.ConfigFile,
		LockFile:       ws.LockFile,
		SelectedEnv:    ws.SelectedEnv(),
	}
}
