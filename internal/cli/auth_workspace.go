package cli

import (
	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	libcli "github.com/shhac/lib-agent-cli/cli"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func authWorkspaceCmd() *cobra.Command {
	workspace := &cobra.Command{
		Use:   "workspace",
		Short: "Manage workspace profiles",
	}
	workspace.AddCommand(
		workspaceListCmd(),
		workspaceSwitchCmd("switch", "Switch the active (default) workspace"),
		workspaceSwitchCmd("set-default", "Set the default workspace (alias for switch)"),
		workspaceRemoveCmd(),
	)
	return workspace
}

func workspaceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured workspaces",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.Read()
			w := output.NewNDJSONWriter(cmd.OutOrStdout())
			for _, alias := range sortedAliases(cfg) {
				ws := cfg.Workspaces[alias]
				if err := w.WriteItem(map[string]any{
					"alias":     alias,
					"name":      ws.WorkspaceName,
					"auth_type": string(ws.AuthType),
					"default":   alias == cfg.DefaultWorkspace,
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func workspaceSwitchCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:               use + " <alias>",
		Short:             short,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeWorkspaceAliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := credential.SetDefaultWorkspace(args[0]); err != nil {
				return output.Wrap(err, output.FixableByAgent).
					WithHint("run 'agent-notion auth workspace list' to see configured workspaces")
			}
			return output.NewNDJSONWriter(cmd.OutOrStdout()).WriteItem(map[string]any{
				"ok": true, "default_workspace": args[0],
			})
		},
	}
}

func workspaceRemoveCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:               "remove <alias>",
		Short:             "Remove a workspace and its stored credentials",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeWorkspaceAliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]
			if err := libcli.RequireConfirm(yes,
				"this removes the stored credentials for workspace '"+alias+"'"); err != nil {
				return err
			}

			wasDefault := alias == config.Read().DefaultWorkspace
			if err := credential.RemoveWorkspace(alias, credential.DefaultKeychainStore()); err != nil {
				return output.Wrap(err, output.FixableByAgent).
					WithHint("run 'agent-notion auth workspace list' to see configured workspaces")
			}

			after := config.Read()
			item := map[string]any{
				"ok":      true,
				"removed": alias,
			}
			if after.DefaultWorkspace != "" {
				item["default_workspace"] = after.DefaultWorkspace
			}
			if wasDefault {
				item["warning"] = "removed the default workspace"
			}
			return output.NewNDJSONWriter(cmd.OutOrStdout()).WriteItem(item)
		},
	}
	libcli.AddConfirmFlag(cmd, &yes)
	return cmd
}

func completeWorkspaceAliases(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return sortedAliases(config.Read()), cobra.ShellCompDirectiveNoFileComp
}
