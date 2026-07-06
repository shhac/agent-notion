package v3

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
	"github.com/shhac/agent-notion/internal/notion"
)

// --- notion param/result constructors ---

func notionSearch(query, filter string) notion.SearchParams {
	return notion.SearchParams{Query: query, Filter: filter}
}
func notionList() notion.ListParams { return notion.ListParams{} }
func notionListBlocks(id string) notion.ListBlocksParams {
	return notion.ListBlocksParams{ID: id}
}
func notionListComments(pageID string) notion.ListCommentsParams {
	return notion.ListCommentsParams{PageID: pageID}
}
func notionAddComment(pageID, body string) notion.AddCommentParams {
	return notion.AddCommentParams{PageID: pageID, Body: body}
}
func notionQuery(id string, filter, sort any) notion.QueryDatabaseParams {
	return notion.QueryDatabaseParams{ID: id, Filter: filter, Sort: sort}
}
func notionCreatePage(parentID, title string, props map[string]any, icon string) notion.CreatePageParams {
	return notion.CreatePageParams{ParentID: parentID, Title: title, Properties: props, Icon: icon}
}
func notionUpdatePage(id, title string, props map[string]any, icon string) notion.UpdatePageParams {
	return notion.UpdatePageParams{ID: id, Title: title, Properties: props, Icon: icon}
}
func notionAppend(id string, blocks []any) notion.AppendBlocksParams {
	return notion.AppendBlocksParams{ID: id, Blocks: blocks}
}
func notionInline(blockID, body, text string, occ int) notion.AddInlineCommentParams {
	return notion.AddInlineCommentParams{BlockID: blockID, Body: body, Text: text, Occurrence: occ}
}
func iconPtr(typ, emoji string) *notion.Icon     { return &notion.Icon{Type: typ, Emoji: emoji} }
func parentPtr(typ, id string) *notion.ParentRef { return &notion.ParentRef{Type: typ, ID: id} }
func userRef(id, name string) *notion.UserRef    { return &notion.UserRef{ID: id, Name: name} }

// newBackend wires a Backend to a mocknotion server, pre-registering a sticky
// saveTransactions success so write paths don't 404.
func newBackend(t *testing.T, s *mocknotion.Server) *Backend {
	t.Helper()
	s.Handle("saveTransactions", mocknotion.Response{Body: map[string]any{"recordMap": map[string]any{}}})
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return &Backend{Client: &Client{
		HTTP:    ts.Client(),
		BaseURL: ts.URL + "/api/v3",
		TokenV2: "fake-token",
		UserID:  "user-1",
		SpaceID: "space-1",
	}}
}

// --- Fixture helpers ---

// rtv builds a v3 rich-text value ([[text]]) as decoded JSON.
func rtv(text string) []any { return []any{[]any{text}} }

func collectionEntity(id, name string, schema map[string]any) map[string]any {
	return map[string]any{
		"id":           id,
		"version":      1,
		"name":         []any{[]any{name}},
		"schema":       schema,
		"parent_id":    "parent-1",
		"parent_table": "block",
	}
}

func searchBody(hits []string, tables map[string]map[string]any) map[string]any {
	results := make([]any, 0, len(hits))
	for _, id := range hits {
		results = append(results, map[string]any{"id": id, "score": 1})
	}
	rm := map[string]any{}
	for name, ents := range tables {
		rm[name] = mocknotion.Table(ents)
	}
	return map[string]any{"results": results, "total": len(hits), "recordMap": rm}
}

func queryBody(blockIDs []string, tables map[string]map[string]any) map[string]any {
	ids := make([]any, len(blockIDs))
	for i, id := range blockIDs {
		ids[i] = id
	}
	rm := map[string]any{}
	for name, ents := range tables {
		rm[name] = mocknotion.Table(ents)
	}
	return map[string]any{"result": map[string]any{"blockIds": ids, "total": len(blockIDs)}, "recordMap": rm}
}

// --- saveTransactions inspection ---

