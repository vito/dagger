package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"dagger.io/dagger"
	"github.com/dagger/dagger/engine/client"
	"github.com/moby/buildkit/identity"
	"github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"
	"github.com/vito/progrock"
	"golang.org/x/term"
)

var (
	serveFocus         bool
	serveForwards      []string
	serveVarsInput     []string
	serveVarsJSONInput string
)

var serveCmd = &cobra.Command{
	Use:                   "serve [CHAIN]",
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		focus = serveFocus
		return loadModCmdWrapper(Serve, "", true)(cmd, args)
	},
	Args: cobra.MinimumNArgs(1),
}

func init() {
	serveCmd.Flags().BoolVar(&serveFocus, "focus", false, "Only show output for focused commands.")
	serveCmd.Flags().StringSliceVarP(&serveForwards, "port", "p", nil, "Forward a port from the service to the host.")
	serveCmd.Flags().StringSliceVar(&serveVarsInput, "var", nil, "query variable")
	serveCmd.Flags().StringVar(&serveVarsJSONInput, "var-json", "", "json query variables (overrides --var)")
}

func Serve(ctx context.Context, engineClient *client.Client, _ *dagger.Module, _ *cobra.Command, args []string) (rerr error) {
	chain := args[0]
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
	if len(serveVarsJSONInput) > 0 {
		if err := json.Unmarshal([]byte(serveVarsJSONInput), &vars); err != nil {
			return err
		}
	} else {
		vars = getKVInput(serveVarsInput)
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

	opts := dagger.HostTunnelOpts{
		Native: len(serveForwards) == 0,
	}
	for _, f := range serveForwards {
		ports, proto, ok := strings.Cut(f, "/")
		if !ok {
			proto = string(dagger.Tcp)
		}
		backStr, frontStr, ok := strings.Cut(ports, ":")
		if !ok {
			frontStr = backStr
		}
		back, err := strconv.Atoi(backStr)
		if err != nil {
			return err
		}
		front, err := strconv.Atoi(frontStr)
		if err != nil {
			return err
		}
		opts.Ports = append(opts.Ports, &dagger.PortForward{
			Backend:  back,
			Frontend: front,
			Protocol: dagger.NetworkProtocol(proto),
		})
	}

	tunnel, err := dag.Host().Tunnel(svc, opts).Start(ctx)
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
