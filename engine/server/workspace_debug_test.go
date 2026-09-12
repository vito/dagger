package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dagger/dagger/core"
)

func TestClientWorkspaceDebugStates(t *testing.T) {
	t.Parallel()

	// A rootless client — the silent failure mode this endpoint exists to
	// expose: detection found no git root, so the workspace is a stub with no
	// config file and no pending modules.
	rootlessWS := &core.Workspace{
		Address:  "/tmp/no-git/app",
		Cwd:      ".",
		ClientID: "client-rootless",
	}
	rootlessWS.SetHostPath("/tmp/no-git/app")
	rootlessWS.SetSource(core.NewWorkspaceSourceRootlessLocal("/tmp/no-git/app"))
	rootlessClient := &daggerClient{
		clientID:        "client-rootless",
		workspaceLoaded: true,
		workspace:       rootlessWS,
	}

	// A healthy git-rooted client with pending, served, and failed modules.
	gitWS := &core.Workspace{
		Address:    "/srv/repo",
		Cwd:        "app",
		ConfigFile: "dagger.toml",
		LockFile:   "dagger.lock",
		ClientID:   "client-git",
	}
	gitWS.SetHostPath("/srv/repo")
	gitWS.SetSource(core.NewWorkspaceSourceClientLocal("/srv/repo"))
	gitWS.SetSelectedEnv("dev")
	gitClient := &daggerClient{
		clientID:        "client-git",
		workspaceLoaded: true,
		workspace:       gitWS,
		pendingModules: []pendingModule{
			{Name: "pending-one"},
			{Ref: "./modules/pending-two"},
		},
		servedWorkspaceModuleNames: map[string]struct{}{
			"served-one": {},
		},
		failedModules: map[string]error{
			"broken-one": errors.New("compile error"),
		},
	}

	// A client whose detection errored.
	erroredClient := &daggerClient{
		clientID:        "client-errored",
		workspaceLoaded: true,
		workspaceErr:    errors.New("workspace detection: boom"),
	}

	srv := &Server{daggerSessions: map[string]*daggerSession{}}
	sess := &daggerSession{
		sessionID: "sess-1",
		clients: map[string]*daggerClient{
			rootlessClient.clientID: rootlessClient,
			gitClient.clientID:      gitClient,
			erroredClient.clientID:  erroredClient,
		},
	}
	sess.state.Store(sessionStateInitialized)
	srv.daggerSessions[sess.sessionID] = sess

	// Uninitialized sessions must not appear.
	pendingSess := &daggerSession{
		sessionID: "sess-uninitialized",
		clients:   map[string]*daggerClient{"c": {clientID: "c"}},
	}
	srv.daggerSessions[pendingSess.sessionID] = pendingSess

	states := srv.ClientWorkspaceDebugStates()
	require.Len(t, states, 3)

	// Sorted by session then client ID.
	require.Equal(t, "client-errored", states[0].ClientID)
	require.Equal(t, "client-git", states[1].ClientID)
	require.Equal(t, "client-rootless", states[2].ClientID)

	errored := states[0]
	require.True(t, errored.WorkspaceLoaded)
	require.Equal(t, "workspace detection: boom", errored.WorkspaceError)
	require.Nil(t, errored.Workspace)

	git := states[1]
	require.Equal(t, "sess-1", git.SessionID)
	require.NotNil(t, git.Workspace)
	require.Equal(t, "*core.WorkspaceSourceClientLocal", git.Workspace.SourceKind)
	require.Equal(t, "*core.WorkspaceSourceClientLocal", git.Workspace.BaseSourceKind)
	require.Equal(t, "/srv/repo", git.Workspace.HostPath)
	require.Equal(t, "app", git.Workspace.Cwd)
	require.Equal(t, "dagger.toml", git.Workspace.ConfigFile)
	require.Equal(t, "dagger.lock", git.Workspace.LockFile)
	require.Equal(t, "dev", git.Workspace.SelectedEnv)
	require.Equal(t, 2, git.PendingModuleCount)
	require.Equal(t, []string{"pending-one", "./modules/pending-two"}, git.PendingModules)
	require.Equal(t, 1, git.ServedWorkspaceModuleCount)
	require.Equal(t, []string{"served-one"}, git.ServedWorkspaceModules)
	require.Equal(t, 1, git.FailedModuleCount)
	require.Equal(t, map[string]string{"broken-one": "compile error"}, git.FailedModules)

	rootless := states[2]
	require.True(t, rootless.WorkspaceLoaded)
	require.Empty(t, rootless.WorkspaceError)
	require.NotNil(t, rootless.Workspace)
	require.Equal(t, "*core.WorkspaceSourceRootlessLocal", rootless.Workspace.SourceKind)
	require.Equal(t, "*core.WorkspaceSourceRootlessLocal", rootless.Workspace.BaseSourceKind)
	require.Empty(t, rootless.Workspace.ConfigFile)
	require.Zero(t, rootless.PendingModuleCount)
	require.Zero(t, rootless.ServedWorkspaceModuleCount)
}
