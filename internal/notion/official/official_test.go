package official

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-notion/internal/notion"
)

// capture records the last request a test server received.
type capture struct {
	method string
	path   string
	query  string
	body   map[string]any
	authOK bool
	versOK bool
}

// decodeBody parses a request's JSON body into a map (empty for no body).
func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, _ := io.ReadAll(r.Body)
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad request body: %v", err)
	}
	return m
}

// record fills a capture from the request and asserts auth/version headers.
func record(t *testing.T, cap *capture, r *http.Request) {
	t.Helper()
	cap.method = r.Method
	cap.path = r.URL.Path
	cap.query = r.URL.RawQuery
	cap.body = decodeBody(t, r)
	cap.authOK = r.Header.Get("Authorization") == "Bearer tkn"
	cap.versOK = r.Header.Get("Notion-Version") == APIVersion
}

func testClient(url string) Client { return Client{BaseURL: url, Token: "tkn"} }

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode: %v", err)
	}
}

func TestSearch(t *testing.T) {
	var cap capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(t, &cap, r)
		writeJSON(t, w, map[string]any{
			"has_more":    false,
			"next_cursor": nil,
			"results": []any{
				map[string]any{"object": "page", "id": "pg1", "url": "u",
					"properties": map[string]any{"Name": map[string]any{"type": "title", "title": []any{map[string]any{"plain_text": "Hi"}}}}},
			},
		})
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).Search(context.Background(), notion.SearchParams{Query: "hi", Filter: "page", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPost || cap.path != "/v1/search" || !cap.authOK || !cap.versOK {
		t.Errorf("request = %+v", cap)
	}
	if cap.body["query"] != "hi" || cap.body["page_size"] != float64(10) {
		t.Errorf("body = %#v", cap.body)
	}
	if filter := cap.body["filter"].(map[string]any); filter["value"] != "page" {
		t.Errorf("filter = %#v", filter)
	}
	if len(got.Items) != 1 || got.Items[0].Title != "Hi" || got.HasMore {
		t.Errorf("result = %#v", got)
	}
}

func TestSearchNextCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"has_more": true, "next_cursor": "cur2", "results": []any{}})
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).Search(context.Background(), notion.SearchParams{Query: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasMore || got.NextCursor != "cur2" {
		t.Errorf("pagination = %+v", got)
	}
}

func TestQueryDatabase(t *testing.T) {
	var cap capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(t, &cap, r)
		writeJSON(t, w, map[string]any{"has_more": false, "next_cursor": nil, "results": []any{
			map[string]any{"id": "row1", "url": "u", "properties": map[string]any{
				"Count": map[string]any{"type": "number", "number": float64(5)}}},
		}})
	}))
	defer srv.Close()

	filter := map[string]any{"property": "Count", "number": map[string]any{"greater_than": 0}}
	got, err := testClient(srv.URL).QueryDatabase(context.Background(), notion.QueryDatabaseParams{ID: "db1", Filter: filter, Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPost || cap.path != "/v1/databases/db1/query" {
		t.Errorf("request = %+v", cap)
	}
	if cap.body["filter"] == nil || cap.body["page_size"] != float64(25) {
		t.Errorf("body = %#v", cap.body)
	}
	if len(got.Items) != 1 || got.Items[0].Properties["Count"] != float64(5) {
		t.Errorf("rows = %#v", got.Items)
	}
}

func TestGetDatabaseDefaultsPageSizeAndMethod(t *testing.T) {
	var cap capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(t, &cap, r)
		writeJSON(t, w, map[string]any{"id": "db1", "url": "u", "title": []any{map[string]any{"plain_text": "T"}},
			"properties": map[string]any{}})
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).GetDatabase(context.Background(), "db1")
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodGet || cap.path != "/v1/databases/db1" {
		t.Errorf("request = %+v", cap)
	}
	if got.Title != "T" {
		t.Errorf("db = %#v", got)
	}
}

