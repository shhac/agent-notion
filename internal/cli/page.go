package cli

import (
	"encoding/json"
	"time"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/ids"
	"github.com/shhac/agent-notion/internal/notion"
	"github.com/shhac/agent-notion/internal/notion/markdown"
	v3 "github.com/shhac/agent-notion/internal/notion/v3"
	libcli "github.com/shhac/lib-agent-cli/cli"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// registerPage wires the `page` command group and its LLM usage card.
func registerPage(root *cobra.Command, g *GlobalFlags) {
	page := &cobra.Command{Use: "page", Short: "Page operations"}
	page.AddCommand(
		pageGetCmd(g),
		pageCreateCmd(g),
		pageUpdateCmd(g),
		pageTrashCmd(g),
		pageRestoreCmd(g),
		pageArchiveCmd(g),
		pageUnarchiveCmd(g),
		pageBacklinksCmd(g),
		pageHistoryCmd(g),
	)
	addDomainUsage("page", pageUsageText)
	root.AddCommand(page)
}

// --- get ---

func pageGetCmd(g *GlobalFlags) *cobra.Command {
	var content, rawContent bool
	cmd := &cobra.Command{
		Use:   "get <page-id>",
		Short: "Get page properties and optionally content",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()

			out, err := withBackend(ctx, g, func(b notion.Backend) (map[string]any, error) {
				detail, err := b.GetPage(ctx, pageID)
				if err != nil {
					return nil, err
				}
				m, err := structToMap(detail)
				if err != nil {
					return nil, err
				}
				if !content && !rawContent {
					return m, nil
				}

				all, err := b.GetAllBlocks(ctx, pageID)
				if err != nil {
					return nil, err
				}
				m["block_count"] = len(all.Blocks)
				if all.HasMore {
					m["content_truncated"] = true
				}

				if rawContent {
					blocks := make([]any, len(all.Blocks))
					for i, blk := range all.Blocks {
						blocks[i] = markdown.FlattenBlock(blk)
					}
					m["blocks"] = blocks
					return m, nil
				}

				var withChildren []string
				for _, blk := range all.Blocks {
					if blk.HasChildren {
						withChildren = append(withChildren, blk.ID)
					}
				}
				childMap := map[string][]notion.NormalizedBlock{}
				if len(withChildren) > 0 {
					childMap, err = b.GetChildBlocks(ctx, withChildren)
					if err != nil {
						return nil, err
					}
				}
				m["content"] = markdown.FromBlocks(all.Blocks, childMap)
				return m, nil
			})
			if err != nil {
				return err
			}
			return emitItem(g, out)
		},
	}
	cmd.Flags().BoolVar(&content, "content", false, "Include page content as markdown")
	cmd.Flags().BoolVar(&rawContent, "raw-content", false, "Include content as structured block objects")
	return cmd
}

// --- create ---

func pageCreateCmd(g *GlobalFlags) *cobra.Command {
	var parent, title, properties, icon string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new page",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			props, err := parseProperties(properties)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.PageCreateResult, error) {
				return b.CreatePage(ctx, notion.CreatePageParams{
					ParentID:   ids.Normalize(parent),
					Title:      title,
					Properties: props,
					Icon:       icon,
				})
			})
			if err != nil {
				return err
			}
			return emitItem(g, result)
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "Parent page ID or database ID")
	cmd.Flags().StringVar(&title, "title", "", "Page title")
	cmd.Flags().StringVar(&properties, "properties", "", "Property values (JSON, for database parents)")
	cmd.Flags().StringVar(&icon, "icon", "", "Page icon emoji")
	_ = cmd.MarkFlagRequired("parent")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

// --- update ---