func opsFromSave(t *testing.T, s *mocknotion.Server) []map[string]any {
	t.Helper()
	calls := s.CallsFor("saveTransactions")
	if len(calls) == 0 {
		t.Fatal("no saveTransactions call recorded")
	}
	var body struct {
		Transactions []struct {
			Operations []map[string]any `json:"operations"`
		} `json:"transactions"`
	}
	if err := json.Unmarshal(calls[len(calls)-1].Body, &body); err != nil {
		t.Fatalf("decode saveTransactions body: %v", err)
	}
	if len(body.Transactions) == 0 {
		t.Fatal("no transactions in saveTransactions body")
	}
	return body.Transactions[0].Operations
}

func opCmd(op map[string]any) string          { s, _ := op["command"].(string); return s }
func opArgs(op map[string]any) map[string]any { m, _ := op["args"].(map[string]any); return m }
func opPointer(op map[string]any) map[string]any {
	m, _ := op["pointer"].(map[string]any)
	return m
}
func opPath(op map[string]any) string {
	raw, _ := op["path"].([]any)
	parts := make([]string, len(raw))
	for i, v := range raw {
		parts[i], _ = v.(string)
	}
	return strings.Join(parts, ".")
}
func findOp(ops []map[string]any, pred func(map[string]any) bool) map[string]any {
	for _, op := range ops {
		if pred(op) {
			return op
		}
	}
	return nil
}

func lastBody(t *testing.T, s *mocknotion.Server, key string) map[string]any {
	t.Helper()
	calls := s.CallsFor(key)
	if len(calls) == 0 {
		t.Fatalf("no %s call recorded", key)
	}
	var m map[string]any
	if err := json.Unmarshal(calls[len(calls)-1].Body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// =============================================================================
// search
// =============================================================================

func TestBackendSearch(t *testing.T) {
	t.Run("transforms results", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("search", searchBody([]string{"page-1"}, map[string]map[string]any{
			"block": {"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"properties": map[string]any{"title": rtv("My Page")}})},
		}))
		b := newBackend(t, s)

		res, err := b.Search(ctx(), notionSearch("test", ""))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 1 || res.Items[0].ID != "page-1" || res.Items[0].Title != "My Page" || res.Items[0].Type != "page" {
			t.Errorf("items = %#v", res.Items)
		}
	})

	t.Run("filter=page drops databases", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("search", searchBody([]string{"p1", "d1"}, map[string]map[string]any{
			"block": {
				"p1": mocknotion.BlockEntity("p1", "page", map[string]any{"properties": map[string]any{"title": rtv("Page")}}),
				"d1": mocknotion.BlockEntity("d1", "collection_view_page", map[string]any{"properties": map[string]any{"title": rtv("DB")}}),
			},
		}))
		b := newBackend(t, s)
		res, err := b.Search(ctx(), notionSearch("test", "page"))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 1 || res.Items[0].Type != "page" {
			t.Errorf("items = %#v", res.Items)
		}
	})

	t.Run("filter=database drops pages", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("search", searchBody([]string{"p1", "d1"}, map[string]map[string]any{
			"block": {
				"p1": mocknotion.BlockEntity("p1", "page", nil),
				"d1": mocknotion.BlockEntity("d1", "collection_view_page", nil),
			},
		}))
		b := newBackend(t, s)
		res, err := b.Search(ctx(), notionSearch("test", "database"))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 1 || res.Items[0].Type != "database" {
			t.Errorf("items = %#v", res.Items)
		}
	})

	t.Run("skips blocks missing from recordMap", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("search", searchBody([]string{"missing"}, map[string]map[string]any{"block": {}}))
		b := newBackend(t, s)
		res, err := b.Search(ctx(), notionSearch("test", ""))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 0 {
			t.Errorf("items = %#v", res.Items)
		}
	})
}

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
// listComments
// =============================================================================