func TestCreatePageInDatabase(t *testing.T) {
	var pageBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/databases/db1":
			writeJSON(t, w, map[string]any{"id": "db1"}) // exists => isDatabase true
		case r.Method == http.MethodPost && r.URL.Path == "/v1/pages":
			pageBody = decodeBody(t, r)
			writeJSON(t, w, map[string]any{"id": "new1", "url": "https://example.test/new1",
				"parent": map[string]any{"type": "database_id", "database_id": "db1"}, "created_time": "2024-01-01T00:00:00Z"})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).CreatePage(context.Background(), notion.CreatePageParams{
		ParentID: "db1", Title: "Task", Properties: map[string]any{"Priority": "High"}, Icon: "🚀",
	})
	if err != nil {
		t.Fatal(err)
	}
	parent := pageBody["parent"].(map[string]any)
	if parent["database_id"] != "db1" {
		t.Errorf("parent = %#v", parent)
	}
	props := pageBody["properties"].(map[string]any)
	if props["Name"] == nil {
		t.Errorf("missing Name title, props = %#v", props)
	}
	prio := props["Priority"].(map[string]any)
	if sel := prio["select"].(map[string]any); sel["name"] != "High" {
		t.Errorf("Priority = %#v", prio)
	}
	if icon := pageBody["icon"].(map[string]any); icon["emoji"] != "🚀" {
		t.Errorf("icon = %#v", icon)
	}
	if got.ID != "new1" || got.Title != "Task" || got.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("result = %#v", got)
	}
}

