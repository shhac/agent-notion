// Backend is the operation surface the CLI needs; the official REST and v3
// internal backends both implement it. Context is the first argument on every
// method. Methods work exclusively with the normalized types in types.go.

package notion

import "context"

// Backend abstracts a Notion API implementation (official REST or v3 internal).
type Backend interface {
	// Kind reports which backend this is: "official" or "v3".
	Kind() string

	// --- Search ---
	Search(ctx context.Context, params SearchParams) (Paginated[SearchResult], error)

	// --- Databases ---
	ListDatabases(ctx context.Context, params ListParams) (Paginated[DatabaseListItem], error)
	GetDatabase(ctx context.Context, id string) (DatabaseDetail, error)
	QueryDatabase(ctx context.Context, params QueryDatabaseParams) (Paginated[QueryRow], error)
	GetDatabaseSchema(ctx context.Context, id string) (DatabaseSchema, error)

	// --- Pages ---
	GetPage(ctx context.Context, id string) (PageDetail, error)
	CreatePage(ctx context.Context, params CreatePageParams) (PageCreateResult, error)
	UpdatePage(ctx context.Context, params UpdatePageParams) (PageUpdateResult, error)
	// TrashPage moves a page to Trash (recoverable via RestorePage).
	TrashPage(ctx context.Context, id string) (PageTrashResult, error)
	RestorePage(ctx context.Context, id string) (PageTrashResult, error)
	// ArchivePage sets the real Archive state (distinct from Trash: keeps the
	// page alive but hides it from search). v3-only in practice; the official
	// REST API does not expose this state.
	ArchivePage(ctx context.Context, id string) (PageArchiveResult, error)
	UnarchivePage(ctx context.Context, id string) (PageArchiveResult, error)

	// --- Blocks ---
	// ListBlocks returns a page of a block's direct children (for --raw mode).
	ListBlocks(ctx context.Context, params ListBlocksParams) (Paginated[NormalizedBlock], error)
	// GetAllBlocks fetches up to 1000 blocks (for markdown/content mode).
	GetAllBlocks(ctx context.Context, id string) (BlockListResult, error)
	// GetChildBlocks fetches the children of each given block.
	GetChildBlocks(ctx context.Context, blockIDs []string) (map[string][]NormalizedBlock, error)
	AppendBlocks(ctx context.Context, params AppendBlocksParams) (AppendBlocksResult, error)
	UpdateBlock(ctx context.Context, params UpdateBlockParams) (BlockUpdateResult, error)
	DeleteBlock(ctx context.Context, id string) (BlockDeleteResult, error)
	MoveBlock(ctx context.Context, params MoveBlockParams) (BlockMoveResult, error)

	// --- Comments ---
	ListComments(ctx context.Context, params ListCommentsParams) (Paginated[CommentItem], error)
	AddComment(ctx context.Context, params AddCommentParams) (CommentCreateResult, error)
	AddInlineComment(ctx context.Context, params AddInlineCommentParams) (CommentCreateResult, error)

	// --- Users ---
	ListUsers(ctx context.Context, params ListParams) (Paginated[UserItem], error)
	GetMe(ctx context.Context) (UserMe, error)

	// --- Utility ---
	// IsDatabase reports whether the ID refers to a database (used by page
	// create to detect parent type).
	IsDatabase(ctx context.Context, id string) (bool, error)
}

// ListParams is the common limit/cursor pagination input.
type ListParams struct {
	Limit  int
	Cursor string
}

// SearchParams configures a workspace search.
type SearchParams struct {
	Query  string
	Filter string // "" | "page" | "database"
	Limit  int
	Cursor string
}

// QueryDatabaseParams configures a database query. Filter and Sort are opaque
// backend-specific query objects passed through untouched.
type QueryDatabaseParams struct {
	ID     string
	Filter any
	Sort   any
	Limit  int
	Cursor string
}

// CreatePageParams describes a page to create.
type CreatePageParams struct {
	ParentID   string
	Title      string
	Properties map[string]any
	Icon       string
}

// UpdatePageParams describes edits to a page. Empty Title/Icon and nil
// Properties are left unchanged.
type UpdatePageParams struct {
	ID         string
	Title      string
	Properties map[string]any
	Icon       string
}

// ListBlocksParams configures a paged child-block listing.
type ListBlocksParams struct {
	ID     string
	Limit  int
	Cursor string
}

// AppendBlocksParams describes blocks (official API block objects) to append.
type AppendBlocksParams struct {
	ID     string
	Blocks []any
}

// AppendBlocksResult reports how many blocks were appended.
type AppendBlocksResult struct {
	BlocksAdded int `json:"blocks_added"`
}

// UpdateBlockParams describes edits to a block. Content is nil when unchanged.
type UpdateBlockParams struct {
	ID      string
	Content *string
	Type    string
}

// MoveBlockParams describes a block move. Empty ParentID keeps the current
// parent; empty AfterID prepends.
type MoveBlockParams struct {
	ID       string
	ParentID string
	AfterID  string
}

// ListCommentsParams configures a comment listing for a page.
type ListCommentsParams struct {
	PageID string
	Limit  int
	Cursor string
}

// AddCommentParams describes a page-level comment to add.
type AddCommentParams struct {
	PageID string
	Body   string
}

// AddInlineCommentParams describes an inline comment anchored to text within a
// block. Occurrence selects the nth match (1-based; 0 means the first).
type AddInlineCommentParams struct {
	BlockID    string
	Body       string
	Text       string
	Occurrence int
}
