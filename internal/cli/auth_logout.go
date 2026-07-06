package cli

import (
	"fmt"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	libcli "github.com/shhac/lib-agent-cli/cli"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func authLogoutCmd(g *GlobalFlags) *cobra.Command {
	var (
		all       bool
		workspace string
		yes       bool
	)
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials for a workspace (or --all)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if all {
				if err := libcli.RequireConfirm(yes,
					"this removes ALL workspaces, the OAuth client config, and every keychain entry"); err != nil {
					return err
				}
				if err := credential.ClearAll(g.keychain()); err != nil {
					return output.Wrap(err, output.FixableByHuman)
				}
				return emitItem(g, map[string]any{"ok": true, "cleared": "all"})
			}

			cfg := config.Read()
			target := workspace
			if target == "" {
				target = cfg.DefaultWorkspace
			}
			if target == "" {
				return output.New("no workspaces configured; nothing to log out from", output.FixableByHuman).
					WithHint("run 'agent-notion auth login' or 'agent-notion auth import' to add one")
			}

			if err := libcli.RequireConfirm(yes,
				fmt.Sprintf("this removes the stored credentials for workspace '%s'", target)); err != nil {
				return err
			}

			wasDefault := target == cfg.DefaultWorkspace
			if err := credential.RemoveWorkspace(target, g.keychain()); err != nil {
				return output.Wrap(err, output.FixableByAgent).
					WithHint("run 'agent-notion auth workspace list' to see configured workspaces")
			}

			after := config.Read()
			item := map[string]any{
				"ok":                   true,
				"removed":              target,
				"remaining_workspaces": credential.WorkspaceAliases(after.Workspaces),
			}
			if after.DefaultWorkspace != "" {
				item["default_workspace"] = after.DefaultWorkspace
			}
			if wasDefault {
				item["warning"] = "removed the default workspace"
			}
			return emitItem(g, item)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Remove all workspaces, OAuth config, and keychain entries")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace alias to remove (default: the default workspace)")
	libcli.AddConfirmFlag(cmd, &yes)
	return cmd
}