func pageUpdateCmd(g *GlobalFlags) *cobra.Command {
	var title, properties, icon string
	cmd := &cobra.Command{
		Use:   "update <page-id>",
		Short: "Update page properties",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" && properties == "" && icon == "" {
				return output.New("Nothing to update. Provide --title, --properties, or --icon.", output.FixableByAgent)
			}
			props, err := parseProperties(properties)
			if err != nil {
				return err
			}
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.PageUpdateResult, error) {
				return b.UpdatePage(ctx, notion.UpdatePageParams{
					ID:         pageID,
					Title:      title,
					Properties: props,
					Icon:       icon,
				})
			})
			if err != nil {
				return err
			}
			return emitItem(g, result)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Update the page title")
	cmd.Flags().StringVar(&properties, "properties", "", "Property values to update (JSON)")
	cmd.Flags().StringVar(&icon, "icon", "", "Update the page icon emoji")
	return cmd
}

// --- trash / restore ---

func pageTrashCmd(g *GlobalFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "trash <page-id>",
		Short: "Move a page to Trash (recoverable with 'page restore')",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := libcli.RequireConfirm(yes, "this moves the page to Trash (recoverable with 'page restore')"); err != nil {
				return err
			}
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.PageTrashResult, error) {
				return b.TrashPage(ctx, pageID)
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

func pageRestoreCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <page-id>",
		Short: "Restore a page from Trash (undo 'page trash')",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.PageTrashResult, error) {
				return b.RestorePage(ctx, pageID)
			})
			if err != nil {
				return err
			}
			return emitItem(g, result)
		},
	}
	return cmd
}

// --- archive / unarchive (v3-only capability) ---

func pageArchiveCmd(g *GlobalFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "archive <page-id>",
		Short: "Archive a page (hides from search, keeps page alive — v3 only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := libcli.RequireConfirm(yes, "this archives the page (hides it from search; the page stays alive)"); err != nil {
				return err
			}
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.PageArchiveResult, error) {
				return b.ArchivePage(ctx, pageID)
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

func pageUnarchiveCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unarchive <page-id>",
		Short: "Unarchive a page (undo 'page archive' — v3 only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.PageArchiveResult, error) {
				return b.UnarchivePage(ctx, pageID)
			})
			if err != nil {
				return err
			}
			return emitItem(g, result)
		},
	}
	return cmd
}

// --- backlinks (v3-only) ---

// backlinkRecord is one deduplicated backlink source page.
type backlinkRecord struct {
	BlockID   string `json:"block_id"`
	PageID    string `json:"page_id"`
	PageTitle string `json:"page_title,omitempty"`
}

func pageBacklinksCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backlinks <page-id>",
		Short: "List pages that link to a given page (v3 desktop session required)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()

			result, err := withV3Client(g, func(c *v3.Client, _ *config.V3Session) (v3.GetBacklinksResponse, error) {
				return c.GetBacklinksForBlock(ctx, pageID)
			})
			if err != nil {
				return err
			}

			items := backlinkRecords(result)
			return printList(g, items, map[string]any{"total": len(items)})
		},
	}
	return cmd
}

// backlinkRecords resolves each backlink to its source page, deduplicating by
// page ID and preserving first-seen order.
func backlinkRecords(result v3.GetBacklinksResponse) []any {
	rm := result.RecordMap
	items := []any{}
	seen := map[string]bool{}

	for _, bl := range result.Backlinks {
		blockID := bl.MentionedFrom.BlockID
		block, _ := rm.GetBlock(blockID)

		var pageBlock *v3.Block
		if block != nil && block.ParentID != "" {
			pageBlock, _ = rm.GetBlock(block.ParentID)
		}

		pageID := blockID
		if pageBlock != nil {
			pageID = pageBlock.ID
		}
		if seen[pageID] {
			continue
		}
		seen[pageID] = true

		title := ""
		if t, ok := blockTitle(pageBlock); ok {
			title = t
		} else if t, ok := blockTitle(block); ok {
			title = t
		}
		items = append(items, backlinkRecord{BlockID: blockID, PageID: pageID, PageTitle: title})
	}
	return items
}

// blockTitle returns a block's title text and whether a title property exists.
func blockTitle(b *v3.Block) (string, bool) {
	if b == nil {
		return "", false
	}
	rt, ok := b.Properties["title"]
	if !ok {
		return "", false
	}
	return rt.Plain(), true
}

// --- history (v3-only) ---

