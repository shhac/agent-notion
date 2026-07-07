package cli

import (
	"strings"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/ids"
	v3 "github.com/shhac/agent-notion/internal/notion/v3"
	"github.com/spf13/cobra"
)

// registerActivity wires the `activity` command group (log). v3-only.
func registerActivity(root *cobra.Command, g *GlobalFlags) {
	activity := &cobra.Command{
		Use:   "activity",
		Short: "Workspace and page activity (v3 desktop session required)",
	}
	activity.AddCommand(activityLogCmd(g))
	addDomainUsage("activity", activityUsageText)
	root.AddCommand(activity)
}

// activityEntry is one normalized activity-log record (snake_case).
type activityEntry struct {
	ID        string   `json:"id"`
	Type      string   `json:"type,omitempty"`
	PageID    string   `json:"page_id,omitempty"`
	PageTitle string   `json:"page_title,omitempty"`
	Authors   []string `json:"authors,omitempty"`
	EditTypes []string `json:"edit_types,omitempty"`
	StartTime string   `json:"start_time,omitempty"`
	EndTime   string   `json:"end_time,omitempty"`
}

func activityLogCmd(g *GlobalFlags) *cobra.Command {
	var (
		page  string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show recent activity for the workspace or a page",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			navigableBlockID := ""
			if page != "" {
				navigableBlockID = ids.Normalize(page)
			}

			resp, err := withV3Client(g, func(c *v3.Client, _ *config.V3Session) (v3.GetActivityLogResponse, error) {
				return c.GetActivityLog(cmd.Context(), v3.GetActivityLogParams{
					NavigableBlockID: navigableBlockID,
					Limit:            limit,
				})
			})
			if err != nil {
				return err
			}

			items := make([]any, 0, len(resp.ActivityIDs))
			for _, actID := range resp.ActivityIDs {
				items = append(items, buildActivityEntry(actID, resp))
			}
			return printList(g, items, nil)
		},
	}
	cmd.Flags().StringVar(&page, "page", "", "Scope to a specific page (UUID or dashless ID)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of activity entries")
	return cmd
}

func buildActivityEntry(actID string, resp v3.GetActivityLogResponse) activityEntry {
	act, ok := resp.Activities[actID]
	if !ok {
		return activityEntry{ID: actID}
	}

	blockID := act.NavigableBlockID
	if blockID == "" {
		blockID = act.ParentID
	}

	pageTitle := ""
	if blockID != "" {
		if block, ok := resp.RecordMap.GetBlock(blockID); ok {
			pageTitle = block.Property("title").Plain()
		}
	}

	var authors []string
	seen := map[string]bool{}
	for _, edit := range act.Edits {
		for _, a := range edit.Authors {
			name := a.ID
			if u, ok := resp.RecordMap.GetUser(a.ID); ok {
				name = authorName(u)
			}
			if !seen[name] {
				seen[name] = true
				authors = append(authors, name)
			}
		}
	}

	var editTypes []string
	for _, edit := range act.Edits {
		editTypes = append(editTypes, edit.Type)
	}

	return activityEntry{
		ID:        actID,
		Type:      act.Type,
		PageID:    blockID,
		PageTitle: pageTitle,
		Authors:   authors,
		EditTypes: editTypes,
		StartTime: v3.MsToISO(act.StartTime),
		EndTime:   v3.MsToISO(act.EndTime),
	}
}

func authorName(u *v3.User) string {
	parts := make([]string, 0, 2)
	if u.GivenName != "" {
		parts = append(parts, u.GivenName)
	}
	if u.FamilyName != "" {
		parts = append(parts, u.FamilyName)
	}
	return strings.Join(parts, " ")
}

const activityUsageText = `agent-notion activity — Recent workspace or page activity log

USAGE:
  activity log [--page <page-id>] [--limit <n>]

  Requires a v3 desktop session (auth import-desktop).

OPTIONS:
  --page <page-id>    Scope to a specific page (UUID or dashless ID)
  --limit <n>         Number of activity entries (default: 20)

OUTPUT:
  One NDJSON record per activity:
    { id, type, page_id, page_title, authors, edit_types, start_time, end_time }

  type: Activity type (e.g. "page-edited")
  authors: Resolved author names from workspace users
  edit_types: Types of edits within the activity (e.g. "content-change")
  --format json|yaml wraps the records in one {data: […]} envelope instead.

EXAMPLES:
  activity log                         Recent workspace-wide activity
  activity log --page <page-id>        Activity scoped to a specific page
  activity log --limit 50              Fetch more entries`
