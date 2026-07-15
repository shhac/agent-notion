// Package notion holds the normalized types shared by the two backends
// (official REST API and v3 internal API). Both transform their wire formats
// into these; CLI commands work exclusively with them.
//
// JSON tags are snake_case — the family convention, matching Notion's own
// official API naming (the TS implementation's camelCase was a local
// invention and is an intended contract break, catalogued in the tracker).
package notion

// Paginated is a page of results plus the cursor contract.
type Paginated[T any] struct {
	Items      []T    `json:"items"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// ParentRef locates a resource's parent.
type ParentRef struct {
	Type string `json:"type"` // database | page | workspace | block
	ID   string `json:"id,omitempty"`
}

// SearchResult is one hit from a workspace search.
type SearchResult struct {
	ID           string     `json:"id"`
	Type         string     `json:"type"` // page | database
	Title        string     `json:"title"`
	URL          string     `json:"url"`
	Parent       *ParentRef `json:"parent,omitempty"`
	LastEditedAt string     `json:"last_edited_at,omitempty"`
}

// DatabaseListItem is a database as listed by `database list`.
type DatabaseListItem struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	URL           string     `json:"url"`
	Parent        *ParentRef `json:"parent,omitempty"`
	PropertyCount int        `json:"property_count"`
	LastEditedAt  string     `json:"last_edited_at,omitempty"`
}

// PropertyOption is a select/multi-select/status option.
type PropertyOption struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// PropertyGroup groups status options.
type PropertyGroup struct {
	Name    string   `json:"name"`
	Options []string `json:"options"`
}

// PropertyDefinition describes one database property in `database get`.
type PropertyDefinition struct {
	ID              string           `json:"id"`
	Type            string           `json:"type"`
	Options         []PropertyOption `json:"options,omitempty"`
	Groups          []PropertyGroup  `json:"groups,omitempty"`
	Prefix          string           `json:"prefix,omitempty"`
	RelatedDatabase string           `json:"related_database,omitempty"`
}

// DatabaseDetail is the full `database get` shape.
type DatabaseDetail struct {
	ID           string                        `json:"id"`
	Title        string                        `json:"title"`
	Description  string                        `json:"description,omitempty"`
	URL          string                        `json:"url"`
	Parent       *ParentRef                    `json:"parent,omitempty"`
	Properties   map[string]PropertyDefinition `json:"properties"`
	IsInline     bool                          `json:"is_inline,omitempty"`
	CreatedAt    string                        `json:"created_at,omitempty"`
	LastEditedAt string                        `json:"last_edited_at,omitempty"`
}

// SchemaProperty is one property in the flat `database schema` listing.
type SchemaProperty struct {
	Name            string              `json:"name"`
	ID              string              `json:"id"`
	Type            string              `json:"type"`
	Options         []string            `json:"options,omitempty"`
	Groups          map[string][]string `json:"groups,omitempty"`
	Prefix          string              `json:"prefix,omitempty"`
	RelatedDatabase string              `json:"related_database,omitempty"`
}

// DatabaseSchema is the `database schema` shape.
type DatabaseSchema struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Properties []SchemaProperty `json:"properties"`
}

// QueryRow is one row from `database query`, properties flattened by name.
type QueryRow struct {
	ID           string         `json:"id"`
	URL          string         `json:"url"`
	Properties   map[string]any `json:"properties"`
	CreatedAt    string         `json:"created_at,omitempty"`
	LastEditedAt string         `json:"last_edited_at,omitempty"`
}

// Icon is a page icon (emoji or hosted image).
type Icon struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji,omitempty"`
	URL   string `json:"url,omitempty"`
}

// UserRef names a user attached to a resource (author, editor).
type UserRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// PageDetail is the full `page get` shape.
type PageDetail struct {
	ID           string         `json:"id"`
	URL          string         `json:"url"`
	Parent       *ParentRef     `json:"parent,omitempty"`
	Properties   map[string]any `json:"properties"`
	Icon         *Icon          `json:"icon,omitempty"`
	CreatedAt    string         `json:"created_at,omitempty"`
	CreatedBy    *UserRef       `json:"created_by,omitempty"`
	LastEditedAt string         `json:"last_edited_at,omitempty"`
	LastEditedBy *UserRef       `json:"last_edited_by,omitempty"`
	Archived     bool           `json:"archived,omitempty"`
}

// PageCreateResult reports a created page.
type PageCreateResult struct {
	ID        string         `json:"id"`
	URL       string         `json:"url"`
	Title     string         `json:"title"`
	Parent    map[string]any `json:"parent"`
	CreatedAt string         `json:"created_at,omitempty"`
}

// PageUpdateResult reports an updated page.
type PageUpdateResult struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	LastEditedAt string `json:"last_edited_at,omitempty"`
}

// PageTrashResult reports a trashed page.
type PageTrashResult struct {
	ID      string `json:"id"`
	Trashed bool   `json:"trashed"`
}

// PageArchiveResult reports an archived page.
type PageArchiveResult struct {
	ID       string `json:"id"`
	Archived bool   `json:"archived"`
}

// NormalizedBlock is a block in the official API's type vocabulary with
// type-specific extras flattened alongside.
type NormalizedBlock struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	RichText    string `json:"rich_text"`
	HasChildren bool   `json:"has_children"`
	// Type-specific fields.
	Checked    *bool  `json:"checked,omitempty"`
	Language   string `json:"language,omitempty"`
	URL        string `json:"url,omitempty"`
	Caption    string `json:"caption,omitempty"`
	Emoji      string `json:"emoji,omitempty"`
	Title      string `json:"title,omitempty"`
	Expression string `json:"expression,omitempty"`
	// Table fields. TableWidth and HasColumnHeader are set on `table` blocks;
	// Cells (one flattened string per column) is set on each `table_row` child.
	TableWidth      int      `json:"table_width,omitempty"`
	HasColumnHeader bool     `json:"has_column_header,omitempty"`
	Cells           []string `json:"cells,omitempty"`
}

// BlockListResult is a page of child blocks.
type BlockListResult struct {
	Blocks  []NormalizedBlock `json:"blocks"`
	HasMore bool              `json:"has_more"`
}

// BlockUpdateResult reports an updated block.
type BlockUpdateResult struct {
	ID           string `json:"id"`
	LastEditedAt string `json:"last_edited_at,omitempty"`
}

// BlockDeleteResult reports a deleted block.
type BlockDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// BlockMoveResult reports a moved block.
type BlockMoveResult struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	AfterID  string `json:"after_id,omitempty"`
}

// CommentItem is one comment in `comment list`.
type CommentItem struct {
	ID         string   `json:"id"`
	Body       string   `json:"body"`
	Author     *UserRef `json:"author,omitempty"`
	CreatedAt  string   `json:"created_at,omitempty"`
	AnchorText string   `json:"anchor_text,omitempty"`
}

// CommentCreateResult reports an added comment.
type CommentCreateResult struct {
	ID           string `json:"id"`
	DiscussionID string `json:"discussion_id,omitempty"`
	Body         string `json:"body"`
	CreatedAt    string `json:"created_at,omitempty"`
}

// UserItem is one user in `user list`.
type UserItem struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"` // person | bot
	Email     string `json:"email,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// UserMe is the `user me` shape.
type UserMe struct {
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	Type          string `json:"type"` // person | bot
	WorkspaceName string `json:"workspace_name,omitempty"`
}
