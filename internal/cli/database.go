package cli

import (
	"encoding/json"

	"github.com/shhac/agent-notion/internal/errors"
	"github.com/shhac/agent-notion/internal/ids"
	"github.com/shhac/agent-notion/internal/notion"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// registerDatabase wires the `database` command group (list, get, query,
// schema) plus its usage card.
func registerDatabase(root *cobra.Command, g *GlobalFlags) {
	database := &cobra.Command{
		Use:   "database",
		Short: "Database operations",
	}
	database.AddCommand(
		databaseListCmd(g),
		databaseGetCmd(g),
		databaseQueryCmd(g),
		databaseSchemaCmd(g),
	)
	addDomainUsage("database", databaseUsageText)
	root.AddCommand(database)
}

func databaseListCmd(g *GlobalFlags) *cobra.Command {
	var (
		limit  int
		cursor string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all databases the integration can access",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.Paginated[notion.DatabaseListItem], error) {
				return b.ListDatabases(ctx, notion.ListParams{Limit: g.pageSize(limit), Cursor: cursor})
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

func databaseGetCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <database-id>",
		Short: "Get database metadata and property definitions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ids.Normalize(args[0])
			ctx := cmd.Context()
			db, err := withBackend(ctx, g, func(b notion.Backend) (notion.DatabaseDetail, error) {
				return b.GetDatabase(ctx, id)
			})
			if err != nil {
				return errors.Classify(err)
			}
			return emitItem(g, db)
		},
	}
}

func databaseQueryCmd(g *GlobalFlags) *cobra.Command {
	var (
		filterStr string
		sortStr   string
		limit     int
		cursor    string
	)
	cmd := &cobra.Command{
		Use:   "query <database-id>",
		Short: "Query database rows with optional filters and sorts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ids.Normalize(args[0])

			var filter any
			if filterStr != "" {
				if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
					return output.New("Invalid --filter JSON. Expected a Notion filter object. Run 'agent-notion database schema <id>' to see available properties and types.", output.FixableByAgent)
				}
			}

			var sort any
			if sortStr != "" {
				if err := json.Unmarshal([]byte(sortStr), &sort); err != nil {
					return output.New(`Invalid --sort JSON. Expected a sort object or array. Example: '[{"property":"Name","direction":"ascending"}]'`, output.FixableByAgent)
				}
				if _, isArray := sort.([]any); !isArray {
					sort = []any{sort}
				}
			}

			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.Paginated[notion.QueryRow], error) {
				return b.QueryDatabase(ctx, notion.QueryDatabaseParams{
					ID:     id,
					Filter: filter,
					Sort:   sort,
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
	cmd.Flags().StringVar(&filterStr, "filter", "", "Notion filter object (JSON)")
	cmd.Flags().StringVar(&sortStr, "sort", "", "Notion sort object or array (JSON)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results (default: page_size setting, else 50)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous page")
	return cmd
}

func databaseSchemaCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "schema <database-id>",
		Short: "Get property definitions (compact, LLM-optimized)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ids.Normalize(args[0])
			ctx := cmd.Context()
			schema, err := withBackend(ctx, g, func(b notion.Backend) (notion.DatabaseSchema, error) {
				return b.GetDatabaseSchema(ctx, id)
			})
			if err != nil {
				return errors.Classify(err)
			}
			return emitItem(g, schema)
		},
	}
}

const databaseUsageText = `agent-notion database — Database exploration and querying

SUBCOMMANDS:
  database list [--limit <n>] [--cursor <cursor>]                          List accessible databases
  database get <database-id>                                               Full metadata + property definitions
  database query <database-id> [--filter <json>] [--sort <json>] [--limit <n>] [--cursor <cursor>]
                                                                           Query rows (pages) with filters
  database schema <database-id>                                            Compact property definitions (for building filters)

QUERY FILTERS (JSON):
  --filter '{"property":"Status","status":{"equals":"In progress"}}'
  --filter '{"and":[{"property":"Priority","select":{"equals":"High"}},{"property":"Due Date","date":{"before":"2026-03-01"}}]}'
  --filter '{"property":"Tags","multi_select":{"contains":"Bug"}}'

QUERY SORTS (JSON):
  --sort '[{"property":"Due Date","direction":"ascending"}]'
  --sort '[{"timestamp":"last_edited_time","direction":"descending"}]'

SCHEMA OUTPUT: Lists property names, types, and options (for select/multi_select/status).
  Use schema to discover valid property names and values before building filters.

PROPERTY TYPES: title, rich_text, number, select, multi_select, status, date, people,
  checkbox, url, email, phone_number, files, relation, formula, rollup, unique_id,
  created_time, last_edited_time, created_by, last_edited_by

IDS: UUIDs (with or without dashes).
PAGINATION: --limit <n> --cursor <cursor>`
