package v3

import (
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
)

// =============================================================================
// getPage
// =============================================================================

func TestBackendGetPage(t *testing.T) {
	t.Run("standalone page", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{
				"properties":   map[string]any{"title": rtv("Hello")},
				"parent_table": "block",
				"parent_id":    "parent-1",
				"format":       map[string]any{"page_icon": "📝"},
			})},
		}))
		b := newBackend(t, s)
		res, err := b.GetPage(ctx(), "page-1")
		if err != nil {
			t.Fatal(err)
		}
		if res.ID != "page-1" {
			t.Errorf("id = %q", res.ID)
		}
		eq(t, res.Properties, map[string]any{"title": "Hello"})
		eq(t, res.Icon, iconPtr("emoji", "📝"))
		eq(t, res.Parent, parentPtr("page", "parent-1"))
	})

	t.Run("database row resolves schema", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {"row-1": mocknotion.BlockEntity("row-1", "page", map[string]any{
				"properties":   map[string]any{"title": rtv("Row 1"), "abc1": rtv("Done")},
				"parent_table": "collection",
				"parent_id":    "col-1",
			})},
			"collection": {"col-1": collectionEntity("col-1", "My DB", map[string]any{
				"title": map[string]any{"name": "Name", "type": "title"},
				"abc1":  map[string]any{"name": "Status", "type": "status"},
			})},
		}))
		b := newBackend(t, s)
		res, err := b.GetPage(ctx(), "row-1")
		if err != nil {
			t.Fatal(err)
		}
		eq(t, res.Properties, map[string]any{"Name": "Row 1", "Status": "Done"})
		eq(t, res.Parent, parentPtr("database", "col-1"))
	})

	t.Run("page not found errors", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{"block": {}}))
		b := newBackend(t, s)
		if _, err := b.GetPage(ctx(), "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("err = %v", err)
		}
	})
}

// =============================================================================
// trashPage / restorePage / archivePage / unarchivePage
// =============================================================================

func TestBackendTrashPage(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
		"block": {"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"parent_id": "parent-1", "parent_table": "block"})},
	}))
	b := newBackend(t, s)
	res, err := b.TrashPage(ctx(), "page-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "page-1" || !res.Trashed {
		t.Errorf("res = %#v", res)
	}
	ops := opsFromSave(t, s)
	if len(ops) != 3 {
		t.Fatalf("ops = %d, want 3", len(ops))
	}
	if opCmd(ops[0]) != "update" || opArgs(ops[0])["alive"] != false {
		t.Errorf("op0 = %#v", ops[0])
	}
	if opCmd(ops[1]) != "listRemove" || opCmd(ops[2]) != "update" {
		t.Errorf("ops = %#v", ops)
	}
}

func TestBackendTrashPageNotFound(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{"block": {}}))
	b := newBackend(t, s)
	if _, err := b.TrashPage(ctx(), "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v", err)
	}
}

func TestBackendRestorePage(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("restoreRecord", map[string]any{"recordMap": map[string]any{}})
	b := newBackend(t, s)
	res, err := b.RestorePage(ctx(), "page-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "page-1" || res.Trashed {
		t.Errorf("res = %#v", res)
	}
	body := lastBody(t, s, "restoreRecord")
	eq(t, body["pointer"], map[string]any{"table": "block", "id": "page-1", "spaceId": "space-1"})
}

func TestBackendArchivePage(t *testing.T) {
	s := mocknotion.New()
	b := newBackend(t, s)
	res, err := b.ArchivePage(ctx(), "page-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "page-1" || !res.Archived {
		t.Errorf("res = %#v", res)
	}
	ops := opsFromSave(t, s)
	if len(ops) != 2 {
		t.Fatalf("ops = %d, want 2", len(ops))
	}
	args := opArgs(ops[0])
	if args["archived_by_id"] != "user-1" || args["archived_by_table"] != "notion_user" {
		t.Errorf("args = %#v", args)
	}
	if _, ok := args["archived_time"].(float64); !ok {
		t.Errorf("archived_time not a number: %#v", args["archived_time"])
	}
	if _, ok := args["alive"]; ok {
		t.Error("archive args should not contain alive")
	}
	// No parent lookup needed.
	if len(s.CallsFor("loadPageChunk")) != 0 {
		t.Errorf("loadPageChunk called %d times, want 0", len(s.CallsFor("loadPageChunk")))
	}
}

