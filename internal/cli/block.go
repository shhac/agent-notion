package cli

import (
	"encoding/json"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/ids"
	"github.com/shhac/agent-notion/internal/notion"
	"github.com/shhac/agent-notion/internal/notion/markdown"
	libcli "github.com/shhac/lib-agent-cli/cli"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// registerBlock wires the `block` command group and its LLM usage card.
func registerBlock(root *cobra.Command, g *GlobalFlags) {
	block := &cobra.Command{Use: "block", Short: "Block (content) operations"}
	block.AddCommand(
		blockListCmd(g),
		blockAppendCmd(g),
		blockUpdateCmd(g),
		blockDeleteCmd(g),
		blockMoveCmd(g),
		blockReplaceCmd(g),
	)
	addDomainUsage("block", blockUsageText)
	root.AddCommand(block)
}

// --- list ---

type blockListResult struct {
	PageID     string `json:"page_id"`
	Content    string `json:"content"`
	BlockCount int    `json:"block_count"`
	HasMore    bool   `json:"has_more"`
}

func blockListCmd(g *GlobalFlags) *cobra.Command {
	var (
		raw    bool
		limit  int
		cursor string
	)
	cmd := &cobra.Command{
		Use:   "list <page-id>",
		Short: "List blocks (content) of a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()

			if raw {
				page, err := withBackend(ctx, g, func(b notion.Backend) (notion.Paginated[notion.NormalizedBlock], error) {
					return b.ListBlocks(ctx, notion.ListBlocksParams{ID: pageID, Limit: blockRawLimit(limit), Cursor: cursor})
				})
				if err != nil {
					return err
				}
				flat := make([]map[string]any, len(page.Items))
				for i, item := range page.Items {
					flat[i] = markdown.FlattenBlock(item)
				}
				return printPaginated(g, notion.Paginated[map[string]any]{
					Items: flat, HasMore: page.HasMore, NextCursor: page.NextCursor,
				})
			}

			out, err := withBackend(ctx, g, func(b notion.Backend) (blockListResult, error) {
				all, err := b.GetAllBlocks(ctx, pageID)
				if err != nil {
					return blockListResult{}, err
				}
				content, err := renderMarkdown(ctx, b, all.Blocks, config.ReadSettings().MaxDepth)
				if err != nil {
					return blockListResult{}, err
				}
				return blockListResult{
					PageID:     pageID,
					Content:    content,
					BlockCount: len(all.Blocks),
					HasMore:    all.HasMore,
				}, nil
			})
			if err != nil {
				return err
			}
			return emitItem(g, out)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Return structured block objects instead of markdown")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max blocks (raw mode; default 100)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor (raw mode)")
	return cmd
}

// blockRawLimit defaults the raw listing to 100 blocks (the TS default).
func blockRawLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	return limit
}

// --- append ---

type blockAppendResult struct {
	PageID      string `json:"page_id"`
	BlocksAdded int    `json:"blocks_added"`
}

func blockAppendCmd(g *GlobalFlags) *cobra.Command {
	var content, blocks string
	cmd := &cobra.Command{
		Use:   "append <page-id>",
		Short: "Append blocks to a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			children, err := blockChildren(content, blocks)
			if err != nil {
				return err
			}
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()

			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.AppendBlocksResult, error) {
				return b.AppendBlocks(ctx, notion.AppendBlocksParams{ID: pageID, Blocks: children})
			})
			if err != nil {
				return err
			}
			return emitItem(g, blockAppendResult{PageID: pageID, BlocksAdded: result.BlocksAdded})
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "Content as markdown")
	cmd.Flags().StringVar(&blocks, "blocks", "", "Content as Notion block objects (JSON array)")
	return cmd
}

// --- update ---

func blockUpdateCmd(g *GlobalFlags) *cobra.Command {
	var content string
	cmd := &cobra.Command{
		Use:   "update <block-id>",
		Short: "Update a block's content",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if content == "" {
				return output.New("Provide --content with the new text.", output.FixableByAgent)
			}
			blockID := ids.Normalize(args[0])
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.BlockUpdateResult, error) {
				return b.UpdateBlock(ctx, notion.UpdateBlockParams{ID: blockID, Content: &content})
			})
			if err != nil {
				return err
			}
			return emitItem(g, result)
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "New text content for the block")
	return cmd
}

// --- delete ---

func blockDeleteCmd(g *GlobalFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <block-id>",
		Short: "Delete a block",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := libcli.RequireConfirm(yes, "this deletes the block"); err != nil {
				return err
			}
			blockID := ids.Normalize(args[0])
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.BlockDeleteResult, error) {
				return b.DeleteBlock(ctx, blockID)
			})
			if err != nil {
				return err
			}
			return emitItem(g, result)
		},
	}
	libcli.AddConfirmFlag(cmd, &yes)
	return cmd
}

// --- move (v3-only capability) ---

