package v3

import (
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
)

// =============================================================================
// listBlocks
// =============================================================================

func TestBackendListBlocks(t *testing.T) {
	t.Run("normalizes children", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {
				"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"content": []any{"b1", "b2"}}),
				"b1":     mocknotion.BlockEntity("b1", "text", map[string]any{"properties": map[string]any{"title": rtv("Hello")}}),
				"b2":     mocknotion.BlockEntity("b2", "header", map[string]any{"properties": map[string]any{"title": rtv("Heading")}}),
			},
		}))
		b := newBackend(t, s)
		res, err := b.ListBlocks(ctx(), notionListBlocks("page-1"))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 2 || res.Items[0].Type != "paragraph" || res.Items[0].RichText != "Hello" || res.Items[1].Type != "heading_1" {
			t.Errorf("items = %#v", res.Items)
		}
	})

	t.Run("skips dead blocks", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {
				"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"content": []any{"b1", "b2"}}),
				"b1":     mocknotion.BlockEntity("b1", "text", map[string]any{"alive": true}),
				"b2":     mocknotion.BlockEntity("b2", "text", map[string]any{"alive": false}),
			},
		}))
		b := newBackend(t, s)
		res, err := b.ListBlocks(ctx(), notionListBlocks("page-1"))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 1 {
			t.Errorf("items = %#v", res.Items)
		}
	})
}

// TestBackendListBlocksResolvesMention pins the seam: a child block whose title
// carries a person-mention decoration is normalized with the name resolved from
// the record map, not left as the ‣ placeholder.
func TestBackendListBlocksResolvesMention(t *testing.T) {
	mentionTitle := []any{[]any{"‣", []any{[]any{"u", "U1"}}}}
	s := mocknotion.New()
	s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
		"block": {
			"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"content": []any{"b1"}}),
			"b1":     mocknotion.BlockEntity("b1", "text", map[string]any{"properties": map[string]any{"title": mentionTitle}}),
		},
		"notion_user": {"U1": userEntity("U1", "Ivan", "Tchernev")},
	}))
	b := newBackend(t, s)
	res, err := b.ListBlocks(ctx(), notionListBlocks("page-1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].RichText != "@Ivan Tchernev" {
		t.Errorf("items = %#v", res.Items)
	}
}

// =============================================================================
// getAllBlocks / getChildBlocks
// =============================================================================

func TestBackendGetAllBlocks(t *testing.T) {
	t.Run("collects across chunks", func(t *testing.T) {
		s := mocknotion.New()
		chunk1 := mocknotion.RecordMapBody(map[string]map[string]any{
			"block": {
				"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"content": []any{"b1", "b2"}}),
				"b1":     mocknotion.BlockEntity("b1", "text", map[string]any{"properties": map[string]any{"title": rtv("Block 1")}}),
				"b2":     mocknotion.BlockEntity("b2", "text", map[string]any{"properties": map[string]any{"title": rtv("Block 2")}}),
			},
		})
		chunk1["cursor"] = map[string]any{"stack": []any{[]any{"more"}}}
		chunk2 := mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {
				"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"content": []any{"b1", "b2", "b3"}}),
				"b3":     mocknotion.BlockEntity("b3", "text", map[string]any{"properties": map[string]any{"title": rtv("Block 3")}}),
			},
		})
		s.Handle("loadPageChunk", mocknotion.Response{Body: chunk1}, mocknotion.Response{Body: chunk2})
		b := newBackend(t, s)
		res, err := b.GetAllBlocks(ctx(), "page-1")
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Blocks) != 3 || res.HasMore {
			t.Errorf("blocks = %d hasMore = %v", len(res.Blocks), res.HasMore)
		}
	})

	t.Run("deduplicates across chunks", func(t *testing.T) {
		s := mocknotion.New()
		chunk1 := mocknotion.RecordMapBody(map[string]map[string]any{
			"block": {
				"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"content": []any{"b1"}}),
				"b1":     mocknotion.BlockEntity("b1", "text", nil),
			},
		})
		chunk1["cursor"] = map[string]any{"stack": []any{[]any{"more"}}}
		chunk2 := mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {
				"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"content": []any{"b1"}}),
				"b1":     mocknotion.BlockEntity("b1", "text", nil),
			},
		})
		s.Handle("loadPageChunk", mocknotion.Response{Body: chunk1}, mocknotion.Response{Body: chunk2})
		b := newBackend(t, s)
		res, err := b.GetAllBlocks(ctx(), "page-1")
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Blocks) != 1 {
			t.Errorf("blocks = %d, want 1", len(res.Blocks))
		}
	})
}

// =============================================================================
// appendBlocks
// =============================================================================

func paragraphBlock(content string) map[string]any {
	return map[string]any{
		"type":      "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{"text": map[string]any{"content": content}}}},
	}
}

func TestBackendAppendBlocks(t *testing.T) {
	t.Run("single block", func(t *testing.T) {
		s := mocknotion.New()
		b := newBackend(t, s)
		res, err := b.AppendBlocks(ctx(), notionAppend("page-1", []any{paragraphBlock("Hello")}))
		if err != nil {
			t.Fatal(err)
		}
		if res.BlocksAdded != 1 {
			t.Errorf("blocksAdded = %d", res.BlocksAdded)
		}
		ops := opsFromSave(t, s)
		if len(ops) != 3 {
			t.Fatalf("ops = %d, want 3", len(ops))
		}
		if opCmd(ops[0]) != "set" || opArgs(ops[0])["type"] != "text" {
			t.Errorf("op0 = %#v", ops[0])
		}
		if opCmd(ops[1]) != "listAfter" || opCmd(ops[2]) != "update" || opPointer(ops[2])["id"] != "page-1" {
			t.Errorf("ops = %#v", ops)
		}
	})

	t.Run("multiple blocks with after chain", func(t *testing.T) {
		s := mocknotion.New()
		b := newBackend(t, s)
		res, err := b.AppendBlocks(ctx(), notionAppend("page-1", []any{paragraphBlock("First"), paragraphBlock("Second")}))
		if err != nil {
			t.Fatal(err)
		}
		if res.BlocksAdded != 2 {
			t.Errorf("blocksAdded = %d", res.BlocksAdded)
		}
		ops := opsFromSave(t, s)
		if len(ops) != 5 {
			t.Fatalf("ops = %d, want 5", len(ops))
		}
		if _, ok := opArgs(ops[1])["after"]; ok {
			t.Error("first listAfter should have no 'after'")
		}
		firstBlockID := opArgs(ops[0])["id"]
		if opCmd(ops[3]) != "listAfter" || opArgs(ops[3])["after"] != firstBlockID {
			t.Errorf("second listAfter = %#v, want after=%v", ops[3], firstBlockID)
		}
		if opCmd(ops[4]) != "update" || opPointer(ops[4])["id"] != "page-1" {
			t.Errorf("final op = %#v", ops[4])
		}
	})

	t.Run("single parent editMeta", func(t *testing.T) {
		s := mocknotion.New()
		b := newBackend(t, s)
		if _, err := b.AppendBlocks(ctx(), notionAppend("page-1", []any{paragraphBlock("A"), paragraphBlock("B"), paragraphBlock("C")})); err != nil {
			t.Fatal(err)
		}
		ops := opsFromSave(t, s)
		count := 0
		for _, op := range ops {
			if opCmd(op) == "update" && opPointer(op)["id"] == "page-1" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("parent editMeta count = %d, want 1", count)
		}
	})
}
