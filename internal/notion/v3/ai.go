// Notion AI domain logic — model resolution, thread content, and the streaming
// chat pipeline (request building, NDJSON event parsing/accumulation, patch
// normalization) over the v3 Client. AI is v3-only.

package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// --- Lang tag helpers ---

var (
	langTagPrefix     = regexp.MustCompile(`(?i)^<lang\s+[^>]*/>\s*`)
	incompleteLangTag = regexp.MustCompile(`(?i)^<lang\b`)
)

// StripLangTag removes Notion's leading internal language tag from a response.
func StripLangTag(s string) string {
	return langTagPrefix.ReplaceAllString(s, "")
}

// IsIncompleteLangTag reports whether s begins a lang tag still being streamed
// (opened but not yet closed with ">").
func IsIncompleteLangTag(s string) bool {
	return incompleteLangTag.MatchString(s) && !strings.Contains(s, ">")
}

// --- Model resolution ---

// ResolveModel resolves a model name (codename or display name) to its
// codename, falling back to configDefault when modelFlag is empty. Returns ""
// when neither is set (let the API pick), or an error for an unknown name.
func ResolveModel(models []AIModel, modelFlag, configDefault string) (string, error) {
	input := modelFlag
	if input == "" {
		input = configDefault
	}
	if input == "" {
		return "", nil
	}

	for _, m := range models {
		if m.Model == input {
			return m.Model, nil
		}
	}
	lower := strings.ToLower(input)
	for _, m := range models {
		if strings.ToLower(m.ModelMessage) == lower {
			return m.Model, nil
		}
	}
	for _, m := range models {
		if strings.Contains(strings.ToLower(m.ModelMessage), lower) {
			return m.Model, nil
		}
	}
	return "", fmt.Errorf("Unknown model %q. Run 'ai model list --raw' to see available model codenames.", input)
}

// --- Listing wrappers ---

