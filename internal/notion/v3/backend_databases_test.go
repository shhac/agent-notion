package v3

import (
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
)

// =============================================================================
// isDatabase
// =============================================================================

// =============================================================================
// getDatabase / getDatabaseSchema / listDatabases
// =============================================================================

func dbBlockAndCollection(schema map[string]any) map[string]map[string]any {
	return map[string]map[string]any{
		"block":      {"db-1": mocknotion.BlockEntity("db-1", "collection_view_page", map[string]any{"collection_id": "col-1"})},
		"collection": {"col-1": collectionEntity("col-1", "Tasks", schema)},
	}
}

func TestBackendGetDatabase(t *testing.T) {
	t.Run("resolves collection", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(dbBlockAndCollection(map[string]any{
			"title": map[string]any{"name": "Name", "type": "title"},
			"abc1":  map[string]any{"name": "Status", "type": "select"},
		})))
		b := newBackend(t, s)
		res, err := b.GetDatabase(ctx(), "db-1")
		if err != nil {
			t.Fatal(err)
		}
		if res.ID != "db-1" || res.Title != "Tasks" {
			t.Errorf("res = %#v", res)
		}
		if _, ok := res.Properties["Name"]; !ok {
			t.Errorf("missing Name property: %#v", res.Properties)
		}
		if _, ok := res.Properties["Status"]; !ok {
			t.Errorf("missing Status property: %#v", res.Properties)
		}
	})

	t.Run("not found errors", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {"page-1": mocknotion.BlockEntity("page-1", "page", nil)},
		}))
		b := newBackend(t, s)
		if _, err := b.GetDatabase(ctx(), "page-1"); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestBackendGetDatabaseSchema(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(dbBlockAndCollection(map[string]any{
		"title": map[string]any{"name": "Name", "type": "title"},
		"abc1": map[string]any{"name": "Tags", "type": "multi_select", "options": []any{
			map[string]any{"id": "o1", "value": "Frontend"},
			map[string]any{"id": "o2", "value": "Backend"},
		}},
	})))
	b := newBackend(t, s)
	res, err := b.GetDatabaseSchema(ctx(), "db-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "db-1" || res.Title != "Tasks" || len(res.Properties) != 2 {
		t.Fatalf("res = %#v", res)
	}
	tags := findSchemaProp(res.Properties, "Tags")
	if tags == nil {
		t.Fatal("Tags property missing")
	}
	eq(t, tags.Options, []string{"Frontend", "Backend"})
}

func TestBackendListDatabases(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("search", searchBody([]string{"p1", "db-1"}, map[string]map[string]any{
		"block": {
			"p1":   mocknotion.BlockEntity("p1", "page", map[string]any{"properties": map[string]any{"title": rtv("Page")}}),
			"db-1": mocknotion.BlockEntity("db-1", "collection_view_page", map[string]any{"collection_id": "col-1", "properties": map[string]any{"title": rtv("My DB")}}),
		},
		"collection": {"col-1": collectionEntity("col-1", "My DB", map[string]any{"title": map[string]any{"name": "Name", "type": "title"}})},
	}))
	b := newBackend(t, s)
	res, err := b.ListDatabases(ctx(), notionList())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "db-1" || res.Items[0].Title != "My DB" {
		t.Errorf("items = %#v", res.Items)
	}
}

// =============================================================================
// queryDatabase
// =============================================================================

func TestBackendQueryDatabase(t *testing.T) {
	t.Run("schema-mapped rows", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block":      {"db-1": mocknotion.BlockEntity("db-1", "collection_view_page", map[string]any{"collection_id": "col-1", "view_ids": []any{"view-1"}})},
			"collection": {"col-1": collectionEntity("col-1", "Tasks", map[string]any{"title": map[string]any{"name": "Name", "type": "title"}, "abc1": map[string]any{"name": "Status", "type": "status"}})},
		}))
		s.HandleBody("queryCollection", queryBody([]string{"row-1"}, map[string]map[string]any{
			"block": {"row-1": mocknotion.BlockEntity("row-1", "page", map[string]any{"properties": map[string]any{"title": rtv("Task 1"), "abc1": rtv("Done")}})},
		}))
		b := newBackend(t, s)
		res, err := b.QueryDatabase(ctx(), notionQuery("db-1", nil, nil))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 1 || res.Items[0].ID != "row-1" {
			t.Fatalf("items = %#v", res.Items)
		}
		eq(t, res.Items[0].Properties, map[string]any{"Name": "Task 1", "Status": "Done"})

		body := lastBody(t, s, "queryCollection")
		if body["collection"].(map[string]any)["id"] != "col-1" {
			t.Errorf("collection id = %#v", body["collection"])
		}
		if body["collectionView"].(map[string]any)["id"] != "view-1" {
			t.Errorf("collectionView id = %#v", body["collectionView"])
		}
	})

	t.Run("passes filter and sort", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block":      {"db-1": mocknotion.BlockEntity("db-1", "collection_view_page", map[string]any{"collection_id": "col-1", "view_ids": []any{"view-1"}})},
			"collection": {"col-1": collectionEntity("col-1", "Tasks", map[string]any{"title": map[string]any{"name": "Name", "type": "title"}})},
		}))
		s.HandleBody("queryCollection", queryBody(nil, map[string]map[string]any{}))
		b := newBackend(t, s)

		filter := map[string]any{"property": "Status", "value": "Done"}
		sort := map[string]any{"property": "Name", "direction": "ascending"}
		if _, err := b.QueryDatabase(ctx(), notionQuery("db-1", filter, sort)); err != nil {
			t.Fatal(err)
		}
		query2 := lastBody(t, s, "queryCollection")["query2"].(map[string]any)
		eq(t, query2["filter"], filter)
		eq(t, query2["sort"], sort)
	})
}
