// Endpoint methods for the official REST client. Each mirrors a method of the
// retired TS OfficialBackend (see git history), returning the
// normalized types from package notion.

package official

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/shhac/agent-notion/internal/notion"
)

// defaultPageSize matches the TS `?? 50` fallback.
const defaultPageSize = 50

func pageSize(limit int) int {
	if limit <= 0 {
		return defaultPageSize
	}
	return limit
}

// fetchList runs a list request, decodes the shared paginated envelope, applies
// transform to each raw result, and wraps the page. For POST endpoints body
// carries the request payload; for GET endpoints body is nil and path already
// includes the query string.
func fetchList[T any](ctx context.Context, c Client, method, path string, body any, transform func(map[string]any) T) (notion.Paginated[T], error) {
	var resp paginatedRaw
	if err := c.do(ctx, method, path, body, &resp); err != nil {
		return notion.Paginated[T]{}, err
	}
	items := make([]T, 0, len(resp.Results))
	for _, r := range resp.Results {
		items = append(items, transform(r))
	}
	return notion.Paginated[T]{Items: items, HasMore: resp.HasMore, NextCursor: resp.cursor()}, nil
}

// listQuery builds the page_size (+ optional start_cursor) query shared by the
// GET list endpoints. Callers add any endpoint-specific keys before Encode.
func listQuery(limit int, cursor string) url.Values {
	q := url.Values{}
	q.Set("page_size", strconv.Itoa(pageSize(limit)))
	if cursor != "" {
		q.Set("start_cursor", cursor)
	}
	return q
}

// --- Search ---

// Search runs a workspace search (POST /v1/search).
func (c Client) Search(ctx context.Context, p notion.SearchParams) (notion.Paginated[notion.SearchResult], error) {
	body := map[string]any{
		"query":     p.Query,
		"page_size": pageSize(p.Limit),
	}
	if p.Filter != "" {
		body["filter"] = map[string]any{"property": "object", "value": p.Filter}
	}
	if p.Cursor != "" {
		body["start_cursor"] = p.Cursor
	}
	return fetchList(ctx, c, http.MethodPost, "/v1/search", body, transformSearchResult)
}

// --- Databases ---

// ListDatabases lists databases via search filtered to object=database.
func (c Client) ListDatabases(ctx context.Context, p notion.ListParams) (notion.Paginated[notion.DatabaseListItem], error) {
	body := map[string]any{
		"filter":    map[string]any{"property": "object", "value": "database"},
		"page_size": pageSize(p.Limit),
	}
	if p.Cursor != "" {
		body["start_cursor"] = p.Cursor
	}
	return fetchList(ctx, c, http.MethodPost, "/v1/search", body, transformDatabaseListItem)
}

// GetDatabase retrieves a database's full detail (GET /v1/databases/{id}).
func (c Client) GetDatabase(ctx context.Context, id string) (notion.DatabaseDetail, error) {
	var db map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/databases/"+id, nil, &db); err != nil {
		return notion.DatabaseDetail{}, err
	}
	return transformDatabaseDetail(db), nil
}

// GetDatabaseSchema retrieves a database and returns its flat schema.
func (c Client) GetDatabaseSchema(ctx context.Context, id string) (notion.DatabaseSchema, error) {
	var db map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/databases/"+id, nil, &db); err != nil {
		return notion.DatabaseSchema{}, err
	}
	return transformDatabaseSchema(db), nil
}

// QueryDatabase runs a database query (POST /v1/databases/{id}/query).
func (c Client) QueryDatabase(ctx context.Context, p notion.QueryDatabaseParams) (notion.Paginated[notion.QueryRow], error) {
	body := map[string]any{"page_size": pageSize(p.Limit)}
	if p.Filter != nil {
		body["filter"] = p.Filter
	}
	if p.Sort != nil {
		body["sorts"] = p.Sort
	}
	if p.Cursor != "" {
		body["start_cursor"] = p.Cursor
	}
	return fetchList(ctx, c, http.MethodPost, "/v1/databases/"+p.ID+"/query", body, transformQueryRow)
}

