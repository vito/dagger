package workspace

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectInitializedWorkspace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existing := map[string]struct{}{
		"/repo/app/dagger.toml": {},
		"/repo/.git":            {},
	}

	ws, err := Detect(ctx, fakePathExists(existing), "/repo/app")
	require.NoError(t, err)
	require.Equal(t, "/repo", ws.Root)
	require.True(t, ws.HasGitRoot)
	require.Equal(t, "app", ws.Cwd)
	require.Equal(t, "app/dagger.toml", ws.ConfigFile)
	require.Equal(t, "app/dagger.lock", ws.LockFile)
}

func TestDetectInitializedWorkspaceFromNestedCwd(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existing := map[string]struct{}{
		"/repo/dagger.toml": {},
		"/repo/.git":        {},
	}

	ws, err := Detect(ctx, fakePathExists(existing), "/repo/app/sub")
	require.NoError(t, err)
	require.Equal(t, "/repo", ws.Root)
	require.True(t, ws.HasGitRoot)
	require.Equal(t, "app/sub", ws.Cwd)
	require.Equal(t, "dagger.toml", ws.ConfigFile)
	require.Equal(t, "dagger.lock", ws.LockFile)
}

func TestDetectMissingConfigDoesNotChangeBoundary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existing := map[string]struct{}{
		"/repo/app/.dagger":      {},
		"/repo/app/.dagger/lock": {},
		"/repo/.git":             {},
	}

	ws, err := Detect(ctx, fakePathExists(existing), "/repo/app/sub")
	require.NoError(t, err)
	require.Equal(t, "/repo", ws.Root)
	require.True(t, ws.HasGitRoot)
	require.Equal(t, "app/sub", ws.Cwd)
	require.Empty(t, ws.ConfigFile)
	require.Equal(t, "app/dagger.lock", ws.LockFile)
}

func TestDetectUsesExistingLockFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existing := map[string]struct{}{
		"/repo/dagger.lock": {},
		"/repo/.git":        {},
	}

	ws, err := Detect(ctx, fakePathExists(existing), "/repo/app/sub")
	require.NoError(t, err)
	require.Equal(t, "/repo", ws.Root)
	require.True(t, ws.HasGitRoot)
	require.Equal(t, "app/sub", ws.Cwd)
	require.Empty(t, ws.ConfigFile)
	require.Equal(t, "dagger.lock", ws.LockFile)
}

func TestDetectMapsExistingLegacyLockFileToCanonicalLockFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existing := map[string]struct{}{
		"/repo/app/.dagger/lock": {},
		"/repo/.git":             {},
	}

	ws, err := Detect(ctx, fakePathExists(existing), "/repo/app/sub")
	require.NoError(t, err)
	require.Equal(t, "/repo", ws.Root)
	require.True(t, ws.HasGitRoot)
	require.Equal(t, "app/sub", ws.Cwd)
	require.Empty(t, ws.ConfigFile)
	require.Equal(t, "app/dagger.lock", ws.LockFile)
}

func TestDetectReturnsNilWithoutGit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existing := map[string]struct{}{}

	ws, err := Detect(ctx, fakePathExists(existing), "/repo/app")
	require.NoError(t, err)
	require.Nil(t, ws)
}

func TestFindConfigUpwardFindsIgnoredConfig(t *testing.T) {
	t.Parallel()

	// The exact case Detect silently ignores: a dagger.toml above cwd with no
	// .git anywhere. Detect returns nil; FindConfigUpward still names the
	// config so callers can warn about it.
	ctx := context.Background()
	existing := map[string]struct{}{
		"/repo/dagger.toml": {},
	}

	ws, err := Detect(ctx, fakePathExists(existing), "/repo/app")
	require.NoError(t, err)
	require.Nil(t, ws)

	configPath, found, err := FindConfigUpward(ctx, fakePathExists(existing), "/repo/app")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "/repo/dagger.toml", configPath)
}

func TestFindConfigUpwardPrefersNearestConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existing := map[string]struct{}{
		"/repo/dagger.toml":     {},
		"/repo/app/dagger.toml": {},
	}

	configPath, found, err := FindConfigUpward(ctx, fakePathExists(existing), "/repo/app/sub")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "/repo/app/dagger.toml", configPath)
}

func TestFindConfigUpwardReportsNoConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existing := map[string]struct{}{}

	configPath, found, err := FindConfigUpward(ctx, fakePathExists(existing), "/repo/app")
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, configPath)
}

func TestDetectInRootDoesNotClaimGitRoot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existing := map[string]struct{}{
		"/workspace/dagger.toml": {},
	}

	ws, err := DetectInRoot(ctx, fakePathExists(existing), "/workspace/app", "/workspace")
	require.NoError(t, err)
	require.Equal(t, "/workspace", ws.Root)
	require.False(t, ws.HasGitRoot)
	require.Equal(t, "app", ws.Cwd)
	require.Equal(t, "dagger.toml", ws.ConfigFile)
	require.Equal(t, "dagger.lock", ws.LockFile)
}

func fakePathExists(existing map[string]struct{}) PathExistsFunc {
	return func(_ context.Context, path string) (string, bool, error) {
		cleanPath := filepath.Clean(path)
		if _, ok := existing[cleanPath]; ok {
			return filepath.Dir(cleanPath), true, nil
		}
		return "", false, nil
	}
}
