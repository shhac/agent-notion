// V3 backend — Blocks section: list/get, append, update, delete, and move.

package v3

import (
	"context"
	"time"

	"github.com/shhac/agent-notion/internal/notion"
)

// ListBlocks returns a block's direct, alive children.
func (b *Backend) ListBlocks(ctx context.Context, params notion.ListBlocksParams) (notion.Paginated[notion.NormalizedBlock], error) {
	resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: params.ID, Limit: orInt(params.Limit, 50)})
	if err != nil {
		return notion.Paginated[notion.NormalizedBlock]{}, err
	}

	rm := resp.RecordMap
	children := collectAliveChildren(rm, params.ID, nil)
	items := make([]notion.NormalizedBlock, 0, len(children))
	for _, child := range children {
		items = append(items, NormalizeBlock(child, rm))
	}

	return notion.Paginated[notion.NormalizedBlock]{Items: items, HasMore: false}, nil
}

// maxAllBlocks caps how many direct children GetAllBlocks returns across chunks.
const maxAllBlocks = 1000

// collectAliveChildren returns id's alive direct children from the record map,
// in Content order. When seen is non-nil, already-seen children are skipped and
// newly returned ones are recorded (used to dedup across pagination chunks).
func collectAliveChildren(rm RecordMap, id string, seen map[string]bool) []*Block {
	parent, ok := rm.GetBlock(id)
	if !ok {
		return nil
	}
	children := make([]*Block, 0, len(parent.Content))
	for _, childID := range parent.Content {
		child, ok := rm.GetBlock(childID)
		if !ok || !child.IsAlive() {
			continue
		}
		if seen != nil {
			if seen[child.ID] {
				continue
			}
			seen[child.ID] = true
		}
		children = append(children, child)
	}
	return children
}

// GetAllBlocks walks page chunks, collecting a block's alive direct children up
// to maxAllBlocks. Deeper descent is the caller's job (renderMarkdown recurses
// per child), so this returns exactly one level.
func (b *Backend) GetAllBlocks(ctx context.Context, id string) (notion.BlockListResult, error) {
	blocks := []notion.NormalizedBlock{}
	seen := map[string]bool{}
	var cursor *Cursor
	chunkNumber := 0

	for len(blocks) < maxAllBlocks {
		resp, err := b.Client.LoadPageChunk(ctx, LoadPageChunkParams{PageID: id, Limit: 100, Cursor: cursor, ChunkNumber: chunkNumber})
		if err != nil {
			return notion.BlockListResult{}, err
		}
		rm := resp.RecordMap

		for _, child := range collectAliveChildren(rm, id, seen) {
			blocks = append(blocks, NormalizeBlock(child, rm))
		}

		if len(resp.Cursor.Stack) == 0 {
			break
		}
		next := resp.Cursor
		cursor = &next
		chunkNumber++
	}

	return notion.BlockListResult{Blocks: blocks, HasMore: len(blocks) >= maxAllBlocks}, nil
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

	if _, _, err := fetchBlock(ctx, b.Client, params.ID); err != nil {
		return empty, err
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

	block, _, err := fetchBlock(ctx, b.Client, id)
	if err != nil {
		return empty, err
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

	block, _, err := fetchBlock(ctx, b.Client, params.ID)
	if err != nil {
		return empty, err
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