// --- Pages ---

// GetPage retrieves a page's detail (GET /v1/pages/{id}).
func (c Client) GetPage(ctx context.Context, id string) (notion.PageDetail, error) {
	var page map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/pages/"+id, nil, &page); err != nil {
		return notion.PageDetail{}, err
	}
	return transformPageDetail(page), nil
}

// CreatePage creates a page (POST /v1/pages). Whether the parent is a database
// or a page decides the property shape; it probes with isDatabase.
func (c Client) CreatePage(ctx context.Context, p notion.CreatePageParams) (notion.PageCreateResult, error) {
	body := map[string]any{}
	if c.isDatabase(ctx, p.ParentID) {
		body["parent"] = map[string]any{"database_id": p.ParentID}
		body["properties"] = buildDatabaseProperties(p.Title, p.Properties)
	} else {
		body["parent"] = map[string]any{"page_id": p.ParentID}
		body["properties"] = map[string]any{"title": titleValue(p.Title)}
	}
	if p.Icon != "" {
		body["icon"] = emojiIcon(p.Icon)
	}

	var resp map[string]any
	if err := c.do(ctx, http.MethodPost, "/v1/pages", body, &resp); err != nil {
		return notion.PageCreateResult{}, err
	}
	return notion.PageCreateResult{
		ID:        mstr(resp, "id"),
		URL:       mstr(resp, "url"),
		Title:     p.Title,
		Parent:    mmap(resp, "parent"),
		CreatedAt: mstr(resp, "created_time"),
	}, nil
}

// UpdatePage updates a page's title, properties, and/or icon
// (PATCH /v1/pages/{id}).
func (c Client) UpdatePage(ctx context.Context, p notion.UpdatePageParams) (notion.PageUpdateResult, error) {
	body := map[string]any{}
	if p.Title != "" || len(p.Properties) > 0 {
		props := map[string]any{}
		if p.Title != "" {
			props["title"] = titleValue(p.Title)
		}
		for _, key := range sortedKeys(p.Properties) {
			if key == "Name" || key == "title" {
				continue
			}
			props[key] = buildPropertyValue(p.Properties[key])
		}
		body["properties"] = props
	}
	if p.Icon != "" {
		body["icon"] = emojiIcon(p.Icon)
	}

	var resp map[string]any
	if err := c.do(ctx, http.MethodPatch, "/v1/pages/"+p.ID, body, &resp); err != nil {
		return notion.PageUpdateResult{}, err
	}
	return notion.PageUpdateResult{
		ID:           mstr(resp, "id"),
		URL:          mstr(resp, "url"),
		LastEditedAt: mstr(resp, "last_edited_time"),
	}, nil
}

// TrashPage moves a page to Trash (PATCH /v1/pages/{id} archived=true).
func (c Client) TrashPage(ctx context.Context, id string) (notion.PageTrashResult, error) {
	if err := c.do(ctx, http.MethodPatch, "/v1/pages/"+id, map[string]any{"archived": true}, nil); err != nil {
		return notion.PageTrashResult{}, err
	}
	return notion.PageTrashResult{ID: id, Trashed: true}, nil
}

// RestorePage restores a trashed page (PATCH /v1/pages/{id} archived=false).
func (c Client) RestorePage(ctx context.Context, id string) (notion.PageTrashResult, error) {
	if err := c.do(ctx, http.MethodPatch, "/v1/pages/"+id, map[string]any{"archived": false}, nil); err != nil {
		return notion.PageTrashResult{}, err
	}
	return notion.PageTrashResult{ID: id, Trashed: false}, nil
}

// isDatabase reports whether id resolves to a database. Any error (including a
// 404 for a page id) is treated as "not a database", matching the TS try/catch.
func (c Client) isDatabase(ctx context.Context, id string) bool {
	return c.do(ctx, http.MethodGet, "/v1/databases/"+id, nil, nil) == nil
}

