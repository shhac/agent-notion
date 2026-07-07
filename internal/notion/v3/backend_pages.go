// V3 backend — Pages section: get, create, update, trash/restore, and the real
// archive/unarchive operations.

package v3

import (
	"context"
	"fmt"
	"time"

	"github.com/shhac/agent-notion/internal/notion"
)

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