func TestBackendUnarchivePage(t *testing.T) {
	s := mocknotion.New()
	b := newBackend(t, s)
	res, err := b.UnarchivePage(ctx(), "page-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Archived {
		t.Errorf("res = %#v", res)
	}
	args := opArgs(opsFromSave(t, s)[0])
	for _, k := range []string{"archived_by_id", "archived_by_table", "archived_time"} {
		if args[k] != nil {
			t.Errorf("%s = %#v, want null", k, args[k])
		}
	}
}

// =============================================================================
// createPage
// =============================================================================

func TestBackendCreatePage(t *testing.T) {
	t.Run("regular page parent", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {"parent-1": mocknotion.BlockEntity("parent-1", "page", nil)},
		}))
		b := newBackend(t, s)
		res, err := b.CreatePage(ctx(), notionCreatePage("parent-1", "New Page", nil, ""))
		if err != nil {
			t.Fatal(err)
		}
		if res.Title != "New Page" || res.ID == "" || !strings.Contains(res.URL, "notion.so") || res.CreatedAt == "" {
			t.Errorf("res = %#v", res)
		}
		eq(t, res.Parent, map[string]any{"type": "page_id", "page_id": "parent-1"})

		ops := opsFromSave(t, s)
		if len(ops) != 3 {
			t.Fatalf("ops = %d, want 3", len(ops))
		}
		if opCmd(ops[0]) != "set" || opArgs(ops[0])["type"] != "page" || opArgs(ops[0])["parent_table"] != "block" || opArgs(ops[0])["parent_id"] != "parent-1" {
			t.Errorf("set op = %#v", ops[0])
		}
		props := opArgs(ops[0])["properties"].(map[string]any)
		eq(t, props["title"], rtv("New Page"))
		if opCmd(ops[1]) != "listAfter" || opCmd(ops[2]) != "update" {
			t.Errorf("ops = %#v", ops)
		}
	})

	t.Run("database parent with schema mapping", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(dbBlockAndCollection(map[string]any{
			"title": map[string]any{"name": "Name", "type": "title"},
			"abc1":  map[string]any{"name": "Status", "type": "select"},
		})))
		b := newBackend(t, s)
		res, err := b.CreatePage(ctx(), notionCreatePage("db-1", "New Row", map[string]any{"Status": "Done"}, ""))
		if err != nil {
			t.Fatal(err)
		}
		eq(t, res.Parent, map[string]any{"type": "database_id", "database_id": "db-1"})

		ops := opsFromSave(t, s)
		setOp := findOp(ops, func(op map[string]any) bool { return opCmd(op) == "set" })
		if opArgs(setOp)["parent_table"] != "collection" || opArgs(setOp)["parent_id"] != "col-1" {
			t.Errorf("set op = %#v", setOp)
		}
		props := opArgs(setOp)["properties"].(map[string]any)
		eq(t, props["abc1"], rtv("Done"))

		listAfter := findOp(ops, func(op map[string]any) bool { return opCmd(op) == "listAfter" })
		if opPointer(listAfter)["id"] != "db-1" || opPointer(listAfter)["table"] != "block" {
			t.Errorf("listAfter pointer = %#v", opPointer(listAfter))
		}
	})

	t.Run("with icon", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {"parent-1": mocknotion.BlockEntity("parent-1", "page", nil)},
		}))
		b := newBackend(t, s)
		if _, err := b.CreatePage(ctx(), notionCreatePage("parent-1", "Page with Icon", nil, "🎯")); err != nil {
			t.Fatal(err)
		}
		setOp := findOp(opsFromSave(t, s), func(op map[string]any) bool { return opCmd(op) == "set" })
		if opArgs(setOp)["format"].(map[string]any)["page_icon"] != "🎯" {
			t.Errorf("format = %#v", opArgs(setOp)["format"])
		}
	})

	t.Run("unresolvable database errors", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {"db-1": mocknotion.BlockEntity("db-1", "collection_view_page", map[string]any{"collection_id": "col-missing"})},
		}))
		s.HandleBody("syncRecordValuesMain", map[string]any{"recordMap": map[string]any{}})
		b := newBackend(t, s)
		if _, err := b.CreatePage(ctx(), notionCreatePage("db-1", "Fail", nil, "")); err == nil || !strings.Contains(err.Error(), "Could not resolve database") {
			t.Errorf("err = %v", err)
		}
	})
}

