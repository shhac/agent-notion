package v3

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// ProcessInferenceStream
// =============================================================================

func TestProcessInferenceStream(t *testing.T) {
	t.Run("extracts response text", func(t *testing.T) {
		events := []NdjsonEvent{ev(t, `{"type":"agent-inference","id":"i","value":[{"type":"text","content":"Hello world"}],"traceId":"t","startedAt":1,"previousAttemptValues":[]}`)}
		if got := ProcessInferenceStream(events, nil); got.Response != "Hello world" {
			t.Errorf("response = %q", got.Response)
		}
	})

	t.Run("extracts title", func(t *testing.T) {
		events := []NdjsonEvent{
			ev(t, `{"type":"title","value":"My Chat Title"}`),
			ev(t, `{"type":"agent-inference","id":"i","value":[{"type":"text","content":"response"}],"traceId":"t","startedAt":1,"previousAttemptValues":[]}`),
		}
		if got := ProcessInferenceStream(events, nil); got.Title != "My Chat Title" {
			t.Errorf("title = %q", got.Title)
		}
	})

	t.Run("extracts token counts from final event", func(t *testing.T) {
		events := []NdjsonEvent{ev(t, `{"type":"agent-inference","id":"i","value":[{"type":"text","content":"response"}],"traceId":"t","startedAt":1,"previousAttemptValues":[],"finishedAt":2,"inputTokens":100,"outputTokens":50,"cachedTokensRead":10,"model":"oatmeal-cookie"}`)}
		got := ProcessInferenceStream(events, nil)
		if got.Model != "oatmeal-cookie" || got.Tokens == nil {
			t.Fatalf("result = %#v", got)
		}
		if got.Tokens.Input != 100 || got.Tokens.Output == nil || *got.Tokens.Output != 50 || got.Tokens.Cached == nil || *got.Tokens.Cached != 10 {
			t.Errorf("tokens = %#v", got.Tokens)
		}
	})

	t.Run("strips lang tag from response", func(t *testing.T) {
		events := []NdjsonEvent{ev(t, `{"type":"agent-inference","id":"i","value":[{"type":"text","content":"<lang primary=\"en-US\"/>Hello"}],"traceId":"t","startedAt":1,"previousAttemptValues":[]}`)}
		if got := ProcessInferenceStream(events, nil); got.Response != "Hello" {
			t.Errorf("response = %q", got.Response)
		}
	})

	t.Run("calls onChunk for streaming", func(t *testing.T) {
		var chunks []string
		events := []NdjsonEvent{
			ev(t, `{"type":"agent-inference","id":"i","value":[{"type":"text","content":"Hello"}],"traceId":"t","startedAt":1,"previousAttemptValues":[]}`),
			ev(t, `{"type":"agent-inference","id":"i","value":[{"type":"text","content":"Hello world"}],"traceId":"t","startedAt":1,"previousAttemptValues":[]}`),
		}
		ProcessInferenceStream(events, func(c string) { chunks = append(chunks, c) })
		eq(t, chunks, []string{"Hello", " world"})
	})

	t.Run("empty stream", func(t *testing.T) {
		got := ProcessInferenceStream(nil, nil)
		if got.Response != "" || got.Title != "" || got.Tokens != nil {
			t.Errorf("result = %#v", got)
		}
	})
}

// =============================================================================
// buildInferenceRequest
// =============================================================================

