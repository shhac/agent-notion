// Official backend — implements notion.Backend over the official REST Client.
// The backend-level orchestration (createPage's parent probe, getAllBlocks
// paging cap, cursor handling) already lives in the Client, so this is a thin
// adapter from interface params to client calls, plus guidance errors for the
// four operations only the v3 backend supports.

package official

import (
	"context"

	"github.com/shhac/agent-notion/internal/notion"
	output "github.com/shhac/lib-agent-output"
)

// Backend implements notion.Backend using the official REST API.
type Backend struct {
	client Client
}

var _ notion.Backend = (*Backend)(nil)

// NewBackend wraps a REST Client as a notion.Backend.
func NewBackend(client Client) *Backend { return &Backend{client: client} }

// Kind reports the backend kind.
func (b *Backend) Kind() string { return "official" }

// v3ImportHint points users at the command that enables the v3 backend, which
// implements the operations the official REST API cannot.
const v3ImportHint = "Run 'agent-notion auth import-desktop' to set up the v3 backend."

// v3RequiredErr builds the guidance error for a v3-only operation.
func v3RequiredErr(msg string) error {
	return output.New(msg, output.FixableByHuman).WithHint(v3ImportHint)
}

// --- Search ---

// Search runs a workspace search.
func (b *Backend) Search(ctx context.Context, params notion.SearchParams) (notion.Paginated[notion.SearchResult], error) {
	return b.client.Search(ctx, SearchParams{Query: params.Query, Filter: params.Filter, Limit: params.Limit, Cursor: params.Cursor})
}

// --- Databases ---

// ListDatabases lists databases.
func (b *Backend) ListDatabases(ctx context.Context, params notion.ListParams) (notion.Paginated[notion.DatabaseListItem], error) {
	return b.client.ListDatabases(ctx, ListParams{Limit: params.Limit, Cursor: params.Cursor})
}

// GetDatabase retrieves a database's detail.
func (b *Backend) GetDatabase(ctx context.Context, id string) (notion.DatabaseDetail, error) {
	return b.client.GetDatabase(ctx, id)
}

// QueryDatabase runs a database query.
func (b *Backend) QueryDatabase(ctx context.Context, params notion.QueryDatabaseParams) (notion.Paginated[notion.QueryRow], error) {
	return b.client.QueryDatabase(ctx, QueryParams{ID: params.ID, Filter: params.Filter, Sort: params.Sort, Limit: params.Limit, Cursor: params.Cursor})
}

// GetDatabaseSchema retrieves a database's flat schema.
func (b *Backend) GetDatabaseSchema(ctx context.Context, id string) (notion.DatabaseSchema, error) {
	return b.client.GetDatabaseSchema(ctx, id)
}

// --- Pages ---

// GetPage retrieves a page's detail.
func (b *Backend) GetPage(ctx context.Context, id string) (notion.PageDetail, error) {
	return b.client.GetPage(ctx, id)
}

// CreatePage creates a page; the client probes the parent to pick the property
// shape (database vs page).
func (b *Backend) CreatePage(ctx context.Context, params notion.CreatePageParams) (notion.PageCreateResult, error) {
	return b.client.CreatePage(ctx, CreatePageParams{ParentID: params.ParentID, Title: params.Title, Properties: params.Properties, Icon: params.Icon})
}

// UpdatePage updates a page's title, properties, and/or icon.
func (b *Backend) UpdatePage(ctx context.Context, params notion.UpdatePageParams) (notion.PageUpdateResult, error) {
	return b.client.UpdatePage(ctx, UpdatePageParams{ID: params.ID, Title: params.Title, Properties: params.Properties, Icon: params.Icon})
}

// TrashPage moves a page to Trash.
func (b *Backend) TrashPage(ctx context.Context, id string) (notion.PageTrashResult, error) {
	return b.client.TrashPage(ctx, id)
}

// RestorePage restores a trashed page.
func (b *Backend) RestorePage(ctx context.Context, id string) (notion.PageTrashResult, error) {
	return b.client.RestorePage(ctx, id)
}

// ArchivePage is unsupported on the official REST API (real Archive is a v3
// capability, distinct from Trash).
func (b *Backend) ArchivePage(_ context.Context, _ string) (notion.PageArchiveResult, error) {
	return notion.PageArchiveResult{}, v3RequiredErr(
		"Real Archive (distinct from Trash) requires the v3 backend. To move a page to Trash, use 'page trash' instead.")
}

