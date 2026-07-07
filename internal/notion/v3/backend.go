// V3 backend — implements notion.Backend over the v3 internal API Client.
// Reads and writes go through saveTransactions; comment orchestration lives in
// comments_client.go.

package v3

import (
	"context"
	"fmt"
	"time"

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

// --- Databases ---

// ListDatabases finds database blocks via search and pairs each with its
// collection.
func (b *Backend) ListDatabases(ctx context.Context, params notion.ListParams) (notion.Paginated[notion.DatabaseListItem], error) {
	resp, err := b.Client.Search(ctx, SearchParams{Query: "", Limit: orInt(params.Limit, 50)})
	if err != nil {
		return notion.Paginated[notion.DatabaseListItem]{}, err
	}

	items := []notion.DatabaseListItem{}
	for _, hit := range resp.Results {
		block, ok := resp.RecordMap.GetBlock(hit.ID)
		if !ok || !isDatabaseBlock(block) {
			continue
		}

		var collection *Collection
		if collectionID := blockCollectionID(resp.RecordMap, block.ID); collectionID != "" {
			if c, ok := resp.RecordMap.GetCollection(collectionID); ok {
				collection = c
			}
		} else if c, ok := resp.RecordMap.FirstCollection(); ok {
			collection = c
		}
		if collection != nil {
			items = append(items, ToDatabaseListItem(collection, block.ID))
		}
	}

	return notion.Paginated[notion.DatabaseListItem]{Items: items, HasMore: false}, nil
}

// GetDatabase resolves a database's collection and returns its detail.
func (b *Backend) GetDatabase(ctx context.Context, id string) (notion.DatabaseDetail, error) {
	resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: id, Limit: 1})
	if err != nil {
		return notion.DatabaseDetail{}, err
	}
	collection, err := b.resolveCollection(ctx, id, resp.RecordMap)
	if err != nil {
		return notion.DatabaseDetail{}, err
	}
	if collection == nil {
		return notion.DatabaseDetail{}, fmt.Errorf("Database not found: %s", id)
	}
	return ToDatabaseDetail(collection, id), nil
}

// QueryDatabase resolves the collection + view, runs the query, and maps rows.
func (b *Backend) QueryDatabase(ctx context.Context, params notion.QueryDatabaseParams) (notion.Paginated[notion.QueryRow], error) {
	empty := notion.Paginated[notion.QueryRow]{}

	pageResp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: params.ID, Limit: 1})
	if err != nil {
		return empty, err
	}
	collection, err := b.resolveCollection(ctx, params.ID, pageResp.RecordMap)
	if err != nil {
		return empty, err
	}
	if collection == nil {
		return empty, fmt.Errorf("Database not found: %s", params.ID)
	}

	viewID := ""
	if vids := blockViewIDs(pageResp.RecordMap, params.ID); len(vids) > 0 {
		viewID = vids[0]
	} else if vid, ok := pageResp.RecordMap.FirstCollectionViewID(); ok {
		viewID = vid
	}
	if viewID == "" {
		return empty, fmt.Errorf("No view found for database: %s", params.ID)
	}

	qp := QueryCollectionParams{
		CollectionID:     collection.ID,
		CollectionViewID: viewID,
		Limit:            orInt(params.Limit, 50),
	}
	if params.Filter != nil || params.Sort != nil {
		qp.Filter = params.Filter
		qp.Sort = params.Sort
	}

	result, err := b.Client.QueryCollection(ctx, qp)
	if err != nil {
		return empty, err
	}

	items := []notion.QueryRow{}
	for _, blockID := range result.BlockIDs {
		rowBlock, ok := result.RecordMap.GetBlock(blockID)
		if !ok {
			continue
		}
		items = append(items, ToQueryRow(rowBlock, collection.Schema))
	}

	return notion.Paginated[notion.QueryRow]{Items: items, HasMore: false}, nil
}