// snapshotRecord is one version-history entry.
type snapshotRecord struct {
	ID          string   `json:"id"`
	Version     int64    `json:"version"`
	LastVersion int64    `json:"last_version"`
	Timestamp   string   `json:"timestamp"`
	Authors     []string `json:"authors"`
}

func pageHistoryCmd(g *GlobalFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "history <page-id>",
		Short: "List version history (snapshots) of a page (v3 desktop session required)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()

			result, err := withV3Client(g, func(c *v3.Client, _ *config.V3Session) (v3.GetSnapshotsResponse, error) {
				return c.GetSnapshotsList(ctx, v3.GetSnapshotsListParams{BlockID: pageID, Size: limit})
			})
			if err != nil {
				return err
			}

			items := make([]any, len(result.Snapshots))
			for i, snap := range result.Snapshots {
				authors := make([]string, len(snap.Authors))
				for j, a := range snap.Authors {
					authors[j] = a.ID
				}
				items[i] = snapshotRecord{
					ID:          snap.ID,
					Version:     snap.Version,
					LastVersion: snap.LastVersion,
					Timestamp:   isoFromMillis(snap.Timestamp),
					Authors:     authors,
				}
			}
			return printList(g, items, map[string]any{"total": len(items)})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of snapshots to fetch")
	return cmd
}

// --- shared helpers ---

// structToMap round-trips a value through JSON so a typed result (e.g.
// PageDetail) can be spread and extended with extra keys, matching the TS's
// `{ ...page, content }` output shape.
func structToMap(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// parseProperties parses the --properties JSON object, or returns nil when the
// flag is absent.
func parseProperties(s string) (map[string]any, error) {
	if s == "" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, output.New(
			"Invalid --properties JSON. Expected an object with property names as keys.",
			output.FixableByAgent)
	}
	return m, nil
}

// isoFromMillis renders unix milliseconds as a JS Date.toISOString()-style
// string (UTC, millisecond precision), matching the TS history output.
func isoFromMillis(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000Z")
}

const pageUsageText = `agent-notion page — Page operations

GET
  page get <page-id>                     Properties only
  page get <page-id> --content           Properties + markdown content
  page get <page-id> --raw-content       Properties + block objects
  Adds block_count and content_truncated (true when >1000 blocks) alongside content/blocks.

CREATE
  page create --parent <id> --title <title> [--properties <json>] [--icon <emoji>]
  Database parent: page create --parent <db-id> --title "New Task" --properties '{"Status":"Todo","Priority":"High"}'

UPDATE
  page update <page-id> [--title <title>] [--properties <json>] [--icon <emoji>]
  At least one of --title/--properties/--icon is required. To edit content, use 'block append'.

TRASH / RESTORE / ARCHIVE (Trash and Archive are independent states)
  page trash     <page-id> --yes         Move to Trash (recoverable; destructive, needs --yes)
  page restore   <page-id>               Restore from Trash
  page archive   <page-id> --yes         Archive: hide from search, page stays alive (v3; destructive, needs --yes)
  page unarchive <page-id>               Undo archive (v3)
  archive/unarchive require the v3 backend: run 'agent-notion auth import-desktop'.

BACKLINKS (v3)
  page backlinks <page-id>               Pages that link here
  Output: one record per source page {block_id, page_id, page_title}, then {"@total": n}.

HISTORY (v3)
  page history <page-id> [--limit <n>]   Version-history snapshots (default limit 20)
  Output: one record per snapshot {id, version, last_version, timestamp, authors}, then {"@total": n}.

CONTENT FORMAT (--content): headings #/##/###, lists -/1., todos - [ ]/- [x],
  code fences, quotes >, callouts > icon text, images ![caption](url), dividers ---.

LIMITS: max 1000 blocks per fetch (content_truncated=true if exceeded).
FILE URLS: image/file URLs expire after ~1 hour — re-fetch for fresh URLs.

OUTPUT: NDJSON on stdout; --format json|yaml for a pretty envelope. Page IDs may be
  dashless (from URLs) or dashed UUIDs. Errors on stderr as {error, fixable_by, hint}.`
