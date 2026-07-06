// V3 HTTP client — POST requests to notion.so/api/v3. RecordMap normalization
// happens at the JSON decode boundary via Entry.UnmarshalJSON; there is no
// separate tree-walk normalizer to port.

package v3

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL      = "https://www.notion.so/api/v3"
	notionClientVersion = "23.13.20260217.2221"
	defaultTimeout      = 30 * time.Second
	collectionTimeout   = 60 * time.Second
)

// Client is a v3 internal-API HTTP client. The zero value is unusable
// (credentials are required), but HTTP and BaseURL default sensibly.
type Client struct {
	// HTTP is the underlying HTTP client; nil uses http.DefaultClient.
	HTTP *http.Client
	// BaseURL overrides the API root; "" uses https://www.notion.so/api/v3.
	BaseURL string

	TokenV2 string
	UserID  string
	SpaceID string

	// UserTimeZone is the IANA zone sent as queryCollection's userTimeZone.
	// The TS read this from Intl.DateTimeFormat(); Go has no runtime IANA
	// zone, so callers set it explicitly. Empty defaults to "UTC".
	UserTimeZone string
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultBaseURL
}

func (c *Client) userTimeZone() string {
	if c.UserTimeZone != "" {
		return c.UserTimeZone
	}
	return "UTC"
}

// HTTPError is a non-2xx response from the v3 API. Its message is part of the
// LLM-facing contract (surfaced verbatim), so keep the text shape stable.
type HTTPError struct {
	Status   int
	Endpoint string
	message  string
}

func (e *HTTPError) Error() string { return e.message }

func newHTTPError(resp *http.Response, endpoint string) *HTTPError {
	msg := fmt.Sprintf("v3 API error: %d %s", resp.StatusCode, reasonPhrase(resp))
	if body, _ := io.ReadAll(io.LimitReader(resp.Body, 200)); len(body) > 0 {
		msg += " — " + string(body)
	}
	return &HTTPError{Status: resp.StatusCode, Endpoint: endpoint, message: msg}
}

func reasonPhrase(resp *http.Response) string {
	if t := http.StatusText(resp.StatusCode); t != "" {
		return t
	}
	return resp.Status
}

// newUUID generates v4 UUIDs for saveTransactions. It is a package variable so
// tests can substitute a deterministic generator.
var newUUID = randomUUID

func randomUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10xx
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (c *Client) setHeaders(req *http.Request, extra map[string]string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "token_v2="+c.TokenV2)
	req.Header.Set("x-notion-active-user-header", c.UserID)
	req.Header.Set("notion-client-version", notionClientVersion)
	req.Header.Set("notion-audit-log-platform", "web")
	for k, v := range extra {
		req.Header.Set(k, v)
	}
}

// send performs the POST and returns the open response on 2xx, or an error
// (with the body closed) otherwise. Timeout management is the caller's job.
func (c *Client) send(ctx context.Context, endpoint string, body any, extra map[string]string) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+"/"+endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, extra)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		httpErr := newHTTPError(resp, endpoint)
		_ = resp.Body.Close()
		return nil, httpErr
	}
	return resp, nil
}