// GetDatabaseSchema resolves a database's collection and returns its schema.
func (b *Backend) GetDatabaseSchema(ctx context.Context, id string) (notion.DatabaseSchema, error) {
	resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: id, Limit: 1})
	if err != nil {
		return notion.DatabaseSchema{}, err
	}
	collection, err := b.resolveCollection(ctx, id, resp.RecordMap)
	if err != nil {
		return notion.DatabaseSchema{}, err
	}
	if collection == nil {
		return notion.DatabaseSchema{}, fmt.Errorf("Database not found: %s", id)
	}
	return ToDatabaseSchema(collection, id), nil
}

// --- Pages ---

// GetPage returns a page's detail, resolving the collection schema for database
// rows so property names are human-readable.
func (b *Backend) GetPage(ctx context.Context, id string) (notion.PageDetail, error) {
	resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: id, Limit: 1})
	if err != nil {
		return notion.PageDetail{}, err
	}
	block, ok := resp.RecordMap.GetBlock(id)
	if !ok {
		return notion.PageDetail{}, fmt.Errorf("Page not found: %s", id)
	}

	var schema map[string]PropertySchema
	if block.ParentTable == "collection" {
		if collection, ok := resp.RecordMap.GetCollection(block.ParentID); ok {
			schema = collection.Schema
		}
	}
	return ToPageDetail(block, schema), nil
}

// CreatePage creates a page under a page or database parent.
func (b *Backend) CreatePage(ctx context.Context, params notion.CreatePageParams) (notion.PageCreateResult, error) {
	empty := notion.PageCreateResult{}
	spaceID, userID := b.Client.SpaceID, b.Client.UserID
	newPageID := newUUID()

	resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: params.ParentID, Limit: 1})
	if err != nil {
		return empty, err
	}
	parentBlock, _ := resp.RecordMap.GetBlock(params.ParentID)
	isDB := parentBlock != nil && isDatabaseBlock(parentBlock)

	v3Props := map[string]any{"title": NewRichText(params.Title)}
	var parentTable, parentID string
	if isDB {
		collection, err := b.resolveCollection(ctx, params.ParentID, resp.RecordMap)
		if err != nil {
			return empty, err
		}
		if collection == nil {
			return empty, fmt.Errorf("Could not resolve database: %s", params.ParentID)
		}
		parentTable, parentID = "collection", collection.ID
		if params.Properties != nil {
			mapped, err := MapPropertiesToSchema(params.Properties, collection.Schema)
			if err != nil {
				return empty, err
			}
			for k, v := range mapped {
				v3Props[k] = v
			}
		}
	} else {
		parentTable, parentID = "block", params.ParentID
	}

	var format map[string]any
	if params.Icon != "" {
		format = map[string]any{"page_icon": params.Icon}
	}

	// For database parents, the content listAfter + parent editMeta target
	// the collection_view_page block, while the block itself names the
	// collection as parent.
	var listParent *Pointer
	if isDB {
		listParent = &Pointer{Table: "block", ID: params.ParentID, SpaceID: spaceID}
	}

	now := time.Now()
	ops := CreateBlockOps(CreateBlockParams{
		ID:          newPageID,
		Type:        "page",
		ParentID:    parentID,
		ParentTable: parentTable,
		SpaceID:     spaceID,
		UserID:      userID,
		Properties:  v3Props,
		Format:      format,
		ListParent:  listParent,
	}, now)

	if err := b.Client.SaveTransactions(ctx, ops); err != nil {
		return empty, err
	}

	parent := map[string]any{"type": "page_id", "page_id": params.ParentID}
	if isDB {
		parent = map[string]any{"type": "database_id", "database_id": params.ParentID}
	}
	return notion.PageCreateResult{
		ID:        newPageID,
		URL:       notionURL(newPageID),
		Title:     params.Title,
		Parent:    parent,
		CreatedAt: MsToISO(now.UnixMilli()),
	}, nil
}

