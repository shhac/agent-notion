// Notion AI streaming pipeline — request building, the NDJSON inference stream,
// and event accumulation into a ChatResult. Patch normalization for thread
// replies lives in ai_patch.go.

package v3

import (
	"context"
	"encoding/json"
	"time"
)

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

// editToolFlags are the config flags that let the agent mutate a document; they
// are forced off in read-only mode so an ask/answer prompt cannot change content.
var editToolFlags = []string{
	"enableUpdatePageV2Tool",
	"enableUpdatePageAutofixer",
	"enableUpdatePageOrderUpdates",
	"enableUpdatePageMarkdownTree",
	"enableAgentSupportPropertyReorder",
	"enableAgentCreateDbTemplate",
}

// applyReadOnly forces the config into ask/answer mode: read-only on, every
// document-mutating tool off.
func applyReadOnly(configValue map[string]any) {
	configValue["useReadOnlyMode"] = true
	for _, k := range editToolFlags {
		configValue[k] = false
	}
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
	if params.ReadOnly {
		applyReadOnly(configValue)
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
