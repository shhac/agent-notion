// V3 backend — Databases section: list, get, query, and schema, plus the
// requireCollection resolver shared by them.

package v3

import (
	"context"
	"fmt"

	"github.com/shhac/agent-notion/internal/notion"
)

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

// requireCollection loads a database block and resolves its collection,
// failing with the shared not-found message. The record map is returned for
// callers that also need view IDs.
func (b *Backend) requireCollection(ctx context.Context, id string) (*Collection, RecordMap, error) {
	resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: id, Limit: 1})
	if err != nil {
		return nil, nil, err
	}
	collection, err := b.resolveCollection(ctx, id, resp.RecordMap)
	if err != nil {
		return nil, nil, err
	}
	if collection == nil {
		return nil, nil, fmt.Errorf("Database not found: %s", id)
	}
	return collection, resp.RecordMap, nil
}

// GetDatabase resolves a database's collection and returns its detail.
func (b *Backend) GetDatabase(ctx context.Context, id string) (notion.DatabaseDetail, error) {
	collection, _, err := b.requireCollection(ctx, id)
	if err != nil {
		return notion.DatabaseDetail{}, err
	}
	return ToDatabaseDetail(collection, id), nil
}

// QueryDatabase resolves the collection + view, runs the query, and maps rows.
func (b *Backend) QueryDatabase(ctx context.Context, params notion.QueryDatabaseParams) (notion.Paginated[notion.QueryRow], error) {
	empty := notion.Paginated[notion.QueryRow]{}

	collection, rm, err := b.requireCollection(ctx, params.ID)
	if err != nil {
		return empty, err
	}

	viewID := ""
	if vids := blockViewIDs(rm, params.ID); len(vids) > 0 {
		viewID = vids[0]
	} else if vid, ok := rm.FirstCollectionViewID(); ok {
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
	collection, _, err := b.requireCollection(ctx, id)
	if err != nil {
		return notion.DatabaseSchema{}, err
	}
	return ToDatabaseSchema(collection, id), nil
}
