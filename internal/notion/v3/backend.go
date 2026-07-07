// V3 backend — implements notion.Backend over the v3 internal API Client.
// Reads and writes go through saveTransactions; comment orchestration lives in
// comments_client.go. The Pages, Blocks, and Databases sections live in
// backend_pages.go, backend_blocks.go, and backend_databases.go respectively.

package v3

import (
	"context"
	"fmt"

	"github.com/shhac/agent-notion/internal/notion"
)

// Backend implements notion.Backend using the v3 internal API.
type Backend struct {
	Client *Client
}

var _ notion.Backend = (*Backend)(nil)

// NewBackend wraps a v3 Client as a notion.Backend.
func NewBackend(c *Client) *Backend { return &Backend{Client: c} }

// Kind reports the backend kind.
func (b *Backend) Kind() string { return "v3" }

// --- Search ---

// Search runs a workspace search and applies the page/database filter locally
// (v3 search has no direct type filter).
func (b *Backend) Search(ctx context.Context, params notion.SearchParams) (notion.Paginated[notion.SearchResult], error) {
	resp, err := b.Client.Search(ctx, SearchParams{Query: params.Query, Limit: orInt(params.Limit, 20)})
	if err != nil {
		return notion.Paginated[notion.SearchResult]{}, err
	}

	items := []notion.SearchResult{}
	for _, hit := range resp.Results {
		block, ok := resp.RecordMap.GetBlock(hit.ID)
		if !ok {
			continue
		}
		isDB := isDatabaseBlock(block)
		if params.Filter == "page" && isDB {
			continue
		}
		if params.Filter == "database" && !isDB {
			continue
		}
		items = append(items, ToSearchResult(block))
	}

	return notion.Paginated[notion.SearchResult]{Items: items, HasMore: false}, nil
}

// --- Comments (delegated to comments_client.go) ---

// ListComments lists a page's comments.
func (b *Backend) ListComments(ctx context.Context, params notion.ListCommentsParams) (notion.Paginated[notion.CommentItem], error) {
	return listComments(ctx, b.Client, params)
}

// AddComment adds a page-level comment.
func (b *Backend) AddComment(ctx context.Context, params notion.AddCommentParams) (notion.CommentCreateResult, error) {
	return addComment(ctx, b.Client, params)
}

// AddInlineComment adds an inline comment anchored to text within a block.
func (b *Backend) AddInlineComment(ctx context.Context, params notion.AddInlineCommentParams) (notion.CommentCreateResult, error) {
	return addInlineComment(ctx, b.Client, params)
}

// --- Users ---

// ListUsers lists workspace users from the bootstrap record map.
func (b *Backend) ListUsers(ctx context.Context, _ notion.ListParams) (notion.Paginated[notion.UserItem], error) {
	resp, err := b.Client.LoadUserContent(ctx)
	if err != nil {
		return notion.Paginated[notion.UserItem]{}, err
	}
	items := []notion.UserItem{}
	for _, u := range resp.RecordMap.AllUsers() {
		items = append(items, ToUserItem(u))
	}
	return notion.Paginated[notion.UserItem]{Items: items, HasMore: false}, nil
}

// GetMe returns the current user and workspace name.
func (b *Backend) GetMe(ctx context.Context) (notion.UserMe, error) {
	resp, err := b.Client.LoadUserContent(ctx)
	if err != nil {
		return notion.UserMe{}, err
	}
	user, ok := resp.RecordMap.FirstUser()
	if !ok {
		return notion.UserMe{}, fmt.Errorf("Could not retrieve user information")
	}
	spaceName := ""
	if space, ok := resp.RecordMap.FirstSpace(); ok {
		spaceName = space.Name
	}
	return ToUserMe(user, spaceName), nil
}

// --- Private helpers ---

// isDatabaseBlock reports whether a block is a database (collection view).
func isDatabaseBlock(b *Block) bool {
	return b.Type == "collection_view_page" || b.Type == "collection_view"
}

// resolveCollection finds the collection for a database page, fetching it via
// syncRecordValues when it is not already in the record map. Returns nil (no
// error) when it cannot be resolved.
func (b *Backend) resolveCollection(ctx context.Context, pageID string, rm RecordMap) (*Collection, error) {
	if _, ok := rm.GetBlock(pageID); !ok {
		return nil, nil
	}

	collectionID := blockCollectionID(rm, pageID)
	if collectionID != "" {
		if existing, ok := rm.GetCollection(collectionID); ok {
			return existing, nil
		}
		resp, err := b.Client.SyncRecordValues(ctx, []SyncRequest{{Pointer: SyncPointer{ID: collectionID, Table: "collection"}, Version: 0}})
		if err != nil {
			return nil, err
		}
		if collection, ok := resp.RecordMap.GetCollection(collectionID); ok {
			return collection, nil
		}
		return nil, nil
	}

	if collection, ok := rm.FirstCollection(); ok {
		return collection, nil
	}
	return nil, nil
}

// blockCollectionID reads a block's collection_id (set on collection_view
// blocks).
func blockCollectionID(rm RecordMap, id string) string {
	block, ok := rm.GetBlock(id)
	if !ok {
		return ""
	}
	return block.CollectionID
}

// blockViewIDs reads a block's view_ids (set on collection_view blocks).
func blockViewIDs(rm RecordMap, id string) []string {
	block, ok := rm.GetBlock(id)
	if !ok {
		return nil
	}
	return block.ViewIDs
}