func blockMoveCmd(g *GlobalFlags) *cobra.Command {
	var parent, after string
	cmd := &cobra.Command{
		Use:   "move <block-id>",
		Short: "Move a block to a new position or parent (v3 only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			blockID := ids.Normalize(args[0])
			params := notion.MoveBlockParams{ID: blockID}
			if parent != "" {
				params.ParentID = ids.Normalize(parent)
			}
			if after != "" {
				params.AfterID = ids.Normalize(after)
			}
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.BlockMoveResult, error) {
				return b.MoveBlock(ctx, params)
			})
			if err != nil {
				return err
			}
			return emitItem(g, result)
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "New parent block (for moving into a container)")
	cmd.Flags().StringVar(&after, "after", "", "Place after this block (omit for first position)")
	return cmd
}

// --- replace ---

type blockReplaceResult struct {
	PageID        string `json:"page_id"`
	BlocksDeleted int    `json:"blocks_deleted"`
	BlocksAdded   int    `json:"blocks_added"`
}

func blockReplaceCmd(g *GlobalFlags) *cobra.Command {
	var content, blocks string
	var yes bool
	cmd := &cobra.Command{
		Use:   "replace <page-id>",
		Short: "Replace all blocks on a page (deletes existing, appends new)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := libcli.RequireConfirm(yes, "this deletes every existing block on the page before appending the new content"); err != nil {
				return err
			}
			children, err := blockChildren(content, blocks)
			if err != nil {
				return err
			}
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()

			out, err := withBackend(ctx, g, func(b notion.Backend) (blockReplaceResult, error) {
				existing, err := b.GetAllBlocks(ctx, pageID)
				if err != nil {
					return blockReplaceResult{}, err
				}
				for _, blk := range existing.Blocks {
					if _, err := b.DeleteBlock(ctx, blk.ID); err != nil {
						return blockReplaceResult{}, err
					}
				}
				added := 0
				if len(children) > 0 {
					appended, err := b.AppendBlocks(ctx, notion.AppendBlocksParams{ID: pageID, Blocks: children})
					if err != nil {
						return blockReplaceResult{}, err
					}
					added = appended.BlocksAdded
				}
				return blockReplaceResult{PageID: pageID, BlocksDeleted: len(existing.Blocks), BlocksAdded: added}, nil
			})
			if err != nil {
				return err
			}
			return emitItem(g, out)
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "New content as markdown")
	cmd.Flags().StringVar(&blocks, "blocks", "", "New content as Notion block objects (JSON array)")
	libcli.AddConfirmFlag(cmd, &yes)
	return cmd
}

// --- shared ---

// blockChildren resolves the --content (markdown) or --blocks (JSON array)
// flags into official-API block objects, requiring exactly one source.
func blockChildren(content, blocks string) ([]any, error) {
	if content == "" && blocks == "" {
		return nil, output.New("Provide --content (markdown) or --blocks (JSON array).", output.FixableByAgent)
	}
	if blocks != "" {
		var arr []any
		if err := json.Unmarshal([]byte(blocks), &arr); err != nil {
			return nil, output.New("Invalid --blocks JSON. Expected an array of Notion block objects.", output.FixableByAgent)
		}
		return arr, nil
	}
	md := markdown.ToBlocks(content)
	children := make([]any, len(md))
	for i, b := range md {
		children[i] = b
	}
	return children, nil
}

const blockUsageText = `agent-notion block — Read and write page content (blocks)

LIST (read content)
  block list <page-id>                               Content as markdown (default)
  block list <page-id> --raw                         Structured block objects (NDJSON, paginated)
  block list <page-id> --raw --limit 10 --cursor <c> Paginate raw blocks (raw default limit 100)
  Markdown output: {page_id, content, block_count, has_more}
  Raw output: one record per block {id, type, content, has_children}, then {"@pagination": …}

APPEND (add content)
  block append <page-id> --content <markdown>        Append markdown (converted to blocks)
  block append <page-id> --blocks <json>             Append raw Notion block objects (JSON array)
  Output: {page_id, blocks_added}

UPDATE (edit existing block)
  block update <block-id> --content <text>           Replace a block's text (use 'block list --raw' for IDs)
  Output: {id, last_edited_at}

DELETE (destructive, needs --yes)
  block delete <block-id> --yes                      Delete a single block
  Output: {id, deleted}

MOVE (v3 only)
  block move <block-id> --after <block-id>           Move after a sibling
  block move <block-id>                              Move to first position in parent
  block move <block-id> --parent <block-id>          Move into a container (first child)
  Output: {id, parent_id, after_id?}. Needs the v3 backend: 'agent-notion auth import-desktop'.

REPLACE (destructive, needs --yes)
  block replace <page-id> --content <markdown> --yes Delete all blocks, then append new content
  block replace <page-id> --blocks <json> --yes      Delete all blocks, then append raw blocks
  Output: {page_id, blocks_deleted, blocks_added}

LIMITS: max 1000 blocks per request; 500KB append payload; 2000 chars per rich-text object.
  Cannot insert at a specific position — use replace for a full rewrite.
NESTED BLOCKS: children (toggles, columns) are fetched recursively in markdown mode; in raw
  mode check has_children and re-fetch as needed.

OUTPUT: NDJSON on stdout; --format json|yaml for a pretty envelope. IDs may be dashless or
  dashed UUIDs. Errors on stderr as {error, fixable_by, hint}.`