func TestBackendListComments(t *testing.T) {
	t.Run("resolves authors", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block":       {"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"discussions": []any{"disc-1"}})},
			"discussion":  {"disc-1": discussionEntity("disc-1", "page-1", "block", []any{"c1"})},
			"comment":     {"c1": commentEntity("c1", "disc-1", "Nice work", "u1")},
			"notion_user": {"u1": userEntity("u1", "Jane", "Doe")},
		}))
		b := newBackend(t, s)
		res, err := b.ListComments(ctx(), notionListComments("page-1"))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 1 || res.Items[0].Body != "Nice work" {
			t.Fatalf("items = %#v", res.Items)
		}
		eq(t, res.Items[0].Author, userRef("u1", "Jane Doe"))
	})

	t.Run("empty when no discussions", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {"page-1": mocknotion.BlockEntity("page-1", "page", nil)},
		}))
		b := newBackend(t, s)
		res, err := b.ListComments(ctx(), notionListComments("page-1"))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 0 {
			t.Errorf("items = %#v", res.Items)
		}
	})

	t.Run("backfills missing discussions and comments", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"discussions": []any{"disc-1"}})},
		}))
		s.HandleWhen("syncRecordValuesMain",
			func(body json.RawMessage) bool { return bytes.Contains(body, []byte("discussion")) },
			mocknotion.Response{Body: mocknotion.RecordMapBody(map[string]map[string]any{
				"discussion": {"disc-1": discussionEntity("disc-1", "page-1", "block", []any{"c1"})},
			})},
		)
		s.HandleWhen("syncRecordValuesMain",
			func(body json.RawMessage) bool { return bytes.Contains(body, []byte("comment")) },
			mocknotion.Response{Body: mocknotion.RecordMapBody(map[string]map[string]any{
				"comment": {"c1": commentEntity("c1", "disc-1", "Hello", "u1")},
			})},
		)
		b := newBackend(t, s)
		res, err := b.ListComments(ctx(), notionListComments("page-1"))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 1 || res.Items[0].Body != "Hello" {
			t.Errorf("items = %#v", res.Items)
		}
		if n := len(s.CallsFor("syncRecordValuesMain")); n != 2 {
			t.Errorf("syncRecordValues calls = %d, want 2", n)
		}
	})

	t.Run("inline comments from child blocks", func(t *testing.T) {
		s := mocknotion.New()
		childTitle := []any{[]any{"Hello "}, []any{"world", []any{[]any{"m", "disc-inline"}}}}
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block": {
				"page-1":  mocknotion.BlockEntity("page-1", "page", map[string]any{"content": []any{"child-1"}}),
				"child-1": mocknotion.BlockEntity("child-1", "text", map[string]any{"parent_id": "page-1", "parent_table": "block", "properties": map[string]any{"title": childTitle}, "discussions": []any{"disc-inline"}}),
			},
			"discussion":  {"disc-inline": discussionEntity("disc-inline", "child-1", "block", []any{"c-inline"})},
			"comment":     {"c-inline": commentEntity("c-inline", "disc-inline", "Fix this typo", "u1")},
			"notion_user": {"u1": userEntity("u1", "Jane", "Doe")},
		}))
		b := newBackend(t, s)
		res, err := b.ListComments(ctx(), notionListComments("page-1"))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 1 || res.Items[0].Body != "Fix this typo" || res.Items[0].AnchorText != "world" {
			t.Fatalf("items = %#v", res.Items)
		}
		eq(t, res.Items[0].Author, userRef("u1", "Jane Doe"))
	})

	t.Run("page-level comments have no anchor text", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
			"block":      {"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{"discussions": []any{"disc-page"}})},
			"discussion": {"disc-page": discussionEntity("disc-page", "page-1", "block", []any{"c1"})},
			"comment":    {"c1": commentEntity("c1", "disc-page", "Page comment", "u1")},
		}))
		b := newBackend(t, s)
		res, err := b.ListComments(ctx(), notionListComments("page-1"))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) != 1 || res.Items[0].Body != "Page comment" || res.Items[0].AnchorText != "" {
			t.Errorf("items = %#v", res.Items)
		}
	})
}

// =============================================================================
// addComment
// =============================================================================