// =============================================================================
// updatePage
// =============================================================================

func TestBackendUpdatePage(t *testing.T) {
	t.Run("title only", func(t *testing.T) {
		s := mocknotion.New()
		b := newBackend(t, s)
		res, err := b.UpdatePage(ctx(), notionUpdatePage("page-1", "Updated Title", nil, ""))
		if err != nil {
			t.Fatal(err)
		}
		if res.ID != "page-1" || !strings.Contains(res.URL, "notion.so") || res.LastEditedAt == "" {
			t.Errorf("res = %#v", res)
		}
		titleOp := findOp(opsFromSave(t, s), func(op map[string]any) bool { return opCmd(op) == "set" && opPath(op) == "properties.title" })
		if titleOp == nil {
			t.Fatal("title set op missing")
		}
		eq(t, titleOp["args"], rtv("Updated Title"))
		// no loadPageChunk when only title changes
		if len(s.CallsFor("loadPageChunk")) != 0 {
			t.Errorf("loadPageChunk calls = %d, want 0", len(s.CallsFor("loadPageChunk")))
		}
	})

	t.Run("icon", func(t *testing.T) {
		s := mocknotion.New()
		b := newBackend(t, s)
		if _, err := b.UpdatePage(ctx(), notionUpdatePage("page-1", "", nil, "🚀")); err != nil {
			t.Fatal(err)
		}
		iconOp := findOp(opsFromSave(t, s), func(op map[string]any) bool { return opCmd(op) == "set" && opPath(op) == "format.page_icon" })
		if iconOp == nil || iconOp["args"] != "🚀" {
			t.Errorf("iconOp = %#v", iconOp)
		}
	})

	t.Run("database row with schema resolution", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block":      {"row-1": mocknotion.BlockEntity("row-1", "page", map[string]any{"parent_table": "collection", "parent_id": "col-1"})},
			"collection": {"col-1": collectionEntity("col-1", "Tasks", map[string]any{"title": map[string]any{"name": "Name", "type": "title"}, "abc1": map[string]any{"name": "Status", "type": "status"}})},
		}))
		b := newBackend(t, s)
		if _, err := b.UpdatePage(ctx(), notionUpdatePage("row-1", "", map[string]any{"Status": "Done"}, "")); err != nil {
			t.Fatal(err)
		}
		statusOp := findOp(opsFromSave(t, s), func(op map[string]any) bool {
			return opCmd(op) == "set" && opPath(op) == "properties.abc1"
		})
		if statusOp == nil {
			t.Fatal("status set op missing")
		}
		eq(t, statusOp["args"], rtv("Done"))
	})

	t.Run("only editMeta when nothing provided", func(t *testing.T) {
		s := mocknotion.New()
		b := newBackend(t, s)
		if _, err := b.UpdatePage(ctx(), notionUpdatePage("page-1", "", nil, "")); err != nil {
			t.Fatal(err)
		}
		ops := opsFromSave(t, s)
		if len(ops) != 1 || opCmd(ops[0]) != "update" {
			t.Errorf("ops = %#v", ops)
		}
	})
}
