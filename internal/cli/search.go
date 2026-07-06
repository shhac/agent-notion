package cli

import (
	"github.com/shhac/agent-notion/internal/errors"
	"github.com/shhac/agent-notion/internal/notion"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// registerSearch wires the `search` command group. It is the template for
// the other data groups: withBackend + errors.Classify around the backend
// call, printPaginated/emitItem for output.
func registerSearch(root *cobra.Command, g *GlobalFlags) {
	search := &cobra.Command{
		Use:   "search",
		Short: "Search Notion by title (pages and databases)",
	}
	search.AddCommand(searchQueryCmd(g))
	root.AddCommand(search)
}

func searchQueryCmd(g *GlobalFlags) *cobra.Command {
	var (
		filter string
		limit  int
		cursor string
	)
	cmd := &cobra.Command{
		Use:   "query <query>",
		Short: "Search Notion by title (pages and databases)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if filter != "" && filter != "page" && filter != "database" {
				return output.New("invalid --filter: expected 'page' or 'database'", output.FixableByAgent)
			}

			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.Paginated[notion.SearchResult], error) {
				return b.Search(ctx, notion.SearchParams{
					Query:  args[0],
					Filter: filter,
					Limit:  g.pageSize(limit),
					Cursor: cursor,
				})
			})
			if err != nil {
				return errors.Classify(err)
			}
			return printPaginated(g, result)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "Filter by type: page | database")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results (default: page_size setting, else 50)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous page")
	_ = cmd.RegisterFlagCompletionFunc("filter", fixedCompletions("page", "database"))
	return cmd
}