// post sends a request and decodes the JSON response into out (nil discards the
// body). A timeout is applied only when ctx has no deadline of its own.
func (c *Client) post(ctx context.Context, endpoint string, body, out any, timeout time.Duration, extra map[string]string) error {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	resp, err := c.send(ctx, endpoint, body, extra)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// PostStream POSTs and returns the response body for streaming consumption
// (e.g. via ParseNDJSON). The caller must Close the returned reader. Unlike
// post, no internal timeout is applied: streaming responses can be long-lived,
// so cancellation is governed entirely by ctx.
func (c *Client) PostStream(ctx context.Context, endpoint string, body any) (io.ReadCloser, error) {
	extra := map[string]string{
		"Accept":            "application/x-ndjson",
		"x-notion-space-id": c.SpaceID,
	}
	resp, err := c.send(ctx, endpoint, body, extra)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// --- Response types ---

// RecordMapResponse is the common { recordMap } envelope.
type RecordMapResponse struct {
	RecordMap RecordMap `json:"recordMap"`
}

// Cursor is loadPageChunk's pagination cursor.
type Cursor struct {
	Stack []any `json:"stack"`
}

// LoadPageChunkResponse is loadPageChunk's result.
type LoadPageChunkResponse struct {
	RecordMap RecordMap `json:"recordMap"`
	Cursor    Cursor    `json:"cursor"`
}

// SearchHighlight is the matched-text snippet on a search hit.
type SearchHighlight struct {
	Text string `json:"text"`
}

// SearchHit is one search result.
type SearchHit struct {
	ID        string           `json:"id"`
	Highlight *SearchHighlight `json:"highlight,omitempty"`
	Score     float64          `json:"score"`
}

// SearchResponse is search's result.
type SearchResponse struct {
	Results   []SearchHit `json:"results"`
	Total     int         `json:"total"`
	RecordMap RecordMap   `json:"recordMap"`
}

// QueryCollectionResult is queryCollection's normalized result: blockIds and
// total are lifted out of whichever response shape (old flat / new reducer)
// the server used.
type QueryCollectionResult struct {
	BlockIDs       []string
	Total          int
	ReducerResults json.RawMessage
	RecordMap      RecordMap
}

// EnqueueTaskResponse carries the queued task's ID.
type EnqueueTaskResponse struct {
	TaskID string `json:"taskId"`
}

// ExportTaskStatus is the progress payload on an export task.
type ExportTaskStatus struct {
	Type          string `json:"type,omitempty"`
	PagesExported int    `json:"pagesExported,omitempty"`
	ExportURL     string `json:"exportURL,omitempty"`
}

// ExportTask is one task from getTasks.
type ExportTask struct {
	ID        string            `json:"id"`
	EventName string            `json:"eventName"`
	State     string            `json:"state"` // not_started | in_progress | success | failure
	Status    *ExportTaskStatus `json:"status,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// GetTasksResponse wraps getTasks' results.
type GetTasksResponse struct {
	Results []ExportTask `json:"results"`
}

// BacklinkMention identifies where a backlink is mentioned from.
type BacklinkMention struct {
	BlockID string `json:"block_id"`
	Table   string `json:"table"`
}

// Backlink is one backlink to a block.
type Backlink struct {
	BlockID       string          `json:"block_id"`
	MentionedFrom BacklinkMention `json:"mentioned_from"`
}

// GetBacklinksResponse is getBacklinksForBlock's result.
type GetBacklinksResponse struct {
	Backlinks []Backlink `json:"backlinks"`
	RecordMap RecordMap  `json:"recordMap"`
}

// GetSnapshotsResponse is getSnapshotsList's result.
type GetSnapshotsResponse struct {
	Snapshots []Snapshot `json:"snapshots"`
}

// GetActivityLogResponse is getActivityLog's result.
type GetActivityLogResponse struct {
	ActivityIDs []string            `json:"activityIds"`
	Activities  map[string]Activity `json:"activities"`
	RecordMap   RecordMap           `json:"recordMap"`
}

// SyncPointer addresses a record for syncRecordValues.
type SyncPointer struct {
	ID      string `json:"id"`
	Table   string `json:"table"`
	SpaceID string `json:"spaceId,omitempty"`
}

// SyncRequest is one syncRecordValues request.
type SyncRequest struct {
	Pointer SyncPointer `json:"pointer"`
	Version int         `json:"version"`
}

// --- Read endpoints ---

// LoadUserContent fetches the current user's bootstrap record map.
func (c *Client) LoadUserContent(ctx context.Context) (RecordMapResponse, error) {
	var out RecordMapResponse
	err := c.post(ctx, "loadUserContent", map[string]any{}, &out, defaultTimeout, nil)
	return out, err
}

// LoadPageChunkParams configures a loadPageChunk call.
type LoadPageChunkParams struct {
	PageID      string
	Limit       int     // 0 → 100
	Cursor      *Cursor // nil → { stack: [] }
	ChunkNumber int
}

// LoadPageChunk fetches a chunk of a page's blocks.
func (c *Client) LoadPageChunk(ctx context.Context, p LoadPageChunkParams) (LoadPageChunkResponse, error) {
	cursor := map[string]any{"stack": []any{}}
	if p.Cursor != nil {
		cursor = map[string]any{"stack": p.Cursor.Stack}
	}
	body := map[string]any{
		"page":            map[string]string{"id": p.PageID, "spaceId": c.SpaceID},
		"limit":           orInt(p.Limit, 100),
		"cursor":          cursor,
		"chunkNumber":     p.ChunkNumber,
		"verticalColumns": false,
	}
	var out LoadPageChunkResponse
	err := c.post(ctx, "loadPageChunk", body, &out, defaultTimeout, nil)
	return out, err
}

// SyncRecordValues fetches specific records by pointer + version.
func (c *Client) SyncRecordValues(ctx context.Context, requests []SyncRequest) (RecordMapResponse, error) {
	var out RecordMapResponse
	err := c.post(ctx, "syncRecordValuesMain", map[string]any{"requests": requests}, &out, defaultTimeout, nil)
	return out, err
}

// SyncRecordValuesForPointers fetches records for the given pointers, each at
// version -1 (latest).
func (c *Client) SyncRecordValuesForPointers(ctx context.Context, pointers []SyncPointer) (RecordMapResponse, error) {
	requests := make([]SyncRequest, 0, len(pointers))
	for _, p := range pointers {
		requests = append(requests, SyncRequest{Pointer: p, Version: -1})
	}
	var out RecordMapResponse
	err := c.post(ctx, "syncRecordValuesMain", map[string]any{"requests": requests}, &out, defaultTimeout, nil)
	return out, err
}

// QueryCollectionParams configures a queryCollection call.
type QueryCollectionParams struct {
	CollectionID     string
	CollectionViewID string
	Filter           any // query2.filter, omitted when nil
	Sort             any // query2.sort, omitted when nil
	Limit            int // 0 → 999
	SearchQuery      string
}

// QueryCollection runs a collection view query and normalizes the block-ID
// listing across the old (flat) and new (reducer) response shapes.
func (c *Client) QueryCollection(ctx context.Context, p QueryCollectionParams) (QueryCollectionResult, error) {
	loader := map[string]any{
		"type": "reducer",
		"reducers": map[string]any{
			"collection_group_results": map[string]any{
				"type":             "results",
				"limit":            orInt(p.Limit, 999),
				"loadContentCover": false,
			},
		},
		"searchQuery":  p.SearchQuery,
		"userTimeZone": c.userTimeZone(),
	}

	query2 := map[string]any{}
	if p.Filter != nil {
		query2["filter"] = p.Filter
	}
	if p.Sort != nil {
		query2["sort"] = p.Sort
	}

	body := map[string]any{
		"collection":     map[string]string{"id": p.CollectionID},
		"collectionView": map[string]string{"id": p.CollectionViewID},
		"loader":         loader,
		"query2":         query2,
	}

	var raw struct {
		Result    json.RawMessage `json:"result"`
		RecordMap RecordMap       `json:"recordMap"`
	}
	if err := c.post(ctx, "queryCollection", body, &raw, collectionTimeout, map[string]string{"x-notion-space-id": c.SpaceID}); err != nil {
		return QueryCollectionResult{}, err
	}

	var result struct {
		BlockIDs       []string        `json:"blockIds"`
		Total          *int            `json:"total"`
		ReducerResults json.RawMessage `json:"reducerResults"`
	}
	if len(raw.Result) > 0 {
		if err := json.Unmarshal(raw.Result, &result); err != nil {
			return QueryCollectionResult{}, err
		}
	}

	var reduced struct {
		CGR struct {
			BlockIDs []string `json:"blockIds"`
			Total    *int     `json:"total"`
		} `json:"collection_group_results"`
	}
	if len(result.ReducerResults) > 0 {
		_ = json.Unmarshal(result.ReducerResults, &reduced)
	}

	// blockIds: result.blockIds ?? reducerResults.collection_group_results.blockIds ?? []
	blockIDs := result.BlockIDs
	if blockIDs == nil {
		blockIDs = reduced.CGR.BlockIDs
	}
	if blockIDs == nil {
		blockIDs = []string{}
	}

	// total: result.total ?? reducerResults.collection_group_results.total ?? len(blockIds)
	total := len(blockIDs)
	switch {
	case result.Total != nil:
		total = *result.Total
	case reduced.CGR.Total != nil:
		total = *reduced.CGR.Total
	}

	return QueryCollectionResult{
		BlockIDs:       blockIDs,
		Total:          total,
		ReducerResults: result.ReducerResults,
		RecordMap:      raw.RecordMap,
	}, nil
}

// SearchParams configures a search call.
type SearchParams struct {
	Query      string
	AncestorID string         // set → BlocksInAncestor scope
	Limit      int            // 0 → 20
	Filters    map[string]any // merged over the defaults
}

// Search runs a workspace (or ancestor-scoped) search.
func (c *Client) Search(ctx context.Context, p SearchParams) (SearchResponse, error) {
	filters := map[string]any{
		"isDeletedOnly":          false,
		"excludeTemplates":       false,
		"isNavigableOnly":        false,
		"requireEditPermissions": false,
		"ancestors":              []any{},
		"createdBy":              []any{},
		"editedBy":               []any{},
		"lastEditedTime":         map[string]any{},
		"createdTime":            map[string]any{},
	}
	for k, v := range p.Filters {
		filters[k] = v
	}

	body := map[string]any{
		"query":   p.Query,
		"limit":   orInt(p.Limit, 20),
		"sort":    map[string]string{"field": "relevance"},
		"source":  "quick_find_input_change",
		"filters": filters,
	}
	if p.AncestorID != "" {
		body["type"] = "BlocksInAncestor"
		body["ancestorId"] = p.AncestorID
	} else {
		body["type"] = "BlocksInSpace"
		body["spaceId"] = c.SpaceID
	}

	var out SearchResponse
	err := c.post(ctx, "search", body, &out, defaultTimeout, nil)
	return out, err
}

// SaveTransactions submits a batch of operations under a single transaction.
func (c *Client) SaveTransactions(ctx context.Context, operations []Operation) error {
	body := map[string]any{
		"requestId": newUUID(),
		"transactions": []map[string]any{
			{
				"id":         newUUID(),
				"spaceId":    c.SpaceID,
				"operations": operations,
			},
		},
	}
	return c.post(ctx, "saveTransactions", body, nil, defaultTimeout, nil)
}

// RestoreRecordParams identifies a trashed record to restore.
type RestoreRecordParams struct {
	ID    string
	Table string // "" → "block"
}

// RestoreRecord restores a trashed record and returns the updated record map.
func (c *Client) RestoreRecord(ctx context.Context, p RestoreRecordParams) (RecordMapResponse, error) {
	body := map[string]any{
		"pointer": map[string]string{
			"table":   orStr(p.Table, "block"),
			"id":      p.ID,
			"spaceId": c.SpaceID,
		},
	}
	var out RecordMapResponse
	err := c.post(ctx, "restoreRecord", body, &out, defaultTimeout, nil)
	return out, err
}

// --- Export endpoints ---

// EnqueueTaskParams describes a task to enqueue (e.g. an export).
type EnqueueTaskParams struct {
	EventName string
	Request   map[string]any
}

// EnqueueTask queues a background task and returns its ID.
func (c *Client) EnqueueTask(ctx context.Context, p EnqueueTaskParams) (EnqueueTaskResponse, error) {
	body := map[string]any{
		"task": map[string]any{
			"eventName": p.EventName,
			"request":   p.Request,
		},
	}
	var out EnqueueTaskResponse
	err := c.post(ctx, "enqueueTask", body, &out, defaultTimeout, nil)
	return out, err
}

// GetTasks polls the status of previously enqueued tasks.
func (c *Client) GetTasks(ctx context.Context, taskIDs []string) (GetTasksResponse, error) {
	var out GetTasksResponse
	err := c.post(ctx, "getTasks", map[string]any{"taskIds": taskIDs}, &out, defaultTimeout, nil)
	return out, err
}

// --- Backlinks ---

// GetBacklinksForBlock lists blocks that mention the given block.
func (c *Client) GetBacklinksForBlock(ctx context.Context, blockID string) (GetBacklinksResponse, error) {
	body := map[string]any{
		"block": map[string]string{"id": blockID, "spaceId": c.SpaceID},
	}
	var out GetBacklinksResponse
	err := c.post(ctx, "getBacklinksForBlock", body, &out, defaultTimeout, nil)
	return out, err
}

// --- Version history ---

// GetSnapshotsListParams configures a getSnapshotsList call.
type GetSnapshotsListParams struct {
	BlockID string
	Size    int // 0 → 20
}

// GetSnapshotsList lists a block's version-history snapshots.
func (c *Client) GetSnapshotsList(ctx context.Context, p GetSnapshotsListParams) (GetSnapshotsResponse, error) {
	body := map[string]any{
		"block": map[string]string{"id": p.BlockID, "spaceId": c.SpaceID},
		"size":  orInt(p.Size, 20),
	}
	var out GetSnapshotsResponse
	err := c.post(ctx, "getSnapshotsList", body, &out, defaultTimeout, nil)
	return out, err
}

// --- Activity log ---

// GetActivityLogParams configures a getActivityLog call.
type GetActivityLogParams struct {
	NavigableBlockID string // omitted when empty
	Limit            int    // 0 → 20
	StartingAfterID  string // omitted when empty
}

// GetActivityLog fetches the space's (or a block's) activity log.
func (c *Client) GetActivityLog(ctx context.Context, p GetActivityLogParams) (GetActivityLogResponse, error) {
	body := map[string]any{
		"spaceId": c.SpaceID,
		"limit":   orInt(p.Limit, 20),
	}
	if p.NavigableBlockID != "" {
		body["navigableBlockId"] = p.NavigableBlockID
	}
	if p.StartingAfterID != "" {
		body["startingAfterId"] = p.StartingAfterID
	}
	var out GetActivityLogResponse
	err := c.post(ctx, "getActivityLog", body, &out, defaultTimeout, nil)
	return out, err
}

// --- Small helpers ---

func orInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func orStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