// UpdatePage edits a page's title, icon, and/or properties.
func (b *Backend) UpdatePage(ctx context.Context, params notion.UpdatePageParams) (notion.PageUpdateResult, error) {
	empty := notion.PageUpdateResult{}
	spaceID, userID := b.Client.SpaceID, b.Client.UserID

	v3Props := map[string]any{}
	v3Format := map[string]any{}
	if params.Title != "" {
		v3Props["title"] = NewRichText(params.Title)
	}
	if params.Icon != "" {
		v3Format["page_icon"] = params.Icon
	}

	if params.Properties != nil {
		resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: params.ID, Limit: 1})
		if err != nil {
			return empty, err
		}
		if block, ok := resp.RecordMap.GetBlock(params.ID); ok && block.ParentTable == "collection" {
			var schema map[string]PropertySchema
			if collection, ok := resp.RecordMap.GetCollection(block.ParentID); ok {
				schema = collection.Schema
			}
			mapped, err := MapPropertiesToSchema(params.Properties, schema)
			if err != nil {
				return empty, err
			}
			for k, v := range mapped {
				v3Props[k] = v
			}
		}
	}

	now := time.Now()
	up := UpdatePropertyParams{ID: params.ID, SpaceID: spaceID, UserID: userID}
	if len(v3Props) > 0 {
		up.Properties = v3Props
	}
	if len(v3Format) > 0 {
		up.Format = v3Format
	}
	ops := UpdatePropertyOps(up, now)

	if err := b.Client.SaveTransactions(ctx, ops); err != nil {
		return empty, err
	}
	return notion.PageUpdateResult{ID: params.ID, URL: notionURL(params.ID), LastEditedAt: MsToISO(now.UnixMilli())}, nil
}

// TrashPage moves a page to Trash.
func (b *Backend) TrashPage(ctx context.Context, id string) (notion.PageTrashResult, error) {
	empty := notion.PageTrashResult{}
	resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: id, Limit: 1})
	if err != nil {
		return empty, err
	}
	block, ok := resp.RecordMap.GetBlock(id)
	if !ok {
		return empty, fmt.Errorf("Page not found: %s", id)
	}

	ops := TrashBlockOps(TrashBlockParams{
		ID:          id,
		ParentID:    block.ParentID,
		ParentTable: block.ParentTable,
		SpaceID:     b.Client.SpaceID,
		UserID:      b.Client.UserID,
	}, time.Now())

	if err := b.Client.SaveTransactions(ctx, ops); err != nil {
		return empty, err
	}
	return notion.PageTrashResult{ID: id, Trashed: true}, nil
}

// RestorePage restores a trashed page.
func (b *Backend) RestorePage(ctx context.Context, id string) (notion.PageTrashResult, error) {
	if _, err := b.Client.RestoreRecord(ctx, RestoreRecordParams{ID: id, Table: "block"}); err != nil {
		return notion.PageTrashResult{}, err
	}
	return notion.PageTrashResult{ID: id, Trashed: false}, nil
}

// ArchivePage sets the real Archive state on a page.
func (b *Backend) ArchivePage(ctx context.Context, id string) (notion.PageArchiveResult, error) {
	return b.setArchived(ctx, id, true)
}

// UnarchivePage clears the real Archive state on a page.
func (b *Backend) UnarchivePage(ctx context.Context, id string) (notion.PageArchiveResult, error) {
	return b.setArchived(ctx, id, false)
}

func (b *Backend) setArchived(ctx context.Context, id string, archive bool) (notion.PageArchiveResult, error) {
	ops := ArchivePageOps(ArchivePageParams{
		ID:      id,
		SpaceID: b.Client.SpaceID,
		UserID:  b.Client.UserID,
		Archive: archive,
	}, time.Now())
	if err := b.Client.SaveTransactions(ctx, ops); err != nil {
		return notion.PageArchiveResult{}, err
	}
	return notion.PageArchiveResult{ID: id, Archived: archive}, nil
}

// --- Blocks ---

// ListBlocks returns a block's direct, alive children.
func (b *Backend) ListBlocks(ctx context.Context, params notion.ListBlocksParams) (notion.Paginated[notion.NormalizedBlock], error) {
	resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: params.ID, Limit: orInt(params.Limit, 50)})
	if err != nil {
		return notion.Paginated[notion.NormalizedBlock]{}, err
	}

	var childIDs []string
	if parent, ok := resp.RecordMap.GetBlock(params.ID); ok {
		childIDs = parent.Content
	}

	items := []notion.NormalizedBlock{}
	for _, childID := range childIDs {
		child, ok := resp.RecordMap.GetBlock(childID)
		if !ok || !child.IsAlive() {
			continue
		}
		items = append(items, NormalizeBlock(child))
	}

	return notion.Paginated[notion.NormalizedBlock]{Items: items, HasMore: false}, nil
}

