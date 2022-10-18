package core

import (
	"os"
	"path"
	"testing"

	"dagger.io/dagger"
	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestHostWorkdir(t *testing.T) {
	t.Parallel()

	var secretRes struct {
		Host struct {
			Workdir struct {
				ID core.DirectoryID
			}
		}
	}

	dir := t.TempDir()
	err := os.WriteFile(path.Join(dir, "foo"), []byte("bar"), 0600)
	require.NoError(t, err)

	err = testutil.Query(
		`{
			host {
				workdir {
					id
				}
			}
		}`, &secretRes, nil, dagger.WithWorkdir(dir))
	require.NoError(t, err)

	hostRes := secretRes.Host.Workdir.ID
	require.NotEmpty(t, hostRes)

	var execRes struct {
		Container struct {
			From struct {
				WithMountedDirectory struct {
					Exec struct {
						Stdout struct{ Contents string }
					}
				}
			}
		}
	}

	err = testutil.Query(
		`query Test($host: DirectoryID!) {
			container {
				from(address: "alpine:3.16.2") {
					withMountedDirectory(path: "/host", source: $host) {
						exec(args: ["ls", "/host"]) {
							stdout { contents }
						}
					}
				}
			}
		}`, &execRes, &testutil.QueryOptions{
			Variables: map[string]interface{}{
				"host": hostRes,
			},
		})
	require.NoError(t, err)

	require.Equal(t, "foo\n", execRes.Container.From.WithMountedDirectory.Exec.Stdout.Contents)
}

func TestHostLocalDirReadWrite(t *testing.T) {
	t.Parallel()

	dir1 := t.TempDir()
	err := os.WriteFile(path.Join(dir1, "foo"), []byte("bar"), 0600)
	require.NoError(t, err)

	dir2 := t.TempDir()

	var readRes struct {
		Host struct {
			Directory struct {
				ID core.DirectoryID
			}
		}
	}

	err = testutil.Query(
		`query Test($dir: String!) {
			host {
				directory(path: $dir) {
					id
				}
			}
		}`, &readRes, &testutil.QueryOptions{
			Variables: map[string]interface{}{
				"dir": dir1,
			},
		})
	require.NoError(t, err)

	srcID := readRes.Host.Directory.ID

	var writeRes struct {
		Directory struct {
			Export bool
		}
	}

	err = testutil.Query(
		`query Test($src: DirectoryID!, $dir2: String!) {
			directory(id: $src) {
				export(path: $dir2)
			}
		}`, &writeRes, &testutil.QueryOptions{
			Variables: map[string]any{
				"src":  srcID,
				"dir2": dir2,
			},
		},
	)
	require.NoError(t, err)

	require.True(t, writeRes.Directory.Export)

	content, err := os.ReadFile(path.Join(dir2, "foo"))
	require.NoError(t, err)
	require.Equal(t, "bar", string(content))
}

func TestHostLocalDirWrite(t *testing.T) {
	t.Parallel()

	dir1 := t.TempDir()

	var contentRes struct {
		Directory struct {
			WithNewFile struct {
				ID core.DirectoryID
			}
		}
	}

	err := testutil.Query(
		`{
			directory {
				withNewFile(path: "foo", contents: "bar") {
					id
				}
			}
		}`, &contentRes, nil)
	require.NoError(t, err)

	srcID := contentRes.Directory.WithNewFile.ID

	var writeRes struct {
		Directory struct {
			Export bool
		}
	}

	err = testutil.Query(
		`query Test($src: DirectoryID!, $dir1: String!) {
			directory(id: $src) {
				export(path: $dir1)
			}
		}`, &writeRes, &testutil.QueryOptions{
			Variables: map[string]any{
				"src":  srcID,
				"dir1": dir1,
			},
		},
	)
	require.NoError(t, err)

	require.True(t, writeRes.Directory.Export)

	content, err := os.ReadFile(path.Join(dir1, "foo"))
	require.NoError(t, err)
	require.Equal(t, "bar", string(content))
}

func TestHostVariable(t *testing.T) {
	t.Parallel()

	var secretRes struct {
		Host struct {
			Variable struct {
				Value  string
				Secret struct {
					ID core.SecretID
				}
			}
		}
	}

	require.NoError(t, os.Setenv("HELLO_TEST", "hello"))

	err := testutil.Query(
		`{
			host {
				variable(name: "HELLO_TEST") {
					value
					secret {
						id
					}
				}
			}
		}`, &secretRes, nil)
	require.NoError(t, err)

	varValue := secretRes.Host.Variable.Value
	require.Equal(t, "hello", varValue)

	varSecret := secretRes.Host.Variable.Secret.ID

	var execRes struct {
		Container struct {
			From struct {
				WithSecretVariable struct {
					Exec struct {
						Stdout struct{ Contents string }
					}
				}
			}
		}
	}

	err = testutil.Query(
		`query Test($secret: SecretID!) {
			container {
				from(address: "alpine:3.16.2") {
					withSecretVariable(name: "SECRET", secret: $secret) {
						exec(args: ["env"]) {
							stdout { contents }
						}
					}
				}
			}
		}`, &execRes, &testutil.QueryOptions{
			Variables: map[string]interface{}{
				"secret": varSecret,
			},
		})
	require.NoError(t, err)

	require.Contains(t, execRes.Container.From.WithSecretVariable.Exec.Stdout.Contents, "SECRET=hello")
}