// --- Blocks ---

// ListBlocks lists one page of a block's children
// (GET /v1/blocks/{id}/children).
func (c Client) ListBlocks(ctx context.Context, p notion.ListBlocksParams) (notion.Paginated[notion.NormalizedBlock], error) {
	q := listQuery(p.Limit, p.Cursor)
	return fetchList(ctx, c, http.MethodGet, "/v1/blocks/"+p.ID+"/children?"+q.Encode(), nil, normalizeBlock)
}

// maxAllBlocks caps GetAllBlocks, matching the TS 1000-block ceiling.
const maxAllBlocks = 1000

// GetAllBlocks pages through a block's children up to maxAllBlocks, reporting
// HasMore when the ceiling is hit before exhausting the list.
func (c Client) GetAllBlocks(ctx context.Context, id string) (notion.BlockListResult, error) {
	blocks := []notion.NormalizedBlock{}
	cursor := ""
	hasMore := false

	for len(blocks) < maxAllBlocks {
		q := url.Values{}
		q.Set("page_size", "100")
		if cursor != "" {
			q.Set("start_cursor", cursor)
		}

		var resp paginatedRaw
		if err := c.do(ctx, http.MethodGet, "/v1/blocks/"+id+"/children?"+q.Encode(), nil, &resp); err != nil {
			return notion.BlockListResult{}, err
		}
		for _, b := range resp.Results {
			blocks = append(blocks, normalizeBlock(b))
		}

		if !resp.HasMore {
			break
		}
		if len(blocks) >= maxAllBlocks {
			hasMore = true
			break
		}
		cursor = resp.cursor()
	}

	return notion.BlockListResult{Blocks: blocks, HasMore: hasMore}, nil
}

// AppendBlocks appends children to a block and reports how many were added
// (PATCH /v1/blocks/{id}/children).
func (c Client) AppendBlocks(ctx context.Context, p notion.AppendBlocksParams) (notion.AppendBlocksResult, error) {
	var resp struct {
		Results []any `json:"results"`
	}
	if err := c.do(ctx, http.MethodPatch, "/v1/blocks/"+p.ID+"/children", map[string]any{"children": p.Blocks}, &resp); err != nil {
		return notion.AppendBlocksResult{}, err
	}
	return notion.AppendBlocksResult{BlocksAdded: len(resp.Results)}, nil
}

// UpdateBlock updates a block's rich text (PATCH /v1/blocks/{id}). When Type is
// empty it first retrieves the block to learn its type.
func (c Client) UpdateBlock(ctx context.Context, p notion.UpdateBlockParams) (notion.BlockUpdateResult, error) {
	blockType := p.Type
	if blockType == "" {
		var existing map[string]any
		if err := c.do(ctx, http.MethodGet, "/v1/blocks/"+p.ID, nil, &existing); err != nil {
			return notion.BlockUpdateResult{}, err
		}
		blockType = mstr(existing, "type")
	}

	body := map[string]any{}
	if p.Content != nil {
		body[blockType] = map[string]any{"rich_text": richTextValue(*p.Content)}
	}

	var resp map[string]any
	if err := c.do(ctx, http.MethodPatch, "/v1/blocks/"+p.ID, body, &resp); err != nil {
		return notion.BlockUpdateResult{}, err
	}
	return notion.BlockUpdateResult{ID: mstr(resp, "id"), LastEditedAt: mstr(resp, "last_edited_time")}, nil
}

// DeleteBlock deletes a block (DELETE /v1/blocks/{id}).
func (c Client) DeleteBlock(ctx context.Context, id string) (notion.BlockDeleteResult, error) {
	if err := c.do(ctx, http.MethodDelete, "/v1/blocks/"+id, nil, nil); err != nil {
		return notion.BlockDeleteResult{}, err
	}
	return notion.BlockDeleteResult{ID: id, Deleted: true}, nil
}

// --- Comments ---