// UnarchivePage is unsupported on the official REST API (real Archive is a v3
// capability, distinct from Trash).
func (b *Backend) UnarchivePage(_ context.Context, _ string) (notion.PageArchiveResult, error) {
	return notion.PageArchiveResult{}, v3RequiredErr(
		"Unarchive (real Archive, distinct from Trash) requires the v3 backend. To restore a page from Trash, use 'page restore' instead.")
}

// --- Blocks ---

// ListBlocks returns one page of a block's direct children.
func (b *Backend) ListBlocks(ctx context.Context, params notion.ListBlocksParams) (notion.Paginated[notion.NormalizedBlock], error) {
	return b.client.ListBlocks(ctx, ListBlocksParams{ID: params.ID, Limit: params.Limit, Cursor: params.Cursor})
}

// GetAllBlocks fetches up to 1000 descendant blocks (paging handled by the
// client).
func (b *Backend) GetAllBlocks(ctx context.Context, id string) (notion.BlockListResult, error) {
	return b.client.GetAllBlocks(ctx, id)
}

// GetChildBlocks fetches the children of each given block.
func (b *Backend) GetChildBlocks(ctx context.Context, blockIDs []string) (map[string][]notion.NormalizedBlock, error) {
	return b.client.GetChildBlocks(ctx, blockIDs)
}

// AppendBlocks appends official-API block objects and reports the count added.
func (b *Backend) AppendBlocks(ctx context.Context, params notion.AppendBlocksParams) (notion.AppendBlocksResult, error) {
	added, err := b.client.AppendBlocks(ctx, params.ID, params.Blocks)
	if err != nil {
		return notion.AppendBlocksResult{}, err
	}
	return notion.AppendBlocksResult{BlocksAdded: added}, nil
}

// UpdateBlock updates a block's rich text.
func (b *Backend) UpdateBlock(ctx context.Context, params notion.UpdateBlockParams) (notion.BlockUpdateResult, error) {
	return b.client.UpdateBlock(ctx, UpdateBlockParams{ID: params.ID, Content: params.Content, Type: params.Type})
}

// DeleteBlock deletes a block.
func (b *Backend) DeleteBlock(ctx context.Context, id string) (notion.BlockDeleteResult, error) {
	return b.client.DeleteBlock(ctx, id)
}

// MoveBlock is unsupported on the official REST API (block reordering is a v3
// capability).
func (b *Backend) MoveBlock(_ context.Context, _ notion.MoveBlockParams) (notion.BlockMoveResult, error) {
	return notion.BlockMoveResult{}, v3RequiredErr("Block reordering requires the v3 backend.")
}

// --- Comments ---

// ListComments lists a page's comments.
func (b *Backend) ListComments(ctx context.Context, params notion.ListCommentsParams) (notion.Paginated[notion.CommentItem], error) {
	return b.client.ListComments(ctx, ListCommentsParams{PageID: params.PageID, Limit: params.Limit, Cursor: params.Cursor})
}

// AddComment adds a page-level comment.
func (b *Backend) AddComment(ctx context.Context, params notion.AddCommentParams) (notion.CommentCreateResult, error) {
	return b.client.AddComment(ctx, params.PageID, params.Body)
}

// AddInlineComment is unsupported on the official REST API (inline comments
// anchored to text are a v3 capability).
func (b *Backend) AddInlineComment(_ context.Context, _ notion.AddInlineCommentParams) (notion.CommentCreateResult, error) {
	return notion.CommentCreateResult{}, v3RequiredErr("Inline comments require the v3 backend.")
}

// --- Users ---

// ListUsers lists workspace users.
func (b *Backend) ListUsers(ctx context.Context, params notion.ListParams) (notion.Paginated[notion.UserItem], error) {
	return b.client.ListUsers(ctx, ListParams{Limit: params.Limit, Cursor: params.Cursor})
}

// GetMe returns the current identity.
func (b *Backend) GetMe(ctx context.Context) (notion.UserMe, error) {
	return b.client.GetMe(ctx)
}

// --- Utility ---

// IsDatabase reports whether the ID refers to a database. Any lookup error is
// treated as "not a database", matching the TS behavior.
func (b *Backend) IsDatabase(ctx context.Context, id string) (bool, error) {
	return b.client.isDatabase(ctx, id), nil
}