// GetAllBlocks walks page chunks, collecting alive descendant blocks up to 1000.
func (b *Backend) GetAllBlocks(ctx context.Context, id string) (notion.BlockListResult, error) {
	blocks := []notion.NormalizedBlock{}
	seen := map[string]bool{}
	var cursor *Cursor
	chunkNumber := 0

	for len(blocks) < 1000 {
		resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: id, Limit: 100, Cursor: cursor, ChunkNumber: chunkNumber})
		if err != nil {
			return notion.BlockListResult{}, err
		}
		rm := resp.RecordMap

		var childIDs []string
		if parent, ok := rm.GetBlock(id); ok {
			childIDs = parent.Content
		}
		childIDSet := make(map[string]bool, len(childIDs))
		for _, c := range childIDs {
			childIDSet[c] = true
		}

		for _, childID := range childIDs {
			child, ok := rm.GetBlock(childID)
			if !ok || !child.IsAlive() {
				continue
			}
			if !seen[child.ID] {
				seen[child.ID] = true
				blocks = append(blocks, NormalizeBlock(child))
			}
		}

		// Also include descendant blocks present in the record map.
		for _, block := range rm.AllBlocks() {
			if block.ID == id || seen[block.ID] {
				continue
			}
			if childIDSet[block.ID] || block.ParentID == id {
				seen[block.ID] = true
				blocks = append(blocks, NormalizeBlock(block))
			}
		}

		if len(resp.Cursor.Stack) == 0 || len(blocks) >= 1000 {
			break
		}
		next := resp.Cursor
		cursor = &next
		chunkNumber++
	}

	return notion.BlockListResult{Blocks: blocks, HasMore: len(blocks) >= 1000}, nil
}

// GetChildBlocks fetches all blocks under each given block ID.
func (b *Backend) GetChildBlocks(ctx context.Context, blockIDs []string) (map[string][]notion.NormalizedBlock, error) {
	childMap := make(map[string][]notion.NormalizedBlock, len(blockIDs))
	for _, blockID := range blockIDs {
		result, err := b.GetAllBlocks(ctx, blockID)
		if err != nil {
			return nil, err
		}
		childMap[blockID] = result.Blocks
	}
	return childMap, nil
}

// AppendBlocks converts official API block objects to v3 and appends them,
// chaining ordering via the previous block and emitting a single parent
// editMeta at the end.
func (b *Backend) AppendBlocks(ctx context.Context, params notion.AppendBlocksParams) (notion.AppendBlocksResult, error) {
	spaceID, userID := b.Client.SpaceID, b.Client.UserID
	now := time.Now()

	var allOps []Operation
	previousBlockID := ""

	for _, raw := range params.Blocks {
		blockObj, _ := raw.(map[string]any)
		newBlockID := newUUID()
		args := OfficialBlockToV3Args(blockObj)

		// Chain siblings with AfterID and skip the per-block parent
		// editMeta; one trailing editMeta covers the whole batch.
		allOps = append(allOps, CreateBlockOps(CreateBlockParams{
			ID:                 newBlockID,
			Type:               args.Type,
			ParentID:           params.ID,
			ParentTable:        "block",
			SpaceID:            spaceID,
			UserID:             userID,
			Properties:         args.Properties,
			Format:             args.Format,
			AfterID:            previousBlockID,
			SkipParentEditMeta: true,
		}, now)...)

		previousBlockID = newBlockID
	}

	if len(params.Blocks) > 0 {
		allOps = append(allOps, editMetaOp(ptr("block", params.ID, spaceID), userID, now))
	}

	if err := b.Client.SaveTransactions(ctx, allOps); err != nil {
		return notion.AppendBlocksResult{}, err
	}
	return notion.AppendBlocksResult{BlocksAdded: len(params.Blocks)}, nil
}