func TestCreatePageUnderPage(t *testing.T) {
	var pageBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/databases/parent1":
			w.WriteHeader(http.StatusNotFound) // not a database
			writeJSON(t, w, map[string]any{"object": "error", "status": 404, "code": "object_not_found"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/pages":
			pageBody = decodeBody(t, r)
			writeJSON(t, w, map[string]any{"id": "new1", "url": "u", "parent": map[string]any{"type": "page_id", "page_id": "parent1"}})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	_, err := testClient(srv.URL).CreatePage(context.Background(), notion.CreatePageParams{ParentID: "parent1", Title: "Child"})
	if err != nil {
		t.Fatal(err)
	}
	parent := pageBody["parent"].(map[string]any)
	if parent["page_id"] != "parent1" {
		t.Errorf("parent = %#v", parent)
	}
	props := pageBody["properties"].(map[string]any)
	if props["title"] == nil {
		t.Errorf("expected title property, got %#v", props)
	}
}

func TestUpdatePage(t *testing.T) {
	var cap capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(t, &cap, r)
		writeJSON(t, w, map[string]any{"id": "pg1", "url": "u", "last_edited_time": "2024-02-02T00:00:00Z"})
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).UpdatePage(context.Background(), notion.UpdatePageParams{
		ID: "pg1", Title: "New Title", Properties: map[string]any{"Name": "ignored", "Done": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPatch || cap.path != "/v1/pages/pg1" {
		t.Errorf("request = %+v", cap)
	}
	props := cap.body["properties"].(map[string]any)
	if props["title"] == nil {
		t.Errorf("expected title, props = %#v", props)
	}
	if _, ok := props["Name"]; ok {
		t.Errorf("Name should be skipped, props = %#v", props)
	}
	if done := props["Done"].(map[string]any); done["checkbox"] != true {
		t.Errorf("Done = %#v", done)
	}
	if got.LastEditedAt != "2024-02-02T00:00:00Z" {
		t.Errorf("result = %#v", got)
	}
}

func TestTrashAndRestorePage(t *testing.T) {
	var lastBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastBody = decodeBody(t, r)
		writeJSON(t, w, map[string]any{"id": "pg1"})
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	trashed, err := c.TrashPage(context.Background(), "pg1")
	if err != nil {
		t.Fatal(err)
	}
	if !trashed.Trashed || lastBody["archived"] != true {
		t.Errorf("trash body = %#v result = %#v", lastBody, trashed)
	}

	restored, err := c.RestorePage(context.Background(), "pg1")
	if err != nil {
		t.Fatal(err)
	}
	if restored.Trashed || lastBody["archived"] != false {
		t.Errorf("restore body = %#v result = %#v", lastBody, restored)
	}
}

func TestListBlocks(t *testing.T) {
	var cap capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(t, &cap, r)
		writeJSON(t, w, map[string]any{"has_more": false, "next_cursor": nil, "results": []any{
			map[string]any{"id": "b1", "type": "paragraph", "has_children": false,
				"paragraph": map[string]any{"rich_text": []any{map[string]any{"plain_text": "hi"}}}},
		}})
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).ListBlocks(context.Background(), notion.ListBlocksParams{ID: "blk1", Limit: 30, Cursor: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodGet || cap.path != "/v1/blocks/blk1/children" {
		t.Errorf("request = %+v", cap)
	}
	if cap.query != "page_size=30&start_cursor=c1" {
		t.Errorf("query = %q", cap.query)
	}
	if len(got.Items) != 1 || got.Items[0].RichText != "hi" {
		t.Errorf("blocks = %#v", got.Items)
	}
}

func TestGetAllBlocksPaginates(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			if r.URL.Query().Get("page_size") != "100" {
				t.Errorf("page_size = %q", r.URL.Query().Get("page_size"))
			}
			writeJSON(t, w, map[string]any{"has_more": true, "next_cursor": "cur2", "results": []any{
				map[string]any{"id": "b1", "type": "paragraph", "has_children": false, "paragraph": map[string]any{}}}})
			return
		}
		if r.URL.Query().Get("start_cursor") != "cur2" {
			t.Errorf("start_cursor = %q", r.URL.Query().Get("start_cursor"))
		}
		writeJSON(t, w, map[string]any{"has_more": false, "next_cursor": nil, "results": []any{
			map[string]any{"id": "b2", "type": "paragraph", "has_children": false, "paragraph": map[string]any{}}}})
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).GetAllBlocks(context.Background(), "blk1")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(got.Blocks) != 2 || got.HasMore {
		t.Errorf("calls=%d blocks=%d hasMore=%v", calls, len(got.Blocks), got.HasMore)
	}
}

func TestAppendBlocks(t *testing.T) {
	var cap capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(t, &cap, r)
		writeJSON(t, w, map[string]any{"results": []any{map[string]any{"id": "n1"}, map[string]any{"id": "n2"}}})
	}))
	defer srv.Close()

	blocks := []any{map[string]any{"type": "paragraph"}, map[string]any{"type": "divider"}}
	added, err := testClient(srv.URL).AppendBlocks(context.Background(), notion.AppendBlocksParams{ID: "blk1", Blocks: blocks})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPatch || cap.path != "/v1/blocks/blk1/children" {
		t.Errorf("request = %+v", cap)
	}
	if children := cap.body["children"].([]any); len(children) != 2 {
		t.Errorf("children = %#v", children)
	}
	if added.BlocksAdded != 2 {
		t.Errorf("added = %d", added.BlocksAdded)
	}
}

func TestUpdateBlockRetrievesType(t *testing.T) {
	var patchBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet: // type probe
			writeJSON(t, w, map[string]any{"id": "b1", "type": "heading_2"})
		case http.MethodPatch:
			patchBody = decodeBody(t, r)
			writeJSON(t, w, map[string]any{"id": "b1", "last_edited_time": "2024-03-03T00:00:00Z"})
		}
	}))
	defer srv.Close()

	content := "Updated"
	got, err := testClient(srv.URL).UpdateBlock(context.Background(), notion.UpdateBlockParams{ID: "b1", Content: &content})
	if err != nil {
		t.Fatal(err)
	}
	h2 := patchBody["heading_2"].(map[string]any)
	if h2["rich_text"] == nil {
		t.Errorf("expected heading_2.rich_text, body = %#v", patchBody)
	}
	if got.LastEditedAt != "2024-03-03T00:00:00Z" {
		t.Errorf("result = %#v", got)
	}
}

