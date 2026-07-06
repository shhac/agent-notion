package truncation

import (
	"strings"
	"testing"
)

func obj(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	return m
}

func TestTruncatesLongTruncatableFields(t *testing.T) {
	tr := New(Options{})
	for _, field := range []string{"description", "body", "content"} {
		long := strings.Repeat("a", 300)
		got := obj(t, tr.Apply(map[string]any{field: long}))
		if got[field] != strings.Repeat("a", 200)+"…" {
			t.Errorf("%s not truncated: %q", field, got[field])
		}
		if got[field+"Length"] != 300 {
			t.Errorf("%sLength = %v, want 300", field, got[field+"Length"])
		}
	}
}

func TestShortAndExactFieldsKeptWithCompanion(t *testing.T) {
	tr := New(Options{})

	got := obj(t, tr.Apply(map[string]any{"description": "short text"}))
	if got["description"] != "short text" || got["descriptionLength"] != 10 {
		t.Errorf("short field mangled: %v", got)
	}

	exact := strings.Repeat("x", 200)
	got = obj(t, tr.Apply(map[string]any{"description": exact}))
	if got["description"] != exact || got["descriptionLength"] != 200 {
		t.Errorf("exact-length field mangled: %v", got)
	}
}

func TestNonTruncatableFieldsGetNoCompanion(t *testing.T) {
	got := obj(t, New(Options{}).Apply(map[string]any{"title": "hello", "name": "world"}))
	if got["title"] != "hello" || got["name"] != "world" {
		t.Errorf("fields changed: %v", got)
	}
	if _, ok := got["titleLength"]; ok {
		t.Error("unexpected titleLength companion")
	}
}

func TestNestedObjectsAndArrays(t *testing.T) {
	tr := New(Options{})

	got := obj(t, tr.Apply(map[string]any{
		"page": map[string]any{"description": strings.Repeat("d", 300), "title": "Test"},
	}))
	page := obj(t, got["page"])
	if page["description"] != strings.Repeat("d", 200)+"…" || page["descriptionLength"] != 300 {
		t.Errorf("nested truncation wrong: %v", page)
	}
	if page["title"] != "Test" {
		t.Errorf("nested title changed: %v", page["title"])
	}

	arr, ok := tr.Apply([]any{
		map[string]any{"id": "1", "body": strings.Repeat("b", 300)},
		map[string]any{"id": "2", "body": "short"},
	}).([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("array shape lost: %v", arr)
	}
	first, second := obj(t, arr[0]), obj(t, arr[1])
	if first["body"] != strings.Repeat("b", 200)+"…" || first["bodyLength"] != 300 {
		t.Errorf("array[0] = %v", first)
	}
	if second["body"] != "short" || second["bodyLength"] != 5 {
		t.Errorf("array[1] = %v", second)
	}
}

func TestNilAndPrimitivePassthrough(t *testing.T) {
	tr := New(Options{})
	if got := tr.Apply(nil); got != nil {
		t.Errorf("nil → %v", got)
	}
	if got := tr.Apply("hello"); got != "hello" {
		t.Errorf("string → %v", got)
	}
	if got := tr.Apply(42); got != 42 {
		t.Errorf("int → %v", got)
	}

	got := obj(t, tr.Apply(map[string]any{"description": nil, "title": "ok"}))
	if got["description"] != nil {
		t.Errorf("null description → %v", got["description"])
	}
	if _, ok := got["descriptionLength"]; ok {
		t.Error("companion added for null value")
	}

	got = obj(t, tr.Apply(map[string]any{"content": 42}))
	if got["content"] != 42 {
		t.Errorf("non-string content → %v", got["content"])
	}
	if _, ok := got["contentLength"]; ok {
		t.Error("companion added for non-string value")
	}
}

func TestFullExpandsEverything(t *testing.T) {
	long := strings.Repeat("a", 300)
	got := obj(t, New(Options{Full: true}).Apply(map[string]any{"description": long, "body": long}))
	if got["description"] != long || got["body"] != long {
		t.Errorf("--full still truncated: %v", got)
	}
	if got["descriptionLength"] != 300 || got["bodyLength"] != 300 {
		t.Errorf("companions missing under --full: %v", got)
	}
}

func TestExpandSelectedFields(t *testing.T) {
	long := strings.Repeat("a", 300)

	got := obj(t, New(Options{Expand: "description"}).Apply(map[string]any{"description": long, "body": long}))
	if got["description"] != long {
		t.Error("expanded field truncated")
	}
	if got["body"] == long {
		t.Error("unexpanded field not truncated")
	}

	got = obj(t, New(Options{Expand: " description , body "}).Apply(map[string]any{
		"description": long, "body": long, "content": long,
	}))
	if got["description"] != long || got["body"] != long {
		t.Error("whitespace in expand list not handled")
	}
	if got["content"] == long {
		t.Error("content should still truncate")
	}

	// Full wins over Expand.
	got = obj(t, New(Options{Full: true, Expand: "description"}).Apply(map[string]any{"body": long}))
	if got["body"] != long {
		t.Error("--full should win over --expand")
	}
}

func TestCustomMaxLength(t *testing.T) {
	got := obj(t, New(Options{MaxLength: 50}).Apply(map[string]any{"description": strings.Repeat("a", 100)}))
	if got["description"] != strings.Repeat("a", 50)+"…" || got["descriptionLength"] != 100 {
		t.Errorf("custom max length: %v", got)
	}
}

func TestRuneAwareTruncation(t *testing.T) {
	got := obj(t, New(Options{MaxLength: 3}).Apply(map[string]any{"description": "héllö wörld"}))
	if got["description"] != "hél…" {
		t.Errorf("rune truncation = %q", got["description"])
	}
	if got["descriptionLength"] != 11 {
		t.Errorf("rune length = %v, want 11", got["descriptionLength"])
	}
}
