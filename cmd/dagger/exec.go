package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"github.com/Khan/genqlient/graphql"
	"github.com/adrg/xdg"
	"github.com/containerd/containerd/platforms"
	"github.com/dagger/dagger/dagql/idproto"
	"github.com/dagger/dagger/engine/client"
	"github.com/google/uuid"
	"github.com/iancoleman/strcase"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:     "exec [flags] COMMAND",
	Aliases: []string{"e"},
	Short:   "Run an executable built by a Dagger module",
	Example: strings.TrimSpace(`
dagger exec github.com/dagger/dagger
dagger exec github.com/vito/bass
`,
	),
	GroupID:      execGroup.ID,
	Run:          Exec,
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
}

func init() {
	// don't require -- to disambiguate subcommand flags
	execCmd.Flags().SetInterspersed(false)
}

func Exec(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	err := doExec(ctx, args)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "run canceled")
			os.Exit(2)
			return
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
		return
	}
}

func doExec(ctx context.Context, args []string) error {
	ref := args[0]

	err := withEngineAndTUI(ctx, client.Params{}, func(ctx context.Context, engineClient *client.Client) error {
		dag := engineClient.Dagger()

		modSrc := dag.ModuleSource(ref)

		srcKind, err := modSrc.Kind(ctx)
		if err != nil {
			return err
		}

		if srcKind == dagger.LocalSource {
			abs, err := filepath.Abs(ref)
			if err != nil {
				return err
			}
			modSrc = dag.ModuleSource(".").WithContextDirectory(dag.Host().Directory(abs))
		}

		mod := modSrc.AsModule().Initialize()

		if _, err := mod.Serve(ctx); err != nil {
			return err
		}

		name, err := mod.Name(ctx)
		if err != nil {
			return err
		}

		srcID, err := modSrc.ContextDirectory().ID(ctx)
		if err != nil {
			return err
		}

		var idp idproto.ID
		if err := (&idp).Decode(string(srcID)); err != nil {
			return err
		}
		dig, err := idp.Digest()
		if err != nil {
			return err
		}

		bin := filepath.Join(xdg.DataHome, "dagger", "bin", dig.Encoded())
		if _, err := os.Stat(bin); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			if err := modSrc.Client.MakeRequest(ctx,
				&graphql.Request{
					Query: fmt.Sprintf(
						`query($src: DirectoryID!, $platform: String!, $path: String!) {
						%s(src: $src) {
							executable(platform: $platform) {
								export(path: $path)
							}
						}
					}`,
						strcase.ToLowerCamel(name),
					),
					Variables: map[string]interface{}{
						"src":      srcID,
						"platform": platforms.Format(platforms.DefaultSpec()),
						"path":     bin,
					},
				},
				&graphql.Response{},
			); err != nil {
				return err
			}
		}

		args[0] = bin // sneaky

		return nil
	})
	if err != nil {
		return err
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("generate uuid: %w", err)
	}
	sessionToken := u.String()

	sess, ctx, err := client.Connect(ctx, client.Params{
		SecretToken: sessionToken,
	})
	if err != nil {
		return err
	}
	defer sess.Close()

	// TODO whee
	silent = true

	return execWithSession(ctx, args, sessionToken, sess)
}