func TestDeleteBlock(t *testing.T) {
	var cap capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(t, &cap, r)
		writeJSON(t, w, map[string]any{"id": "b1"})
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).DeleteBlock(context.Background(), "b1")
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodDelete || cap.path != "/v1/blocks/b1" || !got.Deleted {
		t.Errorf("request = %+v result = %#v", cap, got)
	}
}

func TestListComments(t *testing.T) {
	var cap capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(t, &cap, r)
		writeJSON(t, w, map[string]any{"has_more": false, "next_cursor": nil, "results": []any{
			map[string]any{"id": "c1", "rich_text": []any{map[string]any{"plain_text": "hello"}},
				"created_by": map[string]any{"id": "u1", "name": "Al"}}}})
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).ListComments(context.Background(), notion.ListCommentsParams{PageID: "pg1", Limit: 15})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodGet || cap.path != "/v1/comments" {
		t.Errorf("request = %+v", cap)
	}
	if cap.query != "block_id=pg1&page_size=15" {
		t.Errorf("query = %q", cap.query)
	}
	if len(got.Items) != 1 || got.Items[0].Body != "hello" || got.Items[0].Author.Name != "Al" {
		t.Errorf("comments = %#v", got.Items)
	}
}

func TestAddComment(t *testing.T) {
	var cap capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(t, &cap, r)
		writeJSON(t, w, map[string]any{"id": "c1", "created_time": "2024-01-01T00:00:00Z",
			"rich_text": []any{map[string]any{"plain_text": "the body"}}})
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).AddComment(context.Background(), notion.AddCommentParams{PageID: "pg1", Body: "the body"})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPost || cap.path != "/v1/comments" {
		t.Errorf("request = %+v", cap)
	}
	parent := cap.body["parent"].(map[string]any)
	if parent["page_id"] != "pg1" {
		t.Errorf("parent = %#v", parent)
	}
	if got.ID != "c1" || got.Body != "the body" {
		t.Errorf("result = %#v", got)
	}
}

func TestAddCommentFallsBackToRequestBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"id": "c1"}) // no rich_text in response
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).AddComment(context.Background(), notion.AddCommentParams{PageID: "pg1", Body: "fallback text"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "fallback text" {
		t.Errorf("body = %q", got.Body)
	}
}

func TestListUsersAndGetMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/users/me" {
			writeJSON(t, w, map[string]any{"id": "bot1", "name": "Bot", "type": "bot",
				"bot": map[string]any{"workspace_name": "WS"}})
			return
		}
		if r.URL.RawQuery != "page_size=50" {
			t.Errorf("users query = %q", r.URL.RawQuery)
		}
		writeJSON(t, w, map[string]any{"has_more": false, "next_cursor": nil, "results": []any{
			map[string]any{"id": "u1", "name": "Al", "type": "person", "person": map[string]any{"email": "al@example.test"}}}})
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	users, err := c.ListUsers(context.Background(), notion.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(users.Items) != 1 || users.Items[0].Email != "al@example.test" {
		t.Errorf("users = %#v", users.Items)
	}

	me, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.ID != "bot1" || me.Type != "bot" || me.WorkspaceName != "WS" {
		t.Errorf("me = %#v", me)
	}
}

func TestAPIErrorDecoding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(t, w, map[string]any{"object": "error", "status": 400, "code": "validation_error", "message": "bad filter"})
	}))
	defer srv.Close()

	_, err := testClient(srv.URL).GetDatabase(context.Background(), "db1")
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T", err)
	}
	if apiErr.Status != 400 || apiErr.Code != "validation_error" || apiErr.Message != "bad filter" {
		t.Errorf("apiErr = %#v", apiErr)
	}
	if apiErr.Error() != "notion API error (validation_error): bad filter" {
		t.Errorf("Error() = %q", apiErr.Error())
	}
}