func TestBuildInferenceRequest(t *testing.T) {
	params := RunInferenceParams{
		Message: "hi",
		Model:   "oatmeal-cookie",
		User:    AIUser{ID: "u1", Name: "Jane", Email: "jane@example.com"},
		Space:   AISpace{ID: "space-1", Name: "WS"},
	}
	seq := 0
	uuid := func() string { seq++; return "uuid-" + string(rune('0'+seq)) }

	t.Run("new thread", func(t *testing.T) {
		req, isNew := buildInferenceRequest(params, "UTC", timeFixture(), uuid)
		if !isNew {
			t.Fatal("expected new thread")
		}
		if !req.CreateThread || !req.GenerateTitle || req.AsPatchResponse || req.IsPartialTranscript {
			t.Errorf("flags = %+v", req)
		}
		if req.ThreadParentPointer == nil || req.ThreadParentPointer.ID != "space-1" {
			t.Errorf("threadParentPointer = %#v", req.ThreadParentPointer)
		}
		if len(req.Transcript) != 3 {
			t.Fatalf("transcript len = %d", len(req.Transcript))
		}
		configVal := req.Transcript[0].(map[string]any)["value"].(map[string]any)
		if configVal["model"] != "oatmeal-cookie" || configVal["modelFromUser"] != true {
			t.Errorf("config value = %#v", configVal)
		}
		// A flag from the spread block must be present.
		if configVal["useServerUndo"] != true {
			t.Errorf("missing default config flags: %#v", configVal["useServerUndo"])
		}
	})

	t.Run("reply thread", func(t *testing.T) {
		replyParams := params
		replyParams.ThreadID = "thread-9"
		req, isNew := buildInferenceRequest(replyParams, "UTC", timeFixture(), uuid)
		if isNew {
			t.Fatal("expected reply, not new")
		}
		if req.CreateThread || req.GenerateTitle || !req.AsPatchResponse || !req.IsPartialTranscript {
			t.Errorf("flags = %+v", req)
		}
		if req.ThreadParentPointer != nil {
			t.Errorf("reply should have no threadParentPointer")
		}
		if req.ThreadID != "thread-9" {
			t.Errorf("threadId = %q", req.ThreadID)
		}
	})

	t.Run("no search", func(t *testing.T) {
		p := params
		p.NoSearch = true
		req, _ := buildInferenceRequest(p, "UTC", timeFixture(), uuid)
		configVal := req.Transcript[0].(map[string]any)["value"].(map[string]any)
		if configVal["useWebSearch"] != false {
			t.Errorf("useWebSearch = %#v", configVal["useWebSearch"])
		}
		if scopes, ok := configVal["searchScopes"].([]any); !ok || len(scopes) != 0 {
			t.Errorf("searchScopes = %#v", configVal["searchScopes"])
		}
	})

	t.Run("read-only disables edit tools", func(t *testing.T) {
		p := params
		p.ReadOnly = true
		req, _ := buildInferenceRequest(p, "UTC", timeFixture(), uuid)
		configVal := req.Transcript[0].(map[string]any)["value"].(map[string]any)
		if configVal["useReadOnlyMode"] != true {
			t.Errorf("useReadOnlyMode = %#v, want true", configVal["useReadOnlyMode"])
		}
		for _, k := range editToolFlags {
			if configVal[k] != false {
				t.Errorf("read-only should disable %s, got %#v", k, configVal[k])
			}
		}
	})

	t.Run("edits allowed keeps update-page tool on", func(t *testing.T) {
		// Default (ReadOnly false) leaves the mutating tools enabled.
		req, _ := buildInferenceRequest(params, "UTC", timeFixture(), uuid)
		configVal := req.Transcript[0].(map[string]any)["value"].(map[string]any)
		if configVal["useReadOnlyMode"] != false || configVal["enableUpdatePageV2Tool"] != true {
			t.Errorf("non-read-only config = readOnly:%v updatePage:%v", configVal["useReadOnlyMode"], configVal["enableUpdatePageV2Tool"])
		}
	})
}

// timeFixture returns a fixed time for deterministic request building.
func timeFixture() time.Time { return time.UnixMilli(1700000000000).UTC() }

// ensure the request body marshals cleanly.
func TestInferenceRequestMarshals(t *testing.T) {
	req, _ := buildInferenceRequest(RunInferenceParams{Message: "hi", Space: AISpace{ID: "s"}}, "UTC", timeFixture(), func() string { return "u" })
	if _, err := json.Marshal(req); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw, _ := json.Marshal(req)
	if !strings.Contains(string(raw), `"threadType":"workflow"`) {
		t.Errorf("body missing threadType: %s", raw)
	}
}
