package v3

import (
	"encoding/json"
	"testing"
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