// GetAvailableModels returns the AI models for a space.
func GetAvailableModels(ctx context.Context, c *Client, spaceID string) ([]AIModel, error) {
	resp, err := c.GetAvailableModels(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	return resp.Models, nil
}

// TranscriptList is the trimmed AI thread listing the CLI renders.
type TranscriptList struct {
	Transcripts     []InferenceTranscript
	UnreadThreadIDs []string
	HasMore         bool
}

// GetInferenceTranscripts lists the user's AI chat threads.
func GetInferenceTranscripts(ctx context.Context, c *Client, spaceID string, limit int) (TranscriptList, error) {
	resp, err := c.GetInferenceTranscriptsForUser(ctx, spaceID, limit)
	if err != nil {
		return TranscriptList{}, err
	}
	return TranscriptList{
		Transcripts:     resp.Transcripts,
		UnreadThreadIDs: resp.UnreadThreadIDs,
		HasMore:         resp.HasMore,
	}, nil
}

// MarkTranscriptSeen marks a chat thread as read.
func MarkTranscriptSeen(ctx context.Context, c *Client, spaceID, threadID string) (bool, error) {
	resp, err := c.MarkInferenceTranscriptSeen(ctx, spaceID, threadID)
	if err != nil {
		return false, err
	}
	return resp.OK, nil
}

// --- Thread content ---

// GetThreadContent fetches an AI chat thread and its messages. When the thread
// record cannot be found, Found is false and Messages is empty.
func GetThreadContent(ctx context.Context, c *Client, threadID, spaceID string) (ThreadContent, error) {
	threadRec := fetchRecord(ctx, c, "thread", threadID, spaceID)
	if threadRec == nil {
		return ThreadContent{Messages: []ThreadMessage{}, Found: false}, nil
	}

	title := threadTitle(threadRec)
	messageIDs := stringSlice(threadRec["messages"])
	if len(messageIDs) == 0 {
		return ThreadContent{Messages: []ThreadMessage{}, Title: title, Found: true}, nil
	}

	msgTable := fetchTable(ctx, c, "thread_message", messageIDs, spaceID)
	if msgTable == nil {
		return ThreadContent{Messages: []ThreadMessage{}, Title: title, Found: true}, nil
	}

	messages := []ThreadMessage{}
	for _, id := range messageIDs {
		entry, ok := msgTable[id]
		if !ok {
			continue
		}
		var rec map[string]any
		if json.Unmarshal(entry.Value, &rec) != nil {
			continue
		}
		step, _ := rec["step"].(map[string]any)
		if step == nil {
			continue
		}
		stepType, _ := step["type"].(string)
		if msg, ok := ParseThreadMessage(id, stepType, step, rec); ok {
			messages = append(messages, msg)
		}
	}

	return ThreadContent{Messages: messages, Title: title, Found: true}, nil
}

// fetchRecord syncs a single record and returns its entity as a generic map.
func fetchRecord(ctx context.Context, c *Client, table, id, spaceID string) map[string]any {
	resp, err := c.SyncRecordValuesForPointers(ctx, []SyncPointer{{ID: id, Table: table, SpaceID: spaceID}})
	if err != nil {
		return nil
	}
	entry, ok := resp.RecordMap[table][id]
	if !ok {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(entry.Value, &m) != nil {
		return nil
	}
	return m
}

// fetchTable syncs a set of records and returns the named table (nil on error).
func fetchTable(ctx context.Context, c *Client, table string, ids []string, spaceID string) Table {
	pointers := make([]SyncPointer, 0, len(ids))
	for _, id := range ids {
		pointers = append(pointers, SyncPointer{ID: id, Table: table, SpaceID: spaceID})
	}
	resp, err := c.SyncRecordValuesForPointers(ctx, pointers)
	if err != nil {
		return nil
	}
	return resp.RecordMap[table]
}

// threadTitle reads data.title, falling back to title.
func threadTitle(rec map[string]any) string {
	if data, ok := rec["data"].(map[string]any); ok {
		if t, ok := data["title"].(string); ok {
			return t
		}
	}
	if t, ok := rec["title"].(string); ok {
		return t
	}
	return ""
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ParseThreadMessage converts a thread_message step into a ThreadMessage.
// ok is false for skipped step types (config, context, title, record-map, …).
func ParseThreadMessage(id, stepType string, step, rec map[string]any) (ThreadMessage, bool) {
	var createdAt *int64
	if v, ok := rec["created_time"].(float64); ok {
		t := int64(v)
		createdAt = &t
	}

	switch stepType {
	case "user":
		content, _ := ExtractRichText(step["value"])
		return ThreadMessage{ID: id, Role: "user", Content: content, CreatedAt: createdAt}, true

	case "agent-inference":
		content := ""
		if arr, ok := step["value"].([]any); ok {
			for _, v := range arr {
				m, ok := v.(map[string]any)
				if !ok || m["type"] != "text" {
					continue
				}
				content, _ = m["content"].(string)
				break
			}
		}
		return ThreadMessage{ID: id, Role: "assistant", Content: StripLangTag(content), CreatedAt: createdAt}, true

	case "agent-tool-result":
		toolName, _ := step["toolName"].(string)
		state, _ := step["state"].(string)
		errStr, _ := step["error"].(string)
		content := fmt.Sprintf(`Tool "%s" completed`, toolName)
		if errStr != "" {
			content = fmt.Sprintf(`Tool "%s" failed: %s`, toolName, errStr)
		}
		return ThreadMessage{ID: id, Role: "tool", Content: content, CreatedAt: createdAt, ToolName: toolName, ToolState: state}, true

	default:
		return ThreadMessage{}, false
	}
}

// ExtractRichText pulls plain text from Notion's [["a"],["b"]] rich-text form
// or from a bare string. ok is false when the value is neither.
func ExtractRichText(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	if s, ok := value.(string); ok {
		return s, true
	}
	arr, ok := value.([]any)
	if !ok || len(arr) == 0 {
		return "", false
	}
	first, ok := arr[0].([]any)
	if !ok || len(first) == 0 {
		return "", false
	}
	if _, ok := first[0].(string); !ok {
		return "", false
	}
	var b strings.Builder
	for _, v := range arr {
		if inner, ok := v.([]any); ok && len(inner) > 0 {
			if s, ok := inner[0].(string); ok {
				b.WriteString(s)
			}
		}
	}
	return b.String(), true
}

// --- Streaming chat ---

// RunInferenceStream builds the request, opens the NDJSON stream, and invokes
// handle for each normalized event as it arrives. Thread replies (patch
// responses) are normalized into synthetic agent-inference events. handle
// returning an error stops the stream.
func RunInferenceStream(ctx context.Context, c *Client, params RunInferenceParams, handle func(NdjsonEvent) error) error {
	body, isNewThread := buildInferenceRequest(params, c.userTimeZone(), time.Now(), newUUID)

	// No internal deadline: an AI response can stream for a long time. The TS
	// STREAM_TIMEOUT guarded only the header phase (its timer was cleared once
	// headers arrived); the caller's ctx governs cancellation here.
	stream, err := c.PostStream(ctx, "runInferenceTranscript", body)
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()

	if isNewThread {
		return ParseNDJSON(stream, func(raw json.RawMessage) error {
			if e, ok := decodeEvent(raw); ok {
				return handle(e)
			}
			return nil
		})
	}

	pn := &patchNormalizer{emit: handle}
	return ParseNDJSON(stream, func(raw json.RawMessage) error {
		if e, ok := decodeEvent(raw); ok {
			return pn.handle(e)
		}
		return nil
	})
}

// RunInferenceChat runs an inference, streaming text deltas to onChunk (may be
// nil) and returning the accumulated result. This is the convenience path for
// the CLI (live tokens) and MCP (buffered).
func RunInferenceChat(ctx context.Context, c *Client, params RunInferenceParams, onChunk func(string)) (ChatResult, error) {
	acc := &streamAccumulator{onChunk: onChunk}
	if err := RunInferenceStream(ctx, c, params, acc.handle); err != nil {
		return ChatResult{}, err
	}
	return acc.result(), nil
}

// buildInferenceRequest builds the request body and reports whether this starts
// a new thread. It is pure given the timezone, clock, and UUID source.
func buildInferenceRequest(params RunInferenceParams, tz string, now time.Time, uuid func() string) (RunInferenceTranscriptRequest, bool) {
	traceID := uuid()
	threadID := params.ThreadID
	if threadID == "" {
		threadID = uuid()
	}
	isNewThread := params.ThreadID == ""
	if params.IsNewThread != nil {
		isNewThread = *params.IsNewThread
	}

	nowISO := MsToISO(now.UnixMilli())

	req := RunInferenceTranscriptRequest{
		TraceID:                       traceID,
		SpaceID:                       params.Space.ID,
		Transcript:                    buildTranscriptItems(params, tz, nowISO, uuid),
		ThreadID:                      threadID,
		CreateThread:                  isNewThread,
		GenerateTitle:                 isNewThread,
		SaveAllThreadOperations:       true,
		ThreadType:                    "workflow",
		IsPartialTranscript:           !isNewThread,
		AsPatchResponse:               !isNewThread,
		IsUserInAnySalesAssistedSpace: true,
		IsSpaceSalesAssisted:          true,
		DebugOverrides: &DebugOverrides{
			EmitAgentSearchExtractedResults: true,
			CachedInferences:                map[string]any{},
			AnnotationInferences:            map[string]any{},
			EmitInferences:                  false,
		},
	}
	if isNewThread {
		req.ThreadParentPointer = &ThreadParentPointer{Table: "space", ID: params.Space.ID, SpaceID: params.Space.ID}
	}
	return req, isNewThread
}

// defaultConfigFlags is the fixed workflow feature-flag block spread into the
// config transcript item. Names are the API contract — do not rename.
var defaultConfigFlags = map[string]bool{
	"isCustomAgent":                     false,
	"isCustomAgentBuilder":              false,
	"useCustomAgentDraft":               false,
	"use_draft_actor_pointer":           false,
	"enableAgentDiffs":                  true,
	"enableTurnLevelTitleDiff":          true,
	"enableAgentCreateDbTemplate":       true,
	"enableCsvAttachmentSupport":        true,
	"enableAgentCardCustomization":      true,
	"enableUpdatePageV2Tool":            true,
	"enableUpdatePageAutofixer":         true,
	"enableUpdatePageOrderUpdates":      true,
	"enableAgentSupportPropertyReorder": true,
	"useServerUndo":                     true,
	"useReadOnlyMode":                   false,
	"useSearchToolV2":                   false,
	"enableAgentAutomations":            false,
	"enableAgentIntegrations":           false,
	"enableCustomAgents":                false,
	"enableExperimentalIntegrations":    false,
	"enableAgentViewNotificationsTool":  false,
	"enableDatabaseAgents":              false,
	"enableAgentThreadTools":            false,
	"enableRunAgentTool":                false,
	"enableSetupModeTool":               false,
	"enableAgentDashboards":             false,
	"enableSystemPromptAsPage":          false,
	"enableUserSessionContext":          false,
	"enableScriptAgentAdvanced":         false,
	"enableScriptAgent":                 false,
	"enableScriptAgentIntegrations":     false,
	"enableScriptAgentCustomAgentTools": false,
	"enableAgentGenerateImage":          false,
	"enableSpeculativeSearch":           false,
	"enableQueryCalendar":               false,
	"enableQueryMail":                   false,
	"enableMailExplicitToolCalls":       false,
	"enableAgentVerification":           false,
	"enableUpdatePageMarkdownTree":      false,
	"databaseAgentConfigMode":           false,
}

// buildTranscriptItems assembles the config, context, and user transcript items.
func buildTranscriptItems(params RunInferenceParams, tz, nowISO string, uuid func() string) []any {
	searchScopes := []any{}
	if !params.NoSearch {
		searchScopes = []any{map[string]any{"type": "everything"}}
	}
	configValue := map[string]any{
		"type":                "workflow",
		"availableConnectors": []any{},
		"searchScopes":        searchScopes,
		"useWebSearch":        !params.NoSearch,
		"writerMode":          false,
		"modelFromUser":       params.Model != "",
	}
	if params.Model != "" {
		configValue["model"] = params.Model
	}
	for k, v := range defaultConfigFlags {
		configValue[k] = v
	}
	configItem := map[string]any{"id": uuid(), "type": "config", "value": configValue}

	contextValue := map[string]any{
		"timezone":        tz,
		"userName":        params.User.Name,
		"userId":          params.User.ID,
		"userEmail":       params.User.Email,
		"spaceName":       params.Space.Name,
		"spaceId":         params.Space.ID,
		"currentDatetime": nowISO,
		"surface":         "workflows",
	}
	if params.Space.ViewID != "" {
		contextValue["spaceViewId"] = params.Space.ViewID
	}
	if params.PageID != "" {
		contextValue["blockId"] = params.PageID
	}
	contextItem := map[string]any{"id": uuid(), "neverCompress": true, "type": "context", "value": contextValue}

	userItem := map[string]any{
		"id":        uuid(),
		"type":      "user",
		"value":     []any{[]any{params.Message}},
		"userId":    params.User.ID,
		"createdAt": nowISO,
	}

	return []any{configItem, contextItem, userItem}
}

// --- Stream processing ---

// ProcessInferenceStream accumulates a slice of stream events into a ChatResult,
// invoking onChunk (may be nil) for each new text delta. RunInferenceChat is the
// live equivalent over an open stream.
func ProcessInferenceStream(events []NdjsonEvent, onChunk func(string)) ChatResult {
	acc := &streamAccumulator{onChunk: onChunk}
	for _, e := range events {
		_ = acc.handle(e)
	}
	return acc.result()
}

// streamAccumulator folds inference events into a ChatResult, emitting text
// deltas to onChunk as content grows.
type streamAccumulator struct {
	onChunk      func(string)
	lastContent  string
	streamedLen  int // runes already emitted to onChunk
	title        string
	model        string
	inputTokens  *int
	outputTokens *int
	cachedTokens *int
}

func (a *streamAccumulator) handle(e NdjsonEvent) error {
	if e.IsAgentInference() {
		inf, ok := e.AgentInference()
		if !ok {
			return nil
		}
		content := ""
		for _, v := range inf.Value {
			if v.Type == "text" {
				content = v.Content
				break
			}
		}

		if a.onChunk != nil && !IsIncompleteLangTag(content) {
			display := []rune(StripLangTag(content))
			if len(display) > a.streamedLen {
				a.onChunk(string(display[a.streamedLen:]))
				a.streamedLen = len(display)
			}
		}

		a.lastContent = content
		if inf.FinishedAt != nil && *inf.FinishedAt != 0 {
			a.model = inf.Model
			a.inputTokens = inf.InputTokens
			a.outputTokens = inf.OutputTokens
			a.cachedTokens = inf.CachedTokensRead
		}
		return nil
	}
	if title, ok := e.Title(); ok {
		a.title = title
	}
	return nil
}

func (a *streamAccumulator) result() ChatResult {
	r := ChatResult{
		Response: StripLangTag(a.lastContent),
		Title:    a.title,
		Model:    a.model,
	}
	if a.inputTokens != nil {
		r.Tokens = &ChatTokens{Input: *a.inputTokens, Output: a.outputTokens, Cached: a.cachedTokens}
	}
	return r
}

// --- Patch stream normalization (thread replies) ---

// NormalizePatchStream converts a patch-format event slice (used for thread
// replies) into standard NdjsonEvents, in order.
func NormalizePatchStream(events []NdjsonEvent) []NdjsonEvent {
	var out []NdjsonEvent
	pn := &patchNormalizer{emit: func(e NdjsonEvent) error {
		out = append(out, e)
		return nil
	}}
	for _, e := range events {
		_ = pn.handle(e)
	}
	return out
}

// patchNormalizer tracks patch-start slot state and emits synthetic
// agent-inference events as patches apply.
type patchNormalizer struct {
	slots []map[string]any
	emit  func(NdjsonEvent) error
}

func (p *patchNormalizer) handle(e NdjsonEvent) error {
	switch e.Type {
	case "patch-start":
		var ps struct {
			Data struct {
				S []map[string]any `json:"s"`
			} `json:"data"`
		}
		_ = json.Unmarshal(e.Raw, &ps)
		p.slots = ps.Data.S
		if p.slots == nil {
			p.slots = []map[string]any{}
		}
		return nil

	case "patch":
		var pe struct {
			V []PatchOp `json:"v"`
		}
		if json.Unmarshal(e.Raw, &pe) != nil {
			return nil
		}
		for _, op := range pe.V {
			applyPatchOp(&p.slots, op.O, op.P, op.V)
		}
		for _, slot := range p.slots {
			if slot["type"] == "agent-inference" {
				raw, err := json.Marshal(slot)
				if err != nil {
					break
				}
				return p.emit(NdjsonEvent{Type: "agent-inference", Raw: raw})
			}
		}
		return nil

	default:
		return p.emit(e)
	}
}

// applyPatchOp applies one JSON-pointer patch op to the slots array in place.
// Supports "a" (add/append), "x" (string append), and "r" (replace/remove).
func applyPatchOp(slots *[]map[string]any, op, path string, value any) {
	parts := pathParts(path)
	if len(parts) < 2 || parts[0] != "s" {
		return
	}
	slotKey := parts[1]

	if slotKey == "-" && op == "a" {
		m, _ := value.(map[string]any)
		*slots = append(*slots, m)
		return
	}

	idx, err := strconv.Atoi(slotKey)
	if err != nil || idx < 0 || idx >= len(*slots) {
		return
	}
	s := *slots

	if len(parts) == 2 {
		if op == "r" {
			if m, ok := value.(map[string]any); ok {
				s[idx] = m
			}
		}
		return
	}

	fieldParts := parts[2:]
	var parent any = s
	parentKey := slotKey
	var target any = s[idx]
	for i := 0; i < len(fieldParts)-1; i++ {
		key := fieldParts[i]
		parent = target
		parentKey = key
		target = navigate(target, key)
		if target == nil {
			return
		}
	}
	applyLeaf(parent, parentKey, target, fieldParts[len(fieldParts)-1], op, value)
}

// navigate steps into a map key or array index, "" (nil) on any miss.
func navigate(target any, key string) any {
	switch t := target.(type) {
	case map[string]any:
		return t[key]
	case []any:
		if key == "-" {
			return nil
		}
		i, err := strconv.Atoi(key)
		if err != nil || i < 0 || i >= len(t) {
			return nil
		}
		return t[i]
	default:
		return nil
	}
}

// applyLeaf performs the op at the leaf. For array length changes (append,
// splice) the mutated slice is written back through parent[parentKey].
func applyLeaf(parent any, parentKey string, target any, lastKey, op string, value any) {
	switch t := target.(type) {
	case []any:
		if lastKey == "-" {
			if op == "a" {
				setChild(parent, parentKey, append(t, value))
			}
			return
		}
		i, err := strconv.Atoi(lastKey)
		if err != nil || i < 0 {
			return
		}
		switch op {
		case "a":
			switch {
			case i < len(t):
				t[i] = value
			case i == len(t):
				setChild(parent, parentKey, append(t, value))
			}
		case "r":
			if i < len(t) {
				setChild(parent, parentKey, append(t[:i], t[i+1:]...))
			}
		case "x":
			if i < len(t) {
				if s, ok := t[i].(string); ok {
					if add, ok := value.(string); ok {
						t[i] = s + add
					}
				}
			}
		}
	case map[string]any:
		switch op {
		case "a":
			t[lastKey] = value
		case "r":
			delete(t, lastKey)
		case "x":
			if s, ok := t[lastKey].(string); ok {
				if add, ok := value.(string); ok {
					t[lastKey] = s + add
				}
			}
		}
	}
}

// setChild writes val back into a map key or array index.
func setChild(parent any, key string, val []any) {
	switch p := parent.(type) {
	case map[string]any:
		p[key] = val
	case []any:
		if i, err := strconv.Atoi(key); err == nil && i >= 0 && i < len(p) {
			p[i] = val
		}
	}
}

// pathParts splits a JSON pointer into non-empty segments.
func pathParts(path string) []string {
	raw := strings.Split(path, "/")
	parts := make([]string, 0, len(raw))
	for _, p := range raw {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}
