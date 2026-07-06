package cli

import (
	"github.com/shhac/agent-notion/internal/errors"
	"github.com/shhac/agent-notion/internal/notion"
	"github.com/spf13/cobra"
)

// registerUser wires the `user` command group (list, me) plus its usage card.
func registerUser(root *cobra.Command, g *GlobalFlags) {
	user := &cobra.Command{
		Use:   "user",
		Short: "User operations",
	}
	user.AddCommand(userListCmd(g), userMeCmd(g))
	addDomainUsage("user", userUsageText)
	root.AddCommand(user)
}

func userListCmd(g *GlobalFlags) *cobra.Command {
	var (
		limit  int
		cursor string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all users in the workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.Paginated[notion.UserItem], error) {
				return b.ListUsers(ctx, notion.ListParams{Limit: g.pageSize(limit), Cursor: cursor})
			})
			if err != nil {
				return errors.Classify(err)
			}
			return printPaginated(g, result)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results (default: page_size setting, else 50)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous page")
	return cmd
}

func userMeCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Get the bot user (integration) identity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			me, err := withBackend(ctx, g, func(b notion.Backend) (notion.UserMe, error) {
				return b.GetMe(ctx)
			})
			if err != nil {
				return errors.Classify(err)
			}
			return emitItem(g, me)
		},
	}
}

const userUsageText = `agent-notion user — Workspace user information

SUBCOMMANDS:
  user list [--limit <n>] [--cursor <cursor>]    List all users in the workspace
  user me                                        Get the bot user (integration) identity

LIST OUTPUT:
  { "items": [{ id, name, type, email?, avatar_url? }], "pagination"?: ... }

  type: "person" (human user) or "bot" (integration)
  email: Only available for person users (not bots)

ME OUTPUT:
  { id, name, type, workspace_name }

EXAMPLES:
  user list                          List all workspace users
  user list --limit 10               First 10 users
  user me                            Current bot identity`
