package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"dagger.io/dagger"
	"github.com/dagger/dagger/engine/client"
	"github.com/moby/buildkit/identity"
	"github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"
	"github.com/vito/progrock"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
)

var serveCmd = &cobra.Command{
	Use:                   "serve [CHAIN]",
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return loadModCmdWrapper(Serve, "", true)(cmd, args)
	},
	Args: cobra.MinimumNArgs(1),
}

func Serve(ctx context.Context, engineClient *client.Client, _ *dagger.Module, _ *cobra.Command, args []string) (rerr error) {
	eg := new(errgroup.Group)
	for _, chain := range args {
		eg.Go(func() error { return serve(ctx, chain, engineClient) })
	}
	return eg.Wait()
}

func serve(ctx context.Context, chain string, engineClient *client.Client) (rerr error) {
	selectors := strings.Split(chain, ".")

	query := ``
	for _, sel := range selectors {
		query += "{"
		query += sel
	}
	query += "{id}"
	for range selectors {
		query += "}"
	}

	rec := progrock.FromContext(ctx)

	vtx := rec.Vertex(
		digest.FromString(identity.NewID()),
		fmt.Sprintf("serve %s", chain),
		progrock.Focused(),
	)
	defer func() { vtx.Done(rerr) }()

	vars := make(map[string]interface{})
	if len(queryVarsJSONInput) > 0 {
		if err := json.Unmarshal([]byte(queryVarsJSONInput), &vars); err != nil {
			return err
		}
	} else {
		vars = getKVInput(queryVarsInput)
	}

	res := make(map[string]any)
	err := engineClient.Do(ctx, query, "", vars, &res)
	if err != nil {
		return err
	}

	for _, sel := range selectors {
		res = res[sel].(map[string]any)
	}

	svcID := dagger.ServiceID(res["id"].(string))

	dag := engineClient.Dagger()

	svc := dag.Service(svcID)

	tunnel, err := dag.Host().Tunnel(svc, dagger.HostTunnelOpts{
		Native: true,
	}).Start(ctx)
	if err != nil {
		return err
	}

	var out io.Writer
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		out = os.Stdout
	} else {
		out = vtx.Stdout()
	}

	fmt.Fprintf(out, "tunneled!\n")

	ports, err := tunnel.Ports(ctx)
	if err != nil {
		return err
	}

	for _, port := range ports {
		num, err := port.Port(ctx)
		if err != nil {
			return err
		}
		proto, err := port.Protocol(ctx)
		if err != nil {
			return err
		}
		desc, err := port.Description(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%d/%s", num, proto)
		if desc != "" {
			fmt.Fprintf(out, " (%s)", desc)
		}
		fmt.Fprintln(out)
	}

	<-ctx.Done()

	return ctx.Err()
}
