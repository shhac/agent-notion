// V3 comment orchestration — the HTTP halves of comment list/add that drive
// loadPageChunk + syncRecordValues + saveTransactions. The pure discussion
// logic (CollectDiscussionIDs, BuildAnchorTextMap, FindOccurrence) lives in
// comments.go.

package v3

import (
	"context"
	"fmt"
	"time"

	"github.com/shhac/agent-notion/internal/notion"
)

// listComments loads a page, collects its discussions and comments (fetching
// any missing records), and returns them as normalized CommentItems.
func listComments(ctx context.Context, c *Client, params notion.ListCommentsParams) (notion.Paginated[notion.CommentItem], error) {
	empty := notion.Paginated[notion.CommentItem]{Items: []notion.CommentItem{}, HasMore: false}
	limit := orInt(params.Limit, 50)

	resp, err := c.LoadPageChunk(ctx, LoadPageChunkParams{PageID: params.PageID, Limit: 100})
	if err != nil {
		return notion.Paginated[notion.CommentItem]{}, err
	}
	rm := resp.RecordMap

	discussionIDs := CollectDiscussionIDs(rm, params.PageID)
	if len(discussionIDs) == 0 {
		return empty, nil
	}

	// Fetch discussions missing from the record map.
	var missingDiscussions []string
	for _, id := range discussionIDs {
		if _, ok := rm.GetDiscussion(id); !ok {
			missingDiscussions = append(missingDiscussions, id)
		}
	}
	if err := fetchMissingRecords(ctx, c, rm, "discussion", missingDiscussions); err != nil {
		return notion.Paginated[notion.CommentItem]{}, err
	}

	// Gather all comment IDs across discussions.
	var allCommentIDs []string
	for _, id := range discussionIDs {
		if disc, ok := rm.GetDiscussion(id); ok {
			allCommentIDs = append(allCommentIDs, disc.Comments...)
		}
	}

	var missingComments []string
	for _, id := range allCommentIDs {
		if _, ok := rm.GetComment(id); !ok {
			missingComments = append(missingComments, id)
		}
	}
	if err := fetchMissingRecords(ctx, c, rm, "comment", missingComments); err != nil {
		return notion.Paginated[notion.CommentItem]{}, err
	}

	anchors := BuildAnchorTextMap(rm, discussionIDs)

	items := []notion.CommentItem{}
	for _, commentID := range allCommentIDs {
		if len(items) >= limit {
			break
		}
		comment, ok := rm.GetComment(commentID)
		if !ok || !commentAlive(comment) {
			continue
		}
		var user *User
		if comment.CreatedByID != "" {
			if u, ok := rm.GetUser(comment.CreatedByID); ok {
				user = u
			}
		}
		// A comment's parent_id is the discussion it belongs to.
		items = append(items, ToCommentItem(comment, user, anchors[comment.ParentID]))
	}

	return notion.Paginated[notion.CommentItem]{
		Items:   items,
		HasMore: len(items) < len(allCommentIDs),
	}, nil
}

// commentAlive matches the TS `!comment.alive` check: a comment counts as alive
// only when its alive flag is present and true.
func commentAlive(c *Comment) bool {
	return c.Alive != nil && *c.Alive
}

// fetchMissingRecords syncs the given record IDs (at version -1) and merges the
// result into rm.
func fetchMissingRecords(ctx context.Context, c *Client, rm RecordMap, table string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	pointers := make([]SyncPointer, 0, len(ids))
	for _, id := range ids {
		pointers = append(pointers, SyncPointer{ID: id, Table: table})
	}
	resp, err := c.SyncRecordValuesForPointers(ctx, pointers)
	if err != nil {
		return err
	}
	rm.Merge(resp.RecordMap)
	return nil
}

// addComment creates a page-level discussion + first comment.
func addComment(ctx context.Context, c *Client, params notion.AddCommentParams) (notion.CommentCreateResult, error) {
	discussionID := newUUID()
	commentID := newUUID()
	now := time.Now()

	ops := CreateCommentOps(CreateCommentParams{
		DiscussionID: discussionID,
		CommentID:    commentID,
		PageID:       params.PageID,
		SpaceID:      c.SpaceID,
		UserID:       c.UserID,
		Text:         params.Body,
	}, now)

	if err := c.SaveTransactions(ctx, ops); err != nil {
		return notion.CommentCreateResult{}, err
	}
	return notion.CommentCreateResult{
		ID:           commentID,
		DiscussionID: discussionID,
		Body:         params.Body,
		CreatedAt:    MsToISO(now.UnixMilli()),
	}, nil
}

// addInlineComment anchors a new discussion to a run of text within a block by
// injecting the ["m", discussionID] decoration on the target occurrence.
func addInlineComment(ctx context.Context, c *Client, params notion.AddInlineCommentParams) (notion.CommentCreateResult, error) {
	empty := notion.CommentCreateResult{}
	discussionID := newUUID()
	commentID := newUUID()

	resp, err := c.SyncRecordValues(ctx, []SyncRequest{{Pointer: SyncPointer{ID: params.BlockID, Table: "block"}, Version: -1}})
	if err != nil {
		return empty, err
	}
	block, ok := resp.RecordMap.GetBlock(params.BlockID)
	if !ok {
		return empty, fmt.Errorf("Block not found: %s", params.BlockID)
	}

	currentTitle := block.Property("title")
	if len(currentTitle) == 0 {
		return empty, fmt.Errorf("Block %s has no text content to annotate.", params.BlockID)
	}

	plain := currentTitle.Plain()
	occurrence := orInt(params.Occurrence, 1)
	start, found := FindOccurrence(plain, params.Text, occurrence)
	if start == -1 {
		if found == 0 {
			return empty, fmt.Errorf("Text '%s' not found in block %s.", params.Text, params.BlockID)
		}
		plural := "s"
		if found == 1 {
			plural = ""
		}
		return empty, fmt.Errorf("Text '%s' has only %d occurrence%s in this block.", params.Text, found, plural)
	}

	end := start + len([]rune(params.Text))
	updatedTitle, err := AddDecorationToRange(currentTitle, start, end, Decoration{Type: "m", Args: []any{discussionID}})
	if err != nil {
		return empty, err
	}

	now := time.Now()
	ops := CreateInlineCommentOps(CreateInlineCommentParams{
		DiscussionID: discussionID,
		CommentID:    commentID,
		BlockID:      params.BlockID,
		SpaceID:      c.SpaceID,
		UserID:       c.UserID,
		Text:         params.Body,
		UpdatedTitle: updatedTitle,
	}, now)

	if err := c.SaveTransactions(ctx, ops); err != nil {
		return empty, err
	}
	return notion.CommentCreateResult{
		ID:           commentID,
		DiscussionID: discussionID,
		Body:         params.Body,
		CreatedAt:    MsToISO(now.UnixMilli()),
	}, nil
}
