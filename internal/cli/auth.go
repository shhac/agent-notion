package cli

import (
	"os"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	libcli "github.com/shhac/lib-agent-cli/cli"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// registerAuth wires the `auth` command group. globals is threaded through for
// consistency with the family pattern; later auth subcommands will use it.
func registerAuth(root *cobra.Command, _ *libcli.Globals) {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Notion authentication",
	}
	authCmd.AddCommand(authStatusCmd())
	root.AddCommand(authCmd)
}

func authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the resolved Notion credential (never prints the token)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			res, ok := credential.Resolve(config.Read(), credential.DefaultKeychain())
			if !ok {
				return output.New("no Notion credential configured", output.FixableByHuman).
					WithHint("run 'agent-notion auth login', import a desktop token, or set NOTION_TOKEN")
			}

			item := map[string]any{
				"authenticated": true,
				"source":        string(res.Source),
			}
			if res.Workspace != "" {
				item["workspace"] = res.Workspace
			}
			if res.AuthType != "" {
				item["auth_type"] = string(res.AuthType)
			}
			return output.NewNDJSONWriter(os.Stdout).WriteItem(item)
		},
	}
}