func TestBackendAddComment(t *testing.T) {
	s := mocknotion.New()
	b := newBackend(t, s)
	res, err := b.AddComment(ctx(), notionAddComment("page-1", "Test comment"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Body != "Test comment" || res.ID == "" || res.DiscussionID == "" || res.CreatedAt == "" {
		t.Errorf("res = %#v", res)
	}
	if ops := opsFromSave(t, s); len(ops) != 6 {
		t.Errorf("ops = %d, want 6", len(ops))
	}
}

// =============================================================================
// listUsers / getMe
// =============================================================================

func TestBackendListUsers(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("loadUserContent", mocknotion.RecordMapBody(map[string]map[string]any{
		"notion_user": {
			"u1": userEntity("u1", "Alice", "B"),
			"u2": userEntity("u2", "Charlie", "D"),
		},
	}))
	b := newBackend(t, s)
	res, err := b.ListUsers(ctx(), notionList())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 || res.Items[0].Name != "Alice B" || res.Items[1].Name != "Charlie D" || res.HasMore {
		t.Errorf("items = %#v", res.Items)
	}
}

func TestBackendGetMe(t *testing.T) {
	t.Run("with workspace", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadUserContent", mocknotion.RecordMapBody(map[string]map[string]any{
			"notion_user": {"u1": userEntity("u1", "Alice", "B")},
			"space":       {"s1": map[string]any{"id": "s1", "name": "My Workspace"}},
		}))
		b := newBackend(t, s)
		res, err := b.GetMe(ctx())
		if err != nil {
			t.Fatal(err)
		}
		if res.ID != "u1" || res.Name != "Alice B" || res.WorkspaceName != "My Workspace" {
			t.Errorf("res = %#v", res)
		}
	})

	t.Run("role-wrapped records", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadUserContent", map[string]any{"recordMap": map[string]any{
			"notion_user": map[string]any{"u1": mocknotion.RoleWrappedEntry(userEntity("u1", "Alice", "B"), "space-1")},
			"space":       map[string]any{"s1": mocknotion.RoleWrappedEntry(map[string]any{"id": "s1", "name": "My Workspace"}, "space-1")},
		}})
		b := newBackend(t, s)
		res, err := b.GetMe(ctx())
		if err != nil {
			t.Fatal(err)
		}
		if res.ID != "u1" || res.Name != "Alice B" || res.WorkspaceName != "My Workspace" {
			t.Errorf("res = %#v", res)
		}
	})

	t.Run("no user errors", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("loadUserContent", mocknotion.RecordMapBody(map[string]map[string]any{"notion_user": {}}))
		b := newBackend(t, s)
		if _, err := b.GetMe(ctx()); err == nil || !strings.Contains(err.Error(), "user information") {
			t.Errorf("err = %v", err)
		}
	})
}

// =============================================================================
// isDatabase
// =============================================================================

func TestBackendIsDatabase(t *testing.T) {
	cases := []struct {
		name string
		typ  string
		want bool
	}{
		{"collection_view_page", "collection_view_page", true},
		{"collection_view", "collection_view", true},
		{"page", "page", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mocknotion.New()
			s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{
				"block": {"db-1": mocknotion.BlockEntity("db-1", tc.typ, nil)},
			}))
			b := newBackend(t, s)
			got, err := b.IsDatabase(ctx(), "db-1")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("false on error", func(t *testing.T) {
		s := mocknotion.New()
		s.Handle("loadPageChunk", mocknotion.Response{Status: 500, Body: map[string]any{"name": "Boom"}})
		b := newBackend(t, s)
		got, err := b.IsDatabase(ctx(), "missing")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if got {
			t.Error("want false on error")
		}
	})
}

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

