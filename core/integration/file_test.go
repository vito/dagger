package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"dagger.io/dagger"
	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestFile(t *testing.T) {
	t.Parallel()

	var res struct {
		Directory struct {
			WithNewFile struct {
				File struct {
					ID       core.FileID
					Contents string
				}
			}
		}
	}

	err := testutil.Query(
		`{
			directory {
				withNewFile(path: "some-file", contents: "some-content") {
					file(path: "some-file") {
						id
						contents
					}
				}
			}
		}`, &res, nil)
	require.NoError(t, err)
	require.NotEmpty(t, res.Directory.WithNewFile.File.ID)
	require.Equal(t, "some-content", res.Directory.WithNewFile.File.Contents)
}

func TestDirectoryFile(t *testing.T) {
	t.Parallel()

	var res struct {
		Directory struct {
			WithNewFile struct {
				Directory struct {
					File struct {
						ID       core.FileID
						Contents string
					}
				}
			}
		}
	}

	err := testutil.Query(
		`{
			directory {
				withNewFile(path: "some-dir/some-file", contents: "some-content") {
					directory(path: "some-dir") {
						file(path: "some-file") {
							id
							contents
						}
					}
				}
			}
		}`, &res, nil)
	require.NoError(t, err)
	require.NotEmpty(t, res.Directory.WithNewFile.Directory.File.ID)
	require.Equal(t, "some-content", res.Directory.WithNewFile.Directory.File.Contents)
}

func TestFileSize(t *testing.T) {
	t.Parallel()

	var res struct {
		Directory struct {
			WithNewFile struct {
				File struct {
					ID   core.FileID
					Size int
				}
			}
		}
	}

	err := testutil.Query(
		`{
			directory {
				withNewFile(path: "some-file", contents: "some-content") {
					file(path: "some-file") {
						id
						size
					}
				}
			}
		}`, &res, nil)
	require.NoError(t, err)
	require.NotEmpty(t, res.Directory.WithNewFile.File.ID)
	require.Equal(t, len("some-content"), res.Directory.WithNewFile.File.Size)
}

func TestFileExport(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	wd := t.TempDir()
	dest := t.TempDir()

	c, err := dagger.Connect(ctx, dagger.WithWorkdir(wd))
	require.NoError(t, err)
	defer c.Close()

	file := c.Container().From("alpine:3.16.2").File("/etc/alpine-release")

	t.Run("to absolute dir", func(t *testing.T) {
		ok, err := file.Export(ctx, dest)
		require.NoError(t, err)
		require.True(t, ok)

		contents, err := os.ReadFile(filepath.Join(dest, "alpine-release"))
		require.NoError(t, err)
		require.Equal(t, "3.16.2\n", string(contents))

		entries, err := ls(dest)
		require.NoError(t, err)
		require.Len(t, entries, 1)
	})

	t.Run("to workdir", func(t *testing.T) {
		ok, err := file.Export(ctx, ".")
		require.NoError(t, err)
		require.True(t, ok)

		contents, err := os.ReadFile(filepath.Join(wd, "alpine-release"))
		require.NoError(t, err)
		require.Equal(t, "3.16.2\n", string(contents))

		entries, err := ls(wd)
		require.NoError(t, err)
		require.Len(t, entries, 1)
	})

	t.Run("to outer dir", func(t *testing.T) {
		ok, err := file.Export(ctx, "../")
		require.Error(t, err)
		require.False(t, ok)
	})
}
