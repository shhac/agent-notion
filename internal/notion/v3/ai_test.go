package v3

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ev decodes a JSON line into an NdjsonEvent for tests.
func ev(t *testing.T, jsonStr string) NdjsonEvent {
	t.Helper()
	e, ok := decodeEvent(json.RawMessage(jsonStr))
	if !ok {
		t.Fatalf("decodeEvent failed: %s", jsonStr)
	}
	return e
}

func infContent(t *testing.T, e NdjsonEvent) string {
	t.Helper()
	inf, ok := e.AgentInference()
	if !ok {
		t.Fatalf("not an agent-inference event: %s", e.Raw)
	}
	for _, v := range inf.Value {
		if v.Type == "text" {
			return v.Content
		}
	}
	return ""
}

// =============================================================================
// StripLangTag / IsIncompleteLangTag
// =============================================================================

func TestStripLangTag(t *testing.T) {
	cases := map[string]string{
		`<lang primary="en-US"/> Hello`:              "Hello",
		`<lang primary="en-US" secondary="fr"/>text`: "text",
		"Hello world": "Hello world",
		"":            "",
	}
	for in, want := range cases {
		if got := StripLangTag(in); got != want {
			t.Errorf("StripLangTag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsIncompleteLangTag(t *testing.T) {
	if !IsIncompleteLangTag("<lang primary") {
		t.Error("expected incomplete tag to be true")
	}
	if IsIncompleteLangTag(`<lang primary="en-US"/>`) {
		t.Error("complete tag should be false")
	}
	if IsIncompleteLangTag("Hello world") {
		t.Error("non-tag should be false")
	}
}

// =============================================================================
// ExtractRichText
// =============================================================================

func TestExtractRichText(t *testing.T) {
	single := []any{[]any{"hello"}}
	if got, ok := ExtractRichText(single); !ok || got != "hello" {
		t.Errorf("single = %q ok=%v", got, ok)
	}
	multi := []any{[]any{"hello"}, []any{"world"}}
	if got, ok := ExtractRichText(multi); !ok || got != "helloworld" {
		t.Errorf("multi = %q ok=%v", got, ok)
	}
	if got, ok := ExtractRichText("direct text"); !ok || got != "direct text" {
		t.Errorf("string = %q ok=%v", got, ok)
	}
	if _, ok := ExtractRichText(nil); ok {
		t.Error("nil should be not-ok")
	}
	if _, ok := ExtractRichText([]any{}); ok {
		t.Error("empty array should be not-ok")
	}
}

// =============================================================================
// ParseThreadMessage
// =============================================================================

func ptrInt64(v int64) *int64 { return &v }

func TestParseThreadMessage(t *testing.T) {
	t.Run("user", func(t *testing.T) {
		msg, ok := ParseThreadMessage("msg-1", "user",
			map[string]any{"type": "user", "value": []any{[]any{"Hello"}}},
			map[string]any{"created_time": float64(1700000000000)})
		if !ok {
			t.Fatal("not ok")
		}
		eq(t, msg, ThreadMessage{ID: "msg-1", Role: "user", Content: "Hello", CreatedAt: ptrInt64(1700000000000)})
	})

	t.Run("agent-inference strips lang tag", func(t *testing.T) {
		msg, ok := ParseThreadMessage("msg-2", "agent-inference",
			map[string]any{"type": "agent-inference", "value": []any{map[string]any{"type": "text", "content": `<lang primary="en-US"/>response text`}}},
			map[string]any{"created_time": float64(1700000000000)})
		if !ok {
			t.Fatal("not ok")
		}
		eq(t, msg, ThreadMessage{ID: "msg-2", Role: "assistant", Content: "response text", CreatedAt: ptrInt64(1700000000000)})
	})

	t.Run("agent-tool-result success", func(t *testing.T) {
		msg, ok := ParseThreadMessage("msg-3", "agent-tool-result",
			map[string]any{"type": "agent-tool-result", "toolName": "view", "state": "applied"},
			map[string]any{"created_time": float64(1700000000000)})
		if !ok {
			t.Fatal("not ok")
		}
		eq(t, msg, ThreadMessage{ID: "msg-3", Role: "tool", Content: `Tool "view" completed`, CreatedAt: ptrInt64(1700000000000), ToolName: "view", ToolState: "applied"})
	})

	t.Run("agent-tool-result error", func(t *testing.T) {
		msg, ok := ParseThreadMessage("msg-4", "agent-tool-result",
			map[string]any{"type": "agent-tool-result", "toolName": "view", "state": "applied:error", "error": "Not found"},
			map[string]any{"created_time": float64(1700000000000)})
		if !ok {
			t.Fatal("not ok")
		}
		eq(t, msg, ThreadMessage{ID: "msg-4", Role: "tool", Content: `Tool "view" failed: Not found`, CreatedAt: ptrInt64(1700000000000), ToolName: "view", ToolState: "applied:error"})
	})

	t.Run("skipped step types", func(t *testing.T) {
		for _, typ := range []string{"config", "context", "title", "something-else"} {
			if _, ok := ParseThreadMessage("x", typ, map[string]any{"type": typ}, map[string]any{}); ok {
				t.Errorf("%s should be skipped", typ)
			}
		}
	})
}

// =============================================================================
// ResolveModel
// =============================================================================

func TestResolveModel(t *testing.T) {
	models := []AIModel{
		{Model: "oatmeal-cookie", ModelMessage: "GPT-5.2", ModelFamily: "openai", DisplayGroup: "intelligent"},
		{Model: "cinnamon-roll", ModelMessage: "Claude 4 Sonnet", ModelFamily: "anthropic", DisplayGroup: "fast"},
	}
	check := func(flag, def, want string) {
		got, err := ResolveModel(models, flag, def)
		if err != nil {
			t.Fatalf("ResolveModel(%q,%q) err = %v", flag, def, err)
		}
		if got != want {
			t.Errorf("ResolveModel(%q,%q) = %q, want %q", flag, def, got, want)
		}
	}
	check("oatmeal-cookie", "", "oatmeal-cookie") // exact codename
	check("gpt-5.2", "", "oatmeal-cookie")        // case-insensitive display name
	check("sonnet", "", "cinnamon-roll")          // partial display name
	check("", "", "")                             // none provided
	check("", "cinnamon-roll", "cinnamon-roll")   // config default

	if _, err := ResolveModel(models, "nonexistent", ""); err == nil {
		t.Error("expected error for unknown model")
	}
}

// =============================================================================
// applyPatchOp
// =============================================================================

func TestApplyPatchOp(t *testing.T) {
	t.Run("a adds to object", func(t *testing.T) {
		slots := []map[string]any{{"type": "test"}}
		applyPatchOp(&slots, "a", "/s/0/key", "value")
		if slots[0]["key"] != "value" {
			t.Errorf("slots = %#v", slots)
		}
	})

	t.Run("a appends to array with dash", func(t *testing.T) {
		slots := []map[string]any{{"arr": []any{"a", "b"}}}
		applyPatchOp(&slots, "a", "/s/0/arr/-", "c")
		eq(t, slots[0]["arr"], []any{"a", "b", "c"})
	})

	t.Run("x appends to string", func(t *testing.T) {
		slots := []map[string]any{{"content": "Hello"}}
		applyPatchOp(&slots, "x", "/s/0/content", " World")
		if slots[0]["content"] != "Hello World" {
			t.Errorf("content = %#v", slots[0]["content"])
		}
	})

	t.Run("r removes from array", func(t *testing.T) {
		slots := []map[string]any{{"arr": []any{"a", "b", "c"}}}
		applyPatchOp(&slots, "r", "/s/0/arr/1", nil)
		eq(t, slots[0]["arr"], []any{"a", "c"})
	})

	t.Run("r deletes from object", func(t *testing.T) {
		slots := []map[string]any{{"key": "val", "other": float64(1)}}
		applyPatchOp(&slots, "r", "/s/0/key", nil)
		if _, ok := slots[0]["key"]; ok {
			t.Error("key should be deleted")
		}
		if slots[0]["other"] != float64(1) {
			t.Errorf("other = %#v", slots[0]["other"])
		}
	})

	t.Run("nested paths", func(t *testing.T) {
		slots := []map[string]any{{"value": []any{map[string]any{"content": "start"}}}}
		applyPatchOp(&slots, "x", "/s/0/value/0/content", " end")
		got := slots[0]["value"].([]any)[0].(map[string]any)["content"]
		if got != "start end" {
			t.Errorf("content = %#v", got)
		}
	})

	t.Run("no-op invalid slot index", func(t *testing.T) {
		slots := []map[string]any{{"type": "test"}}
		applyPatchOp(&slots, "a", "/s/5/key", "value")
		if len(slots) != 1 || slots[0]["type"] != "test" || slots[0]["key"] != nil {
			t.Errorf("slots = %#v", slots)
		}
	})

	t.Run("no-op path not /s", func(t *testing.T) {
		slots := []map[string]any{{"type": "test"}}
		applyPatchOp(&slots, "a", "/x/0/key", "value")
		if slots[0]["key"] != nil {
			t.Errorf("slots = %#v", slots)
		}
	})

	t.Run("a with /s/- appends new slot", func(t *testing.T) {
		slots := []map[string]any{}
		applyPatchOp(&slots, "a", "/s/-", map[string]any{"type": "agent-inference", "value": []any{}})
		if len(slots) != 1 || slots[0]["type"] != "agent-inference" {
			t.Errorf("slots = %#v", slots)
		}
	})
}

// =============================================================================
// NormalizePatchStream
// =============================================================================

func TestNormalizePatchStream(t *testing.T) {
	t.Run("passes through agent-inference", func(t *testing.T) {
		e := ev(t, `{"type":"agent-inference","id":"inf-1","value":[{"type":"text","content":"Hello"}],"traceId":"t1","startedAt":1,"previousAttemptValues":[]}`)
		out := NormalizePatchStream([]NdjsonEvent{e})
		if len(out) != 1 || out[0].Type != "agent-inference" || infContent(t, out[0]) != "Hello" {
			t.Errorf("out = %#v", out)
		}
	})

	t.Run("passes through record-map", func(t *testing.T) {
		e := ev(t, `{"type":"record-map"}`)
		out := NormalizePatchStream([]NdjsonEvent{e})
		if len(out) != 1 || out[0].Type != "record-map" {
			t.Errorf("out = %#v", out)
		}
	})

	t.Run("patch-start + patch via /s/-", func(t *testing.T) {
		events := []NdjsonEvent{
			ev(t, `{"type":"patch-start","data":{"s":[]},"version":1}`),
			ev(t, `{"type":"patch","v":[{"o":"a","p":"/s/-","v":{"type":"agent-inference","id":"inf-1","value":[{"type":"text","content":"Hi"}],"traceId":"t1","startedAt":1,"previousAttemptValues":[]}}]}`),
		}
		out := NormalizePatchStream(events)
		if len(out) != 1 || out[0].Type != "agent-inference" || infContent(t, out[0]) != "Hi" {
			t.Errorf("out = %#v", out)
		}
	})

	t.Run("text append o:x", func(t *testing.T) {
		events := []NdjsonEvent{
			ev(t, `{"type":"patch-start","data":{"s":[{"type":"agent-inference","id":"inf-1","value":[{"type":"text","content":"Hel"}],"traceId":"t1","startedAt":1,"previousAttemptValues":[]}]},"version":1}`),
			ev(t, `{"type":"patch","v":[{"o":"x","p":"/s/0/value/0/content","v":"lo"}]}`),
		}
		out := NormalizePatchStream(events)
		if len(out) != 1 || infContent(t, out[0]) != "Hello" {
			t.Errorf("out content = %q", infContent(t, out[0]))
		}
	})

	t.Run("remove o:r", func(t *testing.T) {
		events := []NdjsonEvent{
			ev(t, `{"type":"patch-start","data":{"s":[{"type":"agent-inference","id":"inf-1","value":[{"type":"text","content":"keep"},{"type":"thinking","content":"remove"}],"traceId":"t1","startedAt":1,"previousAttemptValues":[]}]},"version":1}`),
			ev(t, `{"type":"patch","v":[{"o":"r","p":"/s/0/value/1"}]}`),
		}
		out := NormalizePatchStream(events)
		if len(out) != 1 {
			t.Fatalf("out = %#v", out)
		}
		inf, _ := out[0].AgentInference()
		if len(inf.Value) != 1 || inf.Value[0].Content != "keep" {
			t.Errorf("value = %#v", inf.Value)
		}
	})
}

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
