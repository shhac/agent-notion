package official

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
	"github.com/shhac/agent-notion/internal/notion"
	output "github.com/shhac/lib-agent-output"
)

// officialBackend wires a Backend to a fresh mocknotion fixture server.
func officialBackend(t *testing.T) (*Backend, *mocknotion.Server) {
	t.Helper()
	s := mocknotion.New()
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return NewBackend(Client{HTTP: ts.Client(), BaseURL: ts.URL, Token: "tkn"}), s
}

func bodyMap(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("request body not JSON: %v", err)
	}
	return m
}

// plainText builds a rich_text-style array of {plain_text} runs.
func plainText(parts ...string) []any {
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = map[string]any{"plain_text": p}
	}
	return out
}

func TestBackendKind(t *testing.T) {
	if k := (&Backend{}).Kind(); k != "official" {
		t.Errorf("Kind() = %q, want official", k)
	}
}

func TestBackendSearch(t *testing.T) {
	b, s := officialBackend(t)
	s.HandleBody("POST /v1/search", map[string]any{
		"has_more": false, "next_cursor": nil,
		"results": []any{map[string]any{"object": "page", "id": "pg1", "url": "u",
			"properties": map[string]any{"Name": map[string]any{"type": "title", "title": plainText("Hi")}}}},
	})

	got, err := b.Search(context.Background(), notion.SearchParams{Query: "hi", Filter: "page"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].Title != "Hi" {
		t.Errorf("search result = %#v", got.Items)
	}
}

func TestBackendDatabases(t *testing.T) {
	b, s := officialBackend(t)
	s.HandleBody("GET /v1/databases/db1", map[string]any{"id": "db1", "url": "u",
		"title": plainText("Tasks"), "properties": map[string]any{}})
	s.HandleBody("POST /v1/databases/db1/query", map[string]any{
		"has_more": false, "next_cursor": nil,
		"results": []any{map[string]any{"id": "row1", "url": "u",
			"properties": map[string]any{"Count": map[string]any{"type": "number", "number": 5}}}},
	})

	db, err := b.GetDatabase(context.Background(), "db1")
	if err != nil {
		t.Fatal(err)
	}
	if db.Title != "Tasks" {
		t.Errorf("GetDatabase title = %q", db.Title)
	}

	rows, err := b.QueryDatabase(context.Background(), notion.QueryDatabaseParams{ID: "db1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Items) != 1 || rows.Items[0].Properties["Count"] != float64(5) {
		t.Errorf("QueryDatabase rows = %#v", rows.Items)
	}
}

func TestBackendGetPage(t *testing.T) {
	b, s := officialBackend(t)
	s.HandleBody("GET /v1/pages/pg1", map[string]any{"id": "pg1", "url": "u",
		"properties": map[string]any{"Name": map[string]any{"type": "title", "title": plainText("Doc")}}})

	got, err := b.GetPage(context.Background(), "pg1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "pg1" || got.Properties["Name"] != "Doc" {
		t.Errorf("GetPage = %#v", got)
	}
}

func TestBackendCreatePageDatabaseParent(t *testing.T) {
	b, s := officialBackend(t)
	// Parent probe resolves to a database.
	s.HandleBody("GET /v1/databases/parent1", map[string]any{"id": "parent1"})
	s.HandleBody("POST /v1/pages", map[string]any{"id": "new1", "url": "u",
		"parent": map[string]any{"type": "database_id", "database_id": "parent1"}, "created_time": "2024-01-01T00:00:00Z"})

	got, err := b.CreatePage(context.Background(), notion.CreatePageParams{
		ParentID: "parent1", Title: "Task", Properties: map[string]any{"Priority": "High"}, Icon: "🚀"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "new1" || got.Title != "Task" {
		t.Errorf("create result = %#v", got)
	}

	body := bodyMap(t, s.CallsFor("POST /v1/pages")[0].Body)
	if parent := body["parent"].(map[string]any); parent["database_id"] != "parent1" {
		t.Errorf("parent = %#v", parent)
	}
	if props := body["properties"].(map[string]any); props["Name"] == nil {
		t.Errorf("database create should carry a Name title: %#v", props)
	}
}

func TestBackendCreatePagePageParent(t *testing.T) {
	b, s := officialBackend(t)
	// Parent probe 404s -> treated as a page parent.
	s.Handle("GET /v1/databases/parent2", mocknotion.Response{Status: 404,
		Body: map[string]any{"object": "error", "status": 404, "code": "object_not_found"}})
	s.HandleBody("POST /v1/pages", map[string]any{"id": "new1", "url": "u",
		"parent": map[string]any{"type": "page_id", "page_id": "parent2"}})

	if _, err := b.CreatePage(context.Background(), notion.CreatePageParams{ParentID: "parent2", Title: "Child"}); err != nil {
		t.Fatal(err)
	}
	body := bodyMap(t, s.CallsFor("POST /v1/pages")[0].Body)
	if parent := body["parent"].(map[string]any); parent["page_id"] != "parent2" {
		t.Errorf("parent = %#v", parent)
	}
	if props := body["properties"].(map[string]any); props["title"] == nil {
		t.Errorf("page create should carry a title property: %#v", props)
	}
}

func TestBackendBlockOps(t *testing.T) {
	b, s := officialBackend(t)
	s.HandleBody("GET /v1/blocks/blk1/children", map[string]any{
		"has_more": false, "next_cursor": nil,
		"results": []any{map[string]any{"id": "c1", "type": "paragraph", "has_children": false,
			"paragraph": map[string]any{"rich_text": plainText("hi")}}},
	})
	s.HandleBody("PATCH /v1/blocks/blk1/children", map[string]any{
		"results": []any{map[string]any{"id": "n1"}, map[string]any{"id": "n2"}}})
	s.HandleBody("GET /v1/blocks/b2", map[string]any{"id": "b2", "type": "paragraph"})
	s.HandleBody("PATCH /v1/blocks/b2", map[string]any{"id": "b2", "last_edited_time": "2024-03-03T00:00:00Z"})
	s.HandleBody("DELETE /v1/blocks/b2", map[string]any{"id": "b2"})

	list, err := b.ListBlocks(context.Background(), notion.ListBlocksParams{ID: "blk1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].RichText != "hi" {
		t.Errorf("ListBlocks = %#v", list.Items)
	}

	all, err := b.GetAllBlocks(context.Background(), "blk1")
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Blocks) != 1 || all.HasMore {
		t.Errorf("GetAllBlocks = %#v hasMore=%v", all.Blocks, all.HasMore)
	}

	appended, err := b.AppendBlocks(context.Background(), notion.AppendBlocksParams{
		ID: "blk1", Blocks: []any{map[string]any{"type": "paragraph"}, map[string]any{"type": "divider"}}})
	if err != nil {
		t.Fatal(err)
	}
	if appended.BlocksAdded != 2 {
		t.Errorf("BlocksAdded = %d", appended.BlocksAdded)
	}

	content := "Updated"
	upd, err := b.UpdateBlock(context.Background(), notion.UpdateBlockParams{ID: "b2", Content: &content})
	if err != nil {
		t.Fatal(err)
	}
	if upd.LastEditedAt != "2024-03-03T00:00:00Z" {
		t.Errorf("UpdateBlock = %#v", upd)
	}

	del, err := b.DeleteBlock(context.Background(), "b2")
	if err != nil {
		t.Fatal(err)
	}
	if !del.Deleted {
		t.Errorf("DeleteBlock = %#v", del)
	}
}

func TestBackendComments(t *testing.T) {
	b, s := officialBackend(t)
	s.HandleBody("GET /v1/comments", map[string]any{
		"has_more": false, "next_cursor": nil,
		"results": []any{map[string]any{"id": "c1", "rich_text": plainText("hello"),
			"created_by": map[string]any{"id": "u1", "name": "Al"}}}})
	s.HandleBody("POST /v1/comments", map[string]any{"id": "c2", "created_time": "2024-01-01T00:00:00Z",
		"rich_text": plainText("the body")})

	list, err := b.ListComments(context.Background(), notion.ListCommentsParams{PageID: "pg1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].Body != "hello" {
		t.Errorf("ListComments = %#v", list.Items)
	}

	added, err := b.AddComment(context.Background(), notion.AddCommentParams{PageID: "pg1", Body: "the body"})
	if err != nil {
		t.Fatal(err)
	}
	if added.ID != "c2" || added.Body != "the body" {
		t.Errorf("AddComment = %#v", added)
	}
}

func TestBackendUsers(t *testing.T) {
	b, s := officialBackend(t)
	s.HandleBody("GET /v1/users", map[string]any{
		"has_more": false, "next_cursor": nil,
		"results": []any{map[string]any{"id": "u1", "name": "Al", "type": "person",
			"person": map[string]any{"email": "al@example.test"}}}})
	s.HandleBody("GET /v1/users/me", map[string]any{"id": "bot1", "name": "Bot", "type": "bot",
		"bot": map[string]any{"workspace_name": "WS"}})

	users, err := b.ListUsers(context.Background(), notion.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(users.Items) != 1 || users.Items[0].Email != "al@example.test" {
		t.Errorf("ListUsers = %#v", users.Items)
	}

	me, err := b.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.ID != "bot1" || me.WorkspaceName != "WS" {
		t.Errorf("GetMe = %#v", me)
	}
}

func TestBackendIsDatabase(t *testing.T) {
	b, s := officialBackend(t)
	s.HandleBody("GET /v1/databases/db1", map[string]any{"id": "db1"})
	s.Handle("GET /v1/databases/pg1", mocknotion.Response{Status: 404,
		Body: map[string]any{"object": "error", "status": 404, "code": "object_not_found"}})

	if ok, err := b.IsDatabase(context.Background(), "db1"); err != nil || !ok {
		t.Errorf("IsDatabase(db1) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := b.IsDatabase(context.Background(), "pg1"); err != nil || ok {
		t.Errorf("IsDatabase(pg1) = %v, %v; want false, nil", ok, err)
	}
}

func TestBackendV3GuidanceErrors(t *testing.T) {
	b := NewBackend(Client{}) // no network needed; these never call out

	assertGuidance := func(t *testing.T, err error, wantMsg string) {
		t.Helper()
		if err == nil {
			t.Fatal("expected a guidance error")
		}
		var oe *output.Error
		if !errors.As(err, &oe) {
			t.Fatalf("want *output.Error, got %T (%v)", err, err)
		}
		if oe.FixableBy != output.FixableByHuman {
			t.Errorf("FixableBy = %q, want human", oe.FixableBy)
		}
		if !strings.Contains(oe.Message, wantMsg) {
			t.Errorf("message = %q, want contains %q", oe.Message, wantMsg)
		}
		if !strings.Contains(oe.Hint, "auth import-desktop") {
			t.Errorf("hint = %q, want to mention import-desktop", oe.Hint)
		}
	}

	t.Run("ArchivePage", func(t *testing.T) {
		_, err := b.ArchivePage(context.Background(), "pg1")
		assertGuidance(t, err, "Real Archive (distinct from Trash) requires the v3 backend")
	})
	t.Run("UnarchivePage", func(t *testing.T) {
		_, err := b.UnarchivePage(context.Background(), "pg1")
		assertGuidance(t, err, "Unarchive (real Archive, distinct from Trash) requires the v3 backend")
	})
	t.Run("MoveBlock", func(t *testing.T) {
		_, err := b.MoveBlock(context.Background(), notion.MoveBlockParams{ID: "b1"})
		assertGuidance(t, err, "Block reordering requires the v3 backend")
	})
	t.Run("AddInlineComment", func(t *testing.T) {
		_, err := b.AddInlineComment(context.Background(), notion.AddInlineCommentParams{BlockID: "b1", Body: "x", Text: "y"})
		assertGuidance(t, err, "Inline comments require the v3 backend")
	})
}
