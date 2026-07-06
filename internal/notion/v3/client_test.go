package v3

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// capturedRequest records what the test server received.
type capturedRequest struct {
	method string
	path   string
	header http.Header
	body   map[string]any
}

// newTestClient spins up an httptest server that captures the request and
// replies with a JSON-encoded response at the given status.
func newTestClient(t *testing.T, response any, status int) (*Client, *capturedRequest) {
	t.Helper()
	cap := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.method = r.Method
		cap.path = r.URL.Path
		cap.header = r.Header.Clone()
		if raw, err := io.ReadAll(r.Body); err == nil && len(raw) > 0 {
			_ = json.Unmarshal(raw, &cap.body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(srv.Close)

	c := &Client{
		HTTP:    srv.Client(),
		BaseURL: srv.URL + "/api/v3",
		TokenV2: "fake-token",
		UserID:  "user-aaa",
		SpaceID: "space-bbb",
	}
	return c, cap
}

func ctx() context.Context { return context.Background() }

// =============================================================================
// Headers
// =============================================================================

func TestHeadersOnEveryRequest(t *testing.T) {
	c, cap := newTestClient(t, RecordMapResponse{}, 200)
	if _, err := c.LoadUserContent(ctx()); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"Content-Type":                "application/json",
		"Cookie":                      "token_v2=fake-token",
		"X-Notion-Active-User-Header": "user-aaa",
		"Notion-Client-Version":       "23.13.20260217.2221",
		"Notion-Audit-Log-Platform":   "web",
	}
	for k, v := range want {
		if got := cap.header.Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
	// Non-stream, non-collection requests must not carry the space-id header.
	if got := cap.header.Get("x-notion-space-id"); got != "" {
		t.Errorf("unexpected x-notion-space-id on loadUserContent: %q", got)
	}
}

func TestQueryCollectionAddsSpaceIDHeader(t *testing.T) {
	c, cap := newTestClient(t, map[string]any{"result": map[string]any{"blockIds": []string{}}, "recordMap": map[string]any{}}, 200)
	if _, err := c.QueryCollection(ctx(), QueryCollectionParams{CollectionID: "col-1", CollectionViewID: "view-1"}); err != nil {
		t.Fatal(err)
	}
	if got := cap.header.Get("x-notion-space-id"); got != "space-bbb" {
		t.Errorf("x-notion-space-id = %q, want space-bbb", got)
	}
}

// =============================================================================
// loadPageChunk — request format
// =============================================================================

func TestLoadPageChunkRequestFormat(t *testing.T) {
	c, cap := newTestClient(t, LoadPageChunkResponse{}, 200)
	if _, err := c.LoadPageChunk(ctx(), LoadPageChunkParams{PageID: "page-123", Limit: 10}); err != nil {
		t.Fatal(err)
	}

	if cap.path != "/api/v3/loadPageChunk" {
		t.Errorf("path = %q", cap.path)
	}
	// New format: page object with id + spaceId, not a flat pageId.
	if got := cap.body["page"]; !reflect.DeepEqual(got, map[string]any{"id": "page-123", "spaceId": "space-bbb"}) {
		t.Errorf("page = %#v", got)
	}
	if _, ok := cap.body["pageId"]; ok {
		t.Error("flat pageId should not be present")
	}
	if cap.body["limit"] != float64(10) {
		t.Errorf("limit = %#v, want 10", cap.body["limit"])
	}
	if cap.body["verticalColumns"] != false {
		t.Errorf("verticalColumns = %#v, want false", cap.body["verticalColumns"])
	}
}

func TestLoadPageChunkDefaultsAndCursor(t *testing.T) {
	c, cap := newTestClient(t, LoadPageChunkResponse{}, 200)
	cursor := &Cursor{Stack: []any{[]any{"some-cursor"}}}
	if _, err := c.LoadPageChunk(ctx(), LoadPageChunkParams{PageID: "page-456", Limit: 50, Cursor: cursor, ChunkNumber: 2}); err != nil {
		t.Fatal(err)
	}
	if cap.body["limit"] != float64(50) {
		t.Errorf("limit = %#v", cap.body["limit"])
	}
	if cap.body["chunkNumber"] != float64(2) {
		t.Errorf("chunkNumber = %#v", cap.body["chunkNumber"])
	}
	if got := cap.body["cursor"]; !reflect.DeepEqual(got, map[string]any{"stack": []any{[]any{"some-cursor"}}}) {
		t.Errorf("cursor = %#v", got)
	}

	// Default limit / cursor when unset.
	c2, cap2 := newTestClient(t, LoadPageChunkResponse{}, 200)
	if _, err := c2.LoadPageChunk(ctx(), LoadPageChunkParams{PageID: "p"}); err != nil {
		t.Fatal(err)
	}
	if cap2.body["limit"] != float64(100) {
		t.Errorf("default limit = %#v, want 100", cap2.body["limit"])
	}
	if got := cap2.body["cursor"]; !reflect.DeepEqual(got, map[string]any{"stack": []any{}}) {
		t.Errorf("default cursor = %#v", got)
	}
}

// =============================================================================
// queryCollection — request shape + response normalization
// =============================================================================

func TestQueryCollectionLoaderAndTimeZone(t *testing.T) {
	c, cap := newTestClient(t, map[string]any{"result": map[string]any{"blockIds": []string{}}, "recordMap": map[string]any{}}, 200)
	if _, err := c.QueryCollection(ctx(), QueryCollectionParams{CollectionID: "col-1", CollectionViewID: "view-1"}); err != nil {
		t.Fatal(err)
	}

	loader, ok := cap.body["loader"].(map[string]any)
	if !ok {
		t.Fatalf("loader missing: %#v", cap.body["loader"])
	}
	if loader["type"] != "reducer" {
		t.Errorf("loader.type = %#v", loader["type"])
	}
	if loader["userTimeZone"] != "UTC" {
		t.Errorf("userTimeZone = %#v, want UTC (default)", loader["userTimeZone"])
	}
	reducers := loader["reducers"].(map[string]any)
	cgr := reducers["collection_group_results"].(map[string]any)
	if cgr["limit"] != float64(999) {
		t.Errorf("default reducer limit = %#v, want 999", cgr["limit"])
	}
	// query2 present but empty when no filter/sort.
	if got := cap.body["query2"]; !reflect.DeepEqual(got, map[string]any{}) {
		t.Errorf("query2 = %#v, want empty object", got)
	}
}

func TestQueryCollectionUsesConfiguredTimeZone(t *testing.T) {
	c, cap := newTestClient(t, map[string]any{"result": map[string]any{"blockIds": []string{}}, "recordMap": map[string]any{}}, 200)
	c.UserTimeZone = "America/New_York"
	if _, err := c.QueryCollection(ctx(), QueryCollectionParams{CollectionID: "c", CollectionViewID: "v"}); err != nil {
		t.Fatal(err)
	}
	loader := cap.body["loader"].(map[string]any)
	if loader["userTimeZone"] != "America/New_York" {
		t.Errorf("userTimeZone = %#v", loader["userTimeZone"])
	}
}

func TestQueryCollectionFilterAndSort(t *testing.T) {
	c, cap := newTestClient(t, map[string]any{"result": map[string]any{"blockIds": []string{}}, "recordMap": map[string]any{}}, 200)
	filter := map[string]any{"property": "Status"}
	sort := []any{map[string]any{"direction": "ascending"}}
	if _, err := c.QueryCollection(ctx(), QueryCollectionParams{CollectionID: "c", CollectionViewID: "v", Filter: filter, Sort: sort}); err != nil {
		t.Fatal(err)
	}
	query2 := cap.body["query2"].(map[string]any)
	if !reflect.DeepEqual(query2["filter"], filter) {
		t.Errorf("query2.filter = %#v", query2["filter"])
	}
	if !reflect.DeepEqual(query2["sort"], sort) {
		t.Errorf("query2.sort = %#v", query2["sort"])
	}
}

func TestQueryCollectionNewReducerShape(t *testing.T) {
	resp := map[string]any{
		"result": map[string]any{
			"type": "reducer",
			"reducerResults": map[string]any{
				"collection_group_results": map[string]any{
					"type":     "results",
					"blockIds": []string{"row-1", "row-2"},
					"hasMore":  false,
				},
			},
			"sizeHint": 2,
		},
		"recordMap": map[string]any{},
	}
	c, _ := newTestClient(t, resp, 200)
	got, err := c.QueryCollection(ctx(), QueryCollectionParams{CollectionID: "c", CollectionViewID: "v"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.BlockIDs, []string{"row-1", "row-2"}) {
		t.Errorf("blockIDs = %#v", got.BlockIDs)
	}
	if got.Total != 2 {
		t.Errorf("total = %d, want 2 (fallback to len)", got.Total)
	}
}

func TestQueryCollectionOldFlatShape(t *testing.T) {
	resp := map[string]any{
		"result":    map[string]any{"blockIds": []string{"row-1"}, "total": 1},
		"recordMap": map[string]any{},
	}
	c, _ := newTestClient(t, resp, 200)
	got, err := c.QueryCollection(ctx(), QueryCollectionParams{CollectionID: "c", CollectionViewID: "v"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.BlockIDs, []string{"row-1"}) || got.Total != 1 {
		t.Errorf("blockIDs=%#v total=%d", got.BlockIDs, got.Total)
	}
}

func TestQueryCollectionEmptyShape(t *testing.T) {
	resp := map[string]any{
		"result":    map[string]any{"type": "reducer", "reducerResults": map[string]any{}},
		"recordMap": map[string]any{},
	}
	c, _ := newTestClient(t, resp, 200)
	got, err := c.QueryCollection(ctx(), QueryCollectionParams{CollectionID: "c", CollectionViewID: "v"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.BlockIDs) != 0 || got.Total != 0 {
		t.Errorf("blockIDs=%#v total=%d, want empty/0", got.BlockIDs, got.Total)
	}
	if got.BlockIDs == nil {
		t.Error("blockIDs should be non-nil empty slice")
	}
}

// =============================================================================
// search — request defaults
// =============================================================================

func TestSearchDefaults(t *testing.T) {
	c, cap := newTestClient(t, SearchResponse{}, 200)
	if _, err := c.Search(ctx(), SearchParams{Query: "hello"}); err != nil {
		t.Fatal(err)
	}
	if cap.body["type"] != "BlocksInSpace" {
		t.Errorf("type = %#v, want BlocksInSpace", cap.body["type"])
	}
	if cap.body["spaceId"] != "space-bbb" {
		t.Errorf("spaceId = %#v", cap.body["spaceId"])
	}
	if cap.body["limit"] != float64(20) {
		t.Errorf("limit = %#v, want 20", cap.body["limit"])
	}
	filters := cap.body["filters"].(map[string]any)
	if filters["isDeletedOnly"] != false || filters["excludeTemplates"] != false {
		t.Errorf("default filters = %#v", filters)
	}
	if got := filters["ancestors"]; !reflect.DeepEqual(got, []any{}) {
		t.Errorf("ancestors = %#v, want []", got)
	}
}

func TestSearchAncestorScopeAndFilterMerge(t *testing.T) {
	c, cap := newTestClient(t, SearchResponse{}, 200)
	if _, err := c.Search(ctx(), SearchParams{
		Query:      "q",
		AncestorID: "anc-1",
		Filters:    map[string]any{"isDeletedOnly": true},
	}); err != nil {
		t.Fatal(err)
	}
	if cap.body["type"] != "BlocksInAncestor" {
		t.Errorf("type = %#v, want BlocksInAncestor", cap.body["type"])
	}
	if cap.body["ancestorId"] != "anc-1" {
		t.Errorf("ancestorId = %#v", cap.body["ancestorId"])
	}
	if _, ok := cap.body["spaceId"]; ok {
		t.Error("spaceId should be absent when ancestorId is set")
	}
	filters := cap.body["filters"].(map[string]any)
	if filters["isDeletedOnly"] != true {
		t.Errorf("merged filter isDeletedOnly = %#v, want true", filters["isDeletedOnly"])
	}
}

// =============================================================================
// saveTransactions — transaction envelope with stubbed UUIDs
// =============================================================================

func TestSaveTransactionsEnvelope(t *testing.T) {
	orig := newUUID
	t.Cleanup(func() { newUUID = orig })
	var seq int
	newUUID = func() string {
		seq++
		return "uuid-" + string(rune('0'+seq))
	}

	c, cap := newTestClient(t, RecordMapResponse{}, 200)
	ops := []Operation{{Pointer: Pointer{Table: "block", ID: "b1", SpaceID: "space-bbb"}, Path: []string{}, Command: "set", Args: map[string]any{"x": 1}}}
	if err := c.SaveTransactions(ctx(), ops); err != nil {
		t.Fatal(err)
	}

	if cap.body["requestId"] != "uuid-1" {
		t.Errorf("requestId = %#v, want uuid-1", cap.body["requestId"])
	}
	txns := cap.body["transactions"].([]any)
	if len(txns) != 1 {
		t.Fatalf("transactions len = %d", len(txns))
	}
	txn := txns[0].(map[string]any)
	if txn["id"] != "uuid-2" {
		t.Errorf("transaction id = %#v, want uuid-2", txn["id"])
	}
	if txn["spaceId"] != "space-bbb" {
		t.Errorf("transaction spaceId = %#v", txn["spaceId"])
	}
	if got := txn["operations"].([]any); len(got) != 1 {
		t.Errorf("operations len = %d, want 1", len(got))
	}
}

// =============================================================================
// restoreRecord
// =============================================================================

func TestRestoreRecord(t *testing.T) {
	c, cap := newTestClient(t, RecordMapResponse{}, 200)
	if _, err := c.RestoreRecord(ctx(), RestoreRecordParams{ID: "page-xyz"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cap.path, "/api/v3/restoreRecord") {
		t.Errorf("path = %q", cap.path)
	}
	if got := cap.body["pointer"]; !reflect.DeepEqual(got, map[string]any{"table": "block", "id": "page-xyz", "spaceId": "space-bbb"}) {
		t.Errorf("pointer = %#v", got)
	}

	c2, cap2 := newTestClient(t, RecordMapResponse{}, 200)
	if _, err := c2.RestoreRecord(ctx(), RestoreRecordParams{ID: "row-1", Table: "collection"}); err != nil {
		t.Fatal(err)
	}
	if cap2.body["pointer"].(map[string]any)["table"] != "collection" {
		t.Errorf("table override not applied: %#v", cap2.body["pointer"])
	}
}

// =============================================================================
// syncRecordValues / syncRecordValuesForPointers
// =============================================================================

func TestSyncRecordValuesForPointers(t *testing.T) {
	c, cap := newTestClient(t, RecordMapResponse{}, 200)
	if _, err := c.SyncRecordValuesForPointers(ctx(), []SyncPointer{
		{ID: "b1", Table: "block"},
		{ID: "c1", Table: "collection", SpaceID: "space-bbb"},
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cap.path, "/api/v3/syncRecordValuesMain") {
		t.Errorf("path = %q", cap.path)
	}
	requests := cap.body["requests"].([]any)
	if len(requests) != 2 {
		t.Fatalf("requests len = %d", len(requests))
	}
	first := requests[0].(map[string]any)
	if first["version"] != float64(-1) {
		t.Errorf("version = %#v, want -1", first["version"])
	}
	ptr := first["pointer"].(map[string]any)
	if ptr["id"] != "b1" || ptr["table"] != "block" {
		t.Errorf("pointer = %#v", ptr)
	}
	if _, ok := ptr["spaceId"]; ok {
		t.Error("spaceId should be omitted when empty")
	}
	// Second pointer carries spaceId.
	if requests[1].(map[string]any)["pointer"].(map[string]any)["spaceId"] != "space-bbb" {
		t.Error("second pointer should include spaceId")
	}
}

// =============================================================================
// getActivityLog — optional fields
// =============================================================================

func TestGetActivityLogOptionalFields(t *testing.T) {
	c, cap := newTestClient(t, GetActivityLogResponse{}, 200)
	if _, err := c.GetActivityLog(ctx(), GetActivityLogParams{}); err != nil {
		t.Fatal(err)
	}
	if cap.body["spaceId"] != "space-bbb" {
		t.Errorf("spaceId = %#v", cap.body["spaceId"])
	}
	if cap.body["limit"] != float64(20) {
		t.Errorf("default limit = %#v, want 20", cap.body["limit"])
	}
	if _, ok := cap.body["navigableBlockId"]; ok {
		t.Error("navigableBlockId should be omitted when empty")
	}
	if _, ok := cap.body["startingAfterId"]; ok {
		t.Error("startingAfterId should be omitted when empty")
	}

	c2, cap2 := newTestClient(t, GetActivityLogResponse{}, 200)
	if _, err := c2.GetActivityLog(ctx(), GetActivityLogParams{NavigableBlockID: "blk-1", Limit: 5, StartingAfterID: "act-9"}); err != nil {
		t.Fatal(err)
	}
	if cap2.body["navigableBlockId"] != "blk-1" || cap2.body["startingAfterId"] != "act-9" || cap2.body["limit"] != float64(5) {
		t.Errorf("optional fields not set: %#v", cap2.body)
	}
}

// =============================================================================
// Response decoding round-trips (recordMap normalization at the boundary)
// =============================================================================

func TestLoadUserContentDecodesRecordMap(t *testing.T) {
	// New space-id-wrapped entry shape must normalize via Entry.UnmarshalJSON.
	resp := map[string]any{
		"recordMap": map[string]any{
			"block": map[string]any{
				"page-1": map[string]any{
					"spaceId": "space-bbb",
					"value":   map[string]any{"value": map[string]any{"id": "page-1", "type": "page"}, "role": "reader"},
				},
			},
		},
	}
	c, _ := newTestClient(t, resp, 200)
	got, err := c.LoadUserContent(ctx())
	if err != nil {
		t.Fatal(err)
	}
	b, ok := got.RecordMap.GetBlock("page-1")
	if !ok || b.ID != "page-1" || b.Type != "page" {
		t.Errorf("block = %+v ok=%v", b, ok)
	}
}

func TestGetTasksDecodes(t *testing.T) {
	resp := GetTasksResponse{Results: []ExportTask{
		{ID: "t1", EventName: "exportBlock", State: "success", Status: &ExportTaskStatus{Type: "complete", PagesExported: 3, ExportURL: "https://example.com/export.zip"}},
	}}
	c, cap := newTestClient(t, resp, 200)
	got, err := c.GetTasks(ctx(), []string{"t1"})
	if err != nil {
		t.Fatal(err)
	}
	if ids := cap.body["taskIds"].([]any); len(ids) != 1 || ids[0] != "t1" {
		t.Errorf("taskIds = %#v", cap.body["taskIds"])
	}
	if len(got.Results) != 1 || got.Results[0].Status.ExportURL != "https://example.com/export.zip" {
		t.Errorf("results = %#v", got.Results)
	}
}

func TestEnqueueTaskRequestAndResponse(t *testing.T) {
	c, cap := newTestClient(t, EnqueueTaskResponse{TaskID: "task-1"}, 200)
	got, err := c.EnqueueTask(ctx(), EnqueueTaskParams{EventName: "exportBlock", Request: map[string]any{"blockId": "b1"}})
	if err != nil {
		t.Fatal(err)
	}
	task := cap.body["task"].(map[string]any)
	if task["eventName"] != "exportBlock" {
		t.Errorf("eventName = %#v", task["eventName"])
	}
	if got.TaskID != "task-1" {
		t.Errorf("taskId = %q", got.TaskID)
	}
}

func TestGetBacklinksAndSnapshots(t *testing.T) {
	c, cap := newTestClient(t, GetBacklinksResponse{
		Backlinks: []Backlink{{BlockID: "target", MentionedFrom: BacklinkMention{BlockID: "block-1", Table: "block"}}},
	}, 200)
	got, err := c.GetBacklinksForBlock(ctx(), "target")
	if err != nil {
		t.Fatal(err)
	}
	if cap.body["block"].(map[string]any)["id"] != "target" {
		t.Errorf("block id = %#v", cap.body["block"])
	}
	if len(got.Backlinks) != 1 || got.Backlinks[0].MentionedFrom.BlockID != "block-1" {
		t.Errorf("backlinks = %#v", got.Backlinks)
	}

	c2, cap2 := newTestClient(t, GetSnapshotsResponse{Snapshots: []Snapshot{{ID: "snap-1", Version: 10}}}, 200)
	gs, err := c2.GetSnapshotsList(ctx(), GetSnapshotsListParams{BlockID: "b1"})
	if err != nil {
		t.Fatal(err)
	}
	if cap2.body["size"] != float64(20) {
		t.Errorf("default size = %#v, want 20", cap2.body["size"])
	}
	if len(gs.Snapshots) != 1 || gs.Snapshots[0].ID != "snap-1" {
		t.Errorf("snapshots = %#v", gs.Snapshots)
	}
}

// =============================================================================
// Errors
// =============================================================================

func TestHTTPErrorShape(t *testing.T) {
	c, _ := newTestClient(t, map[string]any{"errorId": "x", "name": "ValidationError"}, 400)
	_, err := c.LoadUserContent(ctx())
	if err == nil {
		t.Fatal("expected error")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error is not *HTTPError: %T", err)
	}
	if httpErr.Status != 400 {
		t.Errorf("status = %d, want 400", httpErr.Status)
	}
	if httpErr.Endpoint != "loadUserContent" {
		t.Errorf("endpoint = %q", httpErr.Endpoint)
	}
	if !strings.HasPrefix(httpErr.Error(), "v3 API error: 400 Bad Request") {
		t.Errorf("message = %q", httpErr.Error())
	}
	if !strings.Contains(httpErr.Error(), "ValidationError") {
		t.Errorf("message should include body snippet: %q", httpErr.Error())
	}
}

// =============================================================================
// PostStream
// =============================================================================

func TestPostStreamHeadersAndBody(t *testing.T) {
	var gotHeader http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, "{\"type\":\"a\"}\n{\"type\":\"b\"}\n")
	}))
	t.Cleanup(srv.Close)

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL + "/api/v3", TokenV2: "fake-token", UserID: "user-aaa", SpaceID: "space-bbb"}
	rc, err := c.PostStream(ctx(), "runInferenceTranscript", map[string]any{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()

	if gotHeader.Get("Accept") != "application/x-ndjson" {
		t.Errorf("Accept = %q", gotHeader.Get("Accept"))
	}
	if gotHeader.Get("x-notion-space-id") != "space-bbb" {
		t.Errorf("x-notion-space-id = %q", gotHeader.Get("x-notion-space-id"))
	}

	var types []string
	if err := ParseNDJSON(rc, func(raw json.RawMessage) error {
		var v struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		types = append(types, v.Type)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(types, []string{"a", "b"}) {
		t.Errorf("streamed types = %#v", types)
	}
}

func TestPostStreamPropagatesHTTPError(t *testing.T) {
	c, _ := newTestClient(t, map[string]any{"name": "Unauthorized"}, 401)
	_, err := c.PostStream(ctx(), "runInferenceTranscript", map[string]any{})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 401 {
		t.Fatalf("expected 401 HTTPError, got %v", err)
	}
}
