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
			"notion_user": map[string]any{"u1": mocknotion.Entry(userEntity("u1", "Alice", "B"))},
			"space":       map[string]any{"s1": mocknotion.Entry(map[string]any{"id": "s1", "name": "My Workspace"})},
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