func TestBackendGetChildBlocks(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("loadPageChunk", mocknotion.PageChunkBody(map[string]map[string]any{}))
	b := newBackend(t, s)
	ids := []string{"b1", "b2", "b3", "b4", "b5", "b6", "b7"}
	res, err := b.GetChildBlocks(ctx(), ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 7 {
		t.Fatalf("map size = %d, want 7", len(res))
	}
	for _, id := range ids {
		if _, ok := res[id]; !ok {
			t.Errorf("missing entry for %s", id)
		}
	}
	if n := len(s.CallsFor("loadPageChunk")); n != 7 {
		t.Errorf("loadPageChunk calls = %d, want 7", n)
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

// =============================================================================
// addInlineComment
// =============================================================================

func TestBackendAddInlineComment(t *testing.T) {
	syncBlock := func(title []any) map[string]any {
		overrides := map[string]any{"parent_id": "page-1", "parent_table": "block"}
		if title != nil {
			overrides["properties"] = map[string]any{"title": title}
		}
		return mocknotion.RecordMapBody(map[string]map[string]any{
			"block": {"block-1": mocknotion.BlockEntity("block-1", "text", overrides)},
		})
	}

	t.Run("decorates found text", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("syncRecordValuesMain", syncBlock(rtv("Hello world")))
		b := newBackend(t, s)
		res, err := b.AddInlineComment(ctx(), notionInline("block-1", "Fix this", "world", 0))
		if err != nil {
			t.Fatal(err)
		}
		if res.Body != "Fix this" || res.ID == "" || res.DiscussionID == "" || res.CreatedAt == "" {
			t.Errorf("res = %#v", res)
		}
		ops := opsFromSave(t, s)
		if len(ops) != 8 {
			t.Fatalf("ops = %d, want 8", len(ops))
		}
		titleOp := findOp(ops, func(op map[string]any) bool { return opCmd(op) == "set" && opPath(op) == "properties.title" })
		if titleOp == nil {
			t.Fatal("title set op missing")
		}
		if !hasDecoratedSegment(titleOp["args"], "world", "m") {
			t.Errorf("expected 'world' segment with m decoration: %#v", titleOp["args"])
		}
	})

	t.Run("text not found errors", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("syncRecordValuesMain", syncBlock(rtv("Hello world")))
		b := newBackend(t, s)
		if _, err := b.AddInlineComment(ctx(), notionInline("block-1", "Note", "missing", 0)); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("nth occurrence", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("syncRecordValuesMain", syncBlock(rtv("hello hello hello")))
		b := newBackend(t, s)
		if _, err := b.AddInlineComment(ctx(), notionInline("block-1", "Second one", "hello", 2)); err != nil {
			t.Fatal(err)
		}
		titleOp := findOp(opsFromSave(t, s), func(op map[string]any) bool { return opCmd(op) == "set" && opPath(op) == "properties.title" })
		segs := titleOp["args"].([]any)
		if len(segs) != 3 {
			t.Fatalf("segments = %d, want 3: %#v", len(segs), segs)
		}
		if segText(segs[0]) != "hello " || segText(segs[1]) != "hello" || segText(segs[2]) != " hello" {
			t.Errorf("segments = %#v", segs)
		}
		if !segHasDecoration(segs[1], "m") {
			t.Errorf("second hello not decorated: %#v", segs[1])
		}
	})

	t.Run("no text content errors", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("syncRecordValuesMain", syncBlock(nil))
		b := newBackend(t, s)
		if _, err := b.AddInlineComment(ctx(), notionInline("block-1", "Note", "anything", 0)); err == nil || !strings.Contains(err.Error(), "no text content") {
			t.Errorf("err = %v", err)
		}
	})
}

// --- decoration-segment helpers ---

func segText(seg any) string {
	arr, _ := seg.([]any)
	if len(arr) == 0 {
		return ""
	}
	s, _ := arr[0].(string)
	return s
}

func segHasDecoration(seg any, decType string) bool {
	arr, _ := seg.([]any)
	if len(arr) < 2 {
		return false
	}
	decs, _ := arr[1].([]any)
	for _, d := range decs {
		dec, _ := d.([]any)
		if len(dec) > 0 {
			if t, _ := dec[0].(string); t == decType {
				return true
			}
		}
	}
	return false
}

func hasDecoratedSegment(args any, text, decType string) bool {
	segs, _ := args.([]any)
	for _, seg := range segs {
		if segText(seg) == text && segHasDecoration(seg, decType) {
			return true
		}
	}
	return false
}

// --- entity builders ---

func discussionEntity(id, parentID, parentTable string, comments []any) map[string]any {
	return map[string]any{
		"id":           id,
		"version":      1,
		"parent_id":    parentID,
		"parent_table": parentTable,
		"resolved":     false,
		"comments":     comments,
	}
}

func commentEntity(id, discussionID, text, createdBy string) map[string]any {
	return map[string]any{
		"id":               id,
		"version":          1,
		"alive":            true,
		"parent_id":        discussionID,
		"parent_table":     "discussion",
		"text":             []any{[]any{text}},
		"created_by_id":    createdBy,
		"created_by_table": "notion_user",
		"created_time":     1700000000000,
		"last_edited_time": 1700000000000,
	}
}

func userEntity(id, given, family string) map[string]any {
	return map[string]any{
		"id":          id,
		"version":     1,
		"email":       "test@example.com",
		"given_name":  given,
		"family_name": family,
	}
}
