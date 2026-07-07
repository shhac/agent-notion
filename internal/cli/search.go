package cli

import (
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
	addDomainUsage("search", searchUsageText)
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
				return err
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

const searchUsageText = `agent-notion search — Search Notion by title (NOT full-text content search)

USAGE
  search query <query> [--filter page|database] [--limit <n>] [--cursor <cursor>]

IMPORTANT: Search only matches page and database TITLES. It does NOT search
page content.
  To search within content: use "database query <id>" with property filters.
  To find text in page bodies: use "block list <page-id>" and search the output.

OUTPUT
  One NDJSON record per hit: {id, type, title, url, parent?, last_edited_at?}
  type: "page" or "database"; parent: {type: database|page|workspace, id?}
  A trailing {"@pagination": {has_more, next_cursor}} line when more remain;
  pass next_cursor back via --cursor. --format json|yaml wraps everything in
  one {data: […]} envelope instead.

EXAMPLES
  search query "Project Roadmap"              Search all titles
  search query "Task" --filter database       Only databases
  search query "Meeting Notes" --filter page  Only pages
  search query "Q1" --limit 5                 Limit results`