// UpdateBlock updates a block's text content.
func (b *Backend) UpdateBlock(ctx context.Context, params notion.UpdateBlockParams) (notion.BlockUpdateResult, error) {
	empty := notion.BlockUpdateResult{}
	spaceID, userID := b.Client.SpaceID, b.Client.UserID

	resp, err := b.Client.SyncRecordValues(ctx, []SyncRequest{{Pointer: SyncPointer{ID: params.ID, Table: "block"}, Version: -1}})
	if err != nil {
		return empty, err
	}
	if _, ok := resp.RecordMap.GetBlock(params.ID); !ok {
		return empty, fmt.Errorf("Block not found: %s", params.ID)
	}

	v3Props := map[string]any{}
	if params.Content != nil {
		v3Props["title"] = NewRichText(*params.Content)
	}

	now := time.Now()
	up := UpdatePropertyParams{ID: params.ID, SpaceID: spaceID, UserID: userID}
	if len(v3Props) > 0 {
		up.Properties = v3Props
	}
	ops := UpdatePropertyOps(up, now)

	if err := b.Client.SaveTransactions(ctx, ops); err != nil {
		return empty, err
	}
	return notion.BlockUpdateResult{ID: params.ID, LastEditedAt: MsToISO(now.UnixMilli())}, nil
}

// DeleteBlock moves a block to Trash.
func (b *Backend) DeleteBlock(ctx context.Context, id string) (notion.BlockDeleteResult, error) {
	empty := notion.BlockDeleteResult{}
	spaceID, userID := b.Client.SpaceID, b.Client.UserID

	resp, err := b.Client.SyncRecordValues(ctx, []SyncRequest{{Pointer: SyncPointer{ID: id, Table: "block"}, Version: -1}})
	if err != nil {
		return empty, err
	}
	block, ok := resp.RecordMap.GetBlock(id)
	if !ok {
		return empty, fmt.Errorf("Block not found: %s", id)
	}

	ops := TrashBlockOps(TrashBlockParams{
		ID:          id,
		ParentID:    block.ParentID,
		ParentTable: block.ParentTable,
		SpaceID:     spaceID,
		UserID:      userID,
	}, time.Now())

	if err := b.Client.SaveTransactions(ctx, ops); err != nil {
		return empty, err
	}
	return notion.BlockDeleteResult{ID: id, Deleted: true}, nil
}

// MoveBlock moves a block within or across parents.
func (b *Backend) MoveBlock(ctx context.Context, params notion.MoveBlockParams) (notion.BlockMoveResult, error) {
	empty := notion.BlockMoveResult{}
	spaceID, userID := b.Client.SpaceID, b.Client.UserID

	resp, err := b.Client.SyncRecordValues(ctx, []SyncRequest{{Pointer: SyncPointer{ID: params.ID, Table: "block"}, Version: -1}})
	if err != nil {
		return empty, err
	}
	block, ok := resp.RecordMap.GetBlock(params.ID)
	if !ok {
		return empty, fmt.Errorf("Block not found: %s", params.ID)
	}

	newParentID := block.ParentID
	newParentTable := block.ParentTable
	if params.ParentID != "" {
		newParentID = params.ParentID
		newParentTable = "block"
	}

	ops := MoveBlockOps(MoveBlockParams{
		ID:             params.ID,
		OldParentID:    block.ParentID,
		OldParentTable: block.ParentTable,
		NewParentID:    newParentID,
		NewParentTable: newParentTable,
		SpaceID:        spaceID,
		UserID:         userID,
		AfterID:        params.AfterID,
	}, time.Now())

	if err := b.Client.SaveTransactions(ctx, ops); err != nil {
		return empty, err
	}
	return notion.BlockMoveResult{ID: params.ID, ParentID: newParentID, AfterID: params.AfterID}, nil
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

// --- Utility ---

// IsDatabase reports whether the ID refers to a database block. Any error
// (e.g. not found) yields false, matching the TS behavior.
func (b *Backend) IsDatabase(ctx context.Context, id string) (bool, error) {
	resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: id, Limit: 1})
	if err != nil {
		return false, nil
	}
	block, ok := resp.RecordMap.GetBlock(id)
	if !ok {
		return false, nil
	}
	return isDatabaseBlock(block), nil
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
