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
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	queryFile          string
	queryVarsInput     []string
	queryVarsJSONInput string
)

var queryCmd = &cobra.Command{
	Use:                   "query [flags] [operation]",
	Aliases:               []string{"q"},
	DisableFlagsInUseLine: true,
	Long:                  "Send API queries to a dagger engine\n\nWhen no document file, read query from standard input.",
	Short:                 "Send API queries to a dagger engine",
	Example: `
dagger query <<EOF
{
  container {
    from(address:"hello-world") {
      withExec(args:["/hello"]) {
        stdout
      }
    }
  }
}
EOF
`,
	RunE: loadModCmdWrapper(Query, "", true),
	Args: cobra.MaximumNArgs(1), // operation can be specified
}

func Query(ctx context.Context, engineClient *client.Client, _ *dagger.Module, _ *cobra.Command, args []string) error {
	var operation string
	if len(args) > 0 {
		operation = args[0]
	}

	vars := make(map[string]interface{})
	if len(queryVarsJSONInput) > 0 {
		if err := json.Unmarshal([]byte(queryVarsJSONInput), &vars); err != nil {
			return err
		}
	} else {
		vars = getKVInput(queryVarsInput)
	}

	// Use the provided query file if specified
	// Otherwise, if stdin is a pipe or other non-tty thing, read from it.
	// Finally, default to the operations returned by the loadExtension query
	var operations string
	if queryFile != "" {
		inBytes, err := os.ReadFile(queryFile)
		if err != nil {
			return err
		}
		operations = string(inBytes)
	} else if !term.IsTerminal(int(os.Stdin.Fd())) {
		inBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		operations = string(inBytes)
	}

	res := make(map[string]interface{})
	err := engineClient.Do(ctx, operations, operation, vars, &res)
	if err != nil {
		return err
	}
	result, err := json.MarshalIndent(res, "", "    ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", result)
	return nil
}

func getKVInput(kvs []string) map[string]interface{} {
	m := make(map[string]interface{})
	for _, kv := range kvs {
		split := strings.SplitN(kv, "=", 2)
		m[split[0]] = split[1]
	}
	return m
}

func init() {
	// changing usage template so examples are displayed below flags
	queryCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)

	queryCmd.Flags().StringVar(&queryFile, "doc", "", "document query file")
	queryCmd.Flags().StringSliceVar(&queryVarsInput, "var", nil, "query variable")
	queryCmd.Flags().StringVar(&queryVarsJSONInput, "var-json", "", "json query variables (overrides --var)")
}
