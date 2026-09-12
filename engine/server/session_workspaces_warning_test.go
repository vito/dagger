package server

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dagger/dagger/core/workspace"
)

func fakeWorkspacePathExists(existing map[string]struct{}) workspace.PathExistsFunc {
	return func(_ context.Context, path string) (string, bool, error) {
		cleanPath := filepath.Clean(path)
		if _, ok := existing[cleanPath]; ok {
			return filepath.Dir(cleanPath), true, nil
		}
		return "", false, nil
	}
}

func TestIgnoredWorkspaceConfigWarningNamesTheIgnoredConfig(t *testing.T) {
	t.Parallel()

	// A dagger.toml above cwd with no .git anywhere: detection yields no
	// workspace, and the rootless fallback must say why instead of silently
	// composing an empty session.
	ctx := context.Background()
	pathExists := fakeWorkspacePathExists(map[string]struct{}{
		"/home/user/project/dagger.toml": {},
	})

	msg, ok := ignoredWorkspaceConfigWarning(ctx, pathExists, "/home/user/project/app", true)
	require.True(t, ok)
	require.Equal(t, "dagger.toml found at /home/user/project/dagger.toml, but it is not inside a git repository; no workspace loaded (workspace detection requires a git root)", msg)
}

func TestIgnoredWorkspaceConfigWarningSilentWithoutConfig(t *testing.T) {
	t.Parallel()

	// No dagger.toml anywhere: a rootless session is the ordinary no-workspace
	// case and deserves no warning.
	ctx := context.Background()
	pathExists := fakeWorkspacePathExists(map[string]struct{}{})

	_, ok := ignoredWorkspaceConfigWarning(ctx, pathExists, "/home/user/project", true)
	require.False(t, ok)
}

func TestIgnoredWorkspaceConfigWarningSilentForNonLocal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pathExists := fakeWorkspacePathExists(map[string]struct{}{
		"/dagger.toml": {},
	})

	_, ok := ignoredWorkspaceConfigWarning(ctx, pathExists, "/", false)
	require.False(t, ok)
}

func TestIgnoredWorkspaceConfigWarningSilentOnLookupError(t *testing.T) {
	t.Parallel()

	// The warning is best-effort diagnostics; a failing stat must not turn
	// into output (let alone an error) on the rootless path.
	ctx := context.Background()
	pathExists := func(context.Context, string) (string, bool, error) {
		return "", false, errors.New("stat exploded")
	}

	_, ok := ignoredWorkspaceConfigWarning(ctx, pathExists, "/home/user/project", true)
	require.False(t, ok)
}