// ListComments lists comments on a page/block (GET /v1/comments).
func (c Client) ListComments(ctx context.Context, p notion.ListCommentsParams) (notion.Paginated[notion.CommentItem], error) {
	q := listQuery(p.Limit, p.Cursor)
	q.Set("block_id", p.PageID)
	return fetchList(ctx, c, http.MethodGet, "/v1/comments?"+q.Encode(), nil, transformComment)
}

// AddComment adds a top-level comment to a page (POST /v1/comments). The
// returned body falls back to the request text when the response omits it.
func (c Client) AddComment(ctx context.Context, p notion.AddCommentParams) (notion.CommentCreateResult, error) {
	reqBody := map[string]any{
		"parent":    map[string]any{"page_id": p.PageID},
		"rich_text": richTextValue(p.Body),
	}

	var resp map[string]any
	if err := c.do(ctx, http.MethodPost, "/v1/comments", reqBody, &resp); err != nil {
		return notion.CommentCreateResult{}, err
	}
	text := richTextToPlain(mslice(resp, "rich_text"))
	if text == "" {
		text = p.Body
	}
	return notion.CommentCreateResult{
		ID:        mstr(resp, "id"),
		Body:      text,
		CreatedAt: mstr(resp, "created_time"),
	}, nil
}

// --- Users ---

// ListUsers lists workspace users (GET /v1/users).
func (c Client) ListUsers(ctx context.Context, p notion.ListParams) (notion.Paginated[notion.UserItem], error) {
	q := listQuery(p.Limit, p.Cursor)
	return fetchList(ctx, c, http.MethodGet, "/v1/users?"+q.Encode(), nil, transformUser)
}

// GetMe returns the current bot/user identity as a UserMe (GET /v1/users/me).
// This is the normalized cousin of Me, which returns the leaner Bot used by
// `auth import`.
func (c Client) GetMe(ctx context.Context) (notion.UserMe, error) {
	var u map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/users/me", nil, &u); err != nil {
		return notion.UserMe{}, err
	}
	return notion.UserMe{
		ID:            mstr(u, "id"),
		Name:          mstr(u, "name"),
		Type:          mstr(u, "type"),
		WorkspaceName: mstr(mmap(u, "bot"), "workspace_name"),
	}, nil
}

// --- Property value building (for create/update) ---

func buildDatabaseProperties(title string, extra map[string]any) map[string]any {
	props := map[string]any{"Name": titleValue(title)}
	for _, key := range sortedKeys(extra) {
		if key == "Name" || key == "title" {
			continue
		}
		props[key] = buildPropertyValue(extra[key])
	}
	return props
}

// buildPropertyValue maps a plain Go value to a Notion property payload,
// mirroring the TS type switch. JSON numbers arrive as float64; ints are
// accepted too for callers building values directly.
func buildPropertyValue(value any) any {
	switch v := value.(type) {
	case string:
		return map[string]any{"select": map[string]any{"name": v}}
	case bool:
		return map[string]any{"checkbox": v}
	case float64:
		return map[string]any{"number": v}
	case int:
		return map[string]any{"number": v}
	case []any:
		opts := make([]any, len(v))
		for i, item := range v {
			opts[i] = map[string]any{"name": fmt.Sprint(item)}
		}
		return map[string]any{"multi_select": opts}
	default:
		return value
	}
}

// titleValue builds a title property payload.
func titleValue(content string) map[string]any {
	return map[string]any{"title": []any{map[string]any{"text": map[string]any{"content": content}}}}
}

// richTextValue builds the single-run rich_text array the API expects.
func richTextValue(content string) []any {
	return []any{map[string]any{"type": "text", "text": map[string]any{"content": content}}}
}

// emojiIcon builds an emoji icon payload.
func emojiIcon(emoji string) map[string]any {
	return map[string]any{"type": "emoji", "emoji": emoji}
}

// sortedKeys returns a map's keys in sorted order for deterministic iteration
// (Go maps are unordered; the repo prefers stable output over JS insertion
// order).
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
