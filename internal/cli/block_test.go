package cli

import (
	"strings"
	"testing"
)

// --- list ---

func TestBlockListMarkdown(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/blocks/pg1/children", map[string]any{
		"has_more": false, "next_cursor": nil,
		"results": []any{map[string]any{"id": "b1", "type": "heading_1", "has_children": false,
			"heading_1": map[string]any{"rich_text": []any{map[string]any{"plain_text": "Title"}}}}},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "block", "list", "pg1")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["page_id"] != "pg1" || item["content"] != "# Title" || item["block_count"] != float64(1) {
		t.Errorf("list markdown output = %v", item)
	}
}

func TestBlockListRaw(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/blocks/pg1/children", map[string]any{
		"has_more": false, "next_cursor": nil,
		"results": []any{map[string]any{"id": "b1", "type": "paragraph", "has_children": false,
			"paragraph": map[string]any{"rich_text": []any{map[string]any{"plain_text": "hi"}}}}},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "block", "list", "pg1", "--raw")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["id"] != "b1" || item["type"] != "paragraph" || item["content"] != "hi" || item["has_children"] != false {
		t.Errorf("raw block record = %v", item)
	}
}

// --- append ---

func TestBlockAppendMarkdown(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("PATCH /v1/blocks/pg1/children", map[string]any{"results": []any{map[string]any{"id": "n1"}}})

	out, _, err := runCLI(t, "", "--base-url", url, "block", "append", "pg1", "--content", "# Hello")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["page_id"] != "pg1" || item["blocks_added"] != float64(1) {
		t.Errorf("append output = %v", item)
	}
}

func TestBlockAppendRequiresInput(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "block", "append", "pg1")
	if err == nil || !strings.Contains(err.Error(), "Provide --content") {
		t.Errorf("err = %v, want missing-input", err)
	}
}

func TestBlockAppendInvalidBlocksJSON(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "block", "append", "pg1", "--blocks", "{not an array}")
	if err == nil || !strings.Contains(err.Error(), "Invalid --blocks JSON") {
		t.Errorf("err = %v, want invalid-blocks", err)
	}
}

// --- update ---

func TestBlockUpdate(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/blocks/b1", map[string]any{"id": "b1", "type": "paragraph"})
	s.HandleBody("PATCH /v1/blocks/b1", map[string]any{"id": "b1", "last_edited_time": "2024-05-05T00:00:00Z"})

	out, _, err := runCLI(t, "", "--base-url", url, "block", "update", "b1", "--content", "new text")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["id"] != "b1" || item["last_edited_at"] != "2024-05-05T00:00:00Z" {
		t.Errorf("update output = %v", item)
	}
}

func TestBlockUpdateRequiresContent(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "block", "update", "b1")
	if err == nil || !strings.Contains(err.Error(), "Provide --content") {
		t.Errorf("err = %v, want missing-content", err)
	}
}

// --- delete ---

func TestBlockDeleteRequiresYes(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "block", "delete", "b1")
	if err == nil || !strings.Contains(err.Error(), "deletes the block") {
		t.Errorf("err = %v, want confirm-gate refusal", err)
	}
}

func TestBlockDeleteWithYes(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("DELETE /v1/blocks/b1", map[string]any{"id": "b1"})

	out, _, err := runCLI(t, "", "--base-url", url, "block", "delete", "b1", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["id"] != "b1" || item["deleted"] != true {
		t.Errorf("delete output = %v", item)
	}
}

func TestBlockDeleteNormalizesDashlessID(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	const dashed = "22222222-2222-2222-2222-222222222222"
	s.HandleBody("DELETE /v1/blocks/"+dashed, map[string]any{"id": dashed})

	if _, _, err := runCLI(t, "", "--base-url", url, "block", "delete", "22222222222222222222222222222222", "--yes"); err != nil {
		t.Fatal(err)
	}
	if calls := s.CallsFor("DELETE /v1/blocks/" + dashed); len(calls) != 1 {
		t.Errorf("dashless ID not dashed for the API; calls to dashed path = %d", len(calls))
	}
}

// --- move (v3-only) ---

func TestBlockMoveOnOfficialGuidance(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "--backend", "official", "block", "move", "b1", "--after", "b0")
	if err == nil || !strings.Contains(err.Error(), "requires the v3 backend") {
		t.Errorf("err = %v, want v3-required guidance", err)
	}
}

func TestBlockMoveOnV3(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("syncRecordValuesMain", map[string]any{
		"recordMap": map[string]any{"block": map[string]any{
			"b1": map[string]any{"value": map[string]any{"id": "b1", "parent_id": "parent-1", "parent_table": "block"}},
		}},
	})
	s.HandleBody("saveTransactions", map[string]any{"recordMap": map[string]any{}})

	out, _, err := runCLI(t, "", "--base-url", url, "block", "move", "b1", "--after", "b0")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["id"] != "b1" {
		t.Errorf("move output = %v", item)
	}
}

// --- replace ---

func TestBlockReplaceRequiresYes(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "block", "replace", "pg1", "--content", "# New")
	if err == nil || !strings.Contains(err.Error(), "deletes every existing block") {
		t.Errorf("err = %v, want confirm-gate refusal", err)
	}
}

func TestBlockReplaceWithYes(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/blocks/pg1/children", map[string]any{
		"has_more": false, "next_cursor": nil,
		"results": []any{map[string]any{"id": "b1", "type": "paragraph", "has_children": false, "paragraph": map[string]any{}}},
	})
	s.HandleBody("DELETE /v1/blocks/b1", map[string]any{"id": "b1"})
	s.HandleBody("PATCH /v1/blocks/pg1/children", map[string]any{"results": []any{map[string]any{"id": "n1"}}})

	out, _, err := runCLI(t, "", "--base-url", url, "block", "replace", "pg1", "--content", "# New", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["page_id"] != "pg1" || item["blocks_deleted"] != float64(1) || item["blocks_added"] != float64(1) {
		t.Errorf("replace output = %v", item)
	}
}

func TestBlockUsageCard(t *testing.T) {
	isolateState(t)
	out, _, err := runCLI(t, "", "block", "usage")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"block list", "block append", "block replace", "--raw"} {
		if !strings.Contains(out, want) {
			t.Errorf("block usage missing %q", want)
		}
	}
}
