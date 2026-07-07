package v3

import "testing"

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
