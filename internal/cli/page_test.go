package cli

import (
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
)

// mockNotFound is the official-API 404 body used for parent-probe misses.
func mockNotFound() mocknotion.Response {
	return mocknotion.Response{Status: 404,
		Body: map[string]any{"object": "error", "status": 404, "code": "object_not_found"}}
}

// --- official-backend leaves ---

func TestPageGet(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/pages/pg1", map[string]any{"id": "pg1", "url": "u",
		"properties": map[string]any{"Name": map[string]any{"type": "title",
			"title": []any{map[string]any{"plain_text": "Doc"}}}}})

	out, _, err := runCLI(t, "", "--base-url", url, "page", "get", "pg1")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["id"] != "pg1" || item["properties"].(map[string]any)["Name"] != "Doc" {
		t.Errorf("get output = %v", item)
	}
	if _, hasContent := item["content"]; hasContent {
		t.Error("plain get should not include content")
	}
}

func TestPageGetWithContent(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/pages/pg1", map[string]any{"id": "pg1", "url": "u", "properties": map[string]any{}})
	s.HandleBody("GET /v1/blocks/pg1/children", map[string]any{
		"has_more": false, "next_cursor": nil,
		"results": []any{map[string]any{"id": "b1", "type": "heading_1", "has_children": false,
			"heading_1": map[string]any{"rich_text": []any{map[string]any{"plain_text": "Title"}}}}},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "page", "get", "pg1", "--content")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["content"] != "# Title" {
		t.Errorf("content = %v", item["content"])
	}
	if item["block_count"] != float64(1) {
		t.Errorf("block_count = %v", item["block_count"])
	}
}

func TestPageGetNormalizesDashlessID(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	const dashed = "11111111-1111-1111-1111-111111111111"
	s.HandleBody("GET /v1/pages/"+dashed, map[string]any{"id": dashed, "url": "u", "properties": map[string]any{}})

	if _, _, err := runCLI(t, "", "--base-url", url, "page", "get", "11111111111111111111111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls := s.CallsFor("GET /v1/pages/" + dashed); len(calls) != 1 {
		t.Errorf("expected the dashless ID to be dashed for the API; calls to dashed path = %d", len(calls))
	}
}

func TestPageCreate(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	// Parent probe 404s -> page parent.
	s.Handle("GET /v1/databases/parent1", mockNotFound())
	s.HandleBody("POST /v1/pages", map[string]any{"id": "new1", "url": "u",
		"parent": map[string]any{"type": "page_id", "page_id": "parent1"}, "created_time": "2024-01-01T00:00:00Z"})

	out, _, err := runCLI(t, "", "--base-url", url, "page", "create", "--parent", "parent1", "--title", "Child")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["id"] != "new1" || item["title"] != "Child" {
		t.Errorf("create output = %v", item)
	}
}

func TestPageUpdate(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("PATCH /v1/pages/pg1", map[string]any{"id": "pg1", "url": "u", "last_edited_time": "2024-02-02T00:00:00Z"})

	out, _, err := runCLI(t, "", "--base-url", url, "page", "update", "pg1", "--title", "New Title")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["id"] != "pg1" || item["last_edited_at"] != "2024-02-02T00:00:00Z" {
		t.Errorf("update output = %v", item)
	}
}

func TestPageUpdateRequiresAField(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "page", "update", "pg1")
	if err == nil || !strings.Contains(err.Error(), "Nothing to update") {
		t.Errorf("err = %v, want nothing-to-update", err)
	}
}

func TestPageTrashRequiresYes(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "page", "trash", "pg1")
	if err == nil || !strings.Contains(err.Error(), "Trash") {
		t.Errorf("err = %v, want confirm-gate refusal", err)
	}
}

func TestPageTrashWithYes(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("PATCH /v1/pages/pg1", map[string]any{"id": "pg1"})

	out, _, err := runCLI(t, "", "--base-url", url, "page", "trash", "pg1", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["id"] != "pg1" || item["trashed"] != true {
		t.Errorf("trash output = %v", item)
	}
}

func TestPageRestore(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("PATCH /v1/pages/pg1", map[string]any{"id": "pg1"})

	out, _, err := runCLI(t, "", "--base-url", url, "page", "restore", "pg1")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["id"] != "pg1" || item["trashed"] != false {
		t.Errorf("restore output = %v", item)
	}
}

// --- v3-only leaves ---

func TestPageArchiveRequiresYes(t *testing.T) {
	isolateState(t)
	seedV3Session(t)

	_, _, err := runCLI(t, "", "page", "archive", "pg1")
	if err == nil || !strings.Contains(err.Error(), "archives the page") {
		t.Errorf("err = %v, want confirm-gate refusal", err)
	}
}

func TestPageArchiveWithYesOnV3(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("saveTransactions", map[string]any{"recordMap": map[string]any{}})

	out, _, err := runCLI(t, "", "--base-url", url, "page", "archive", "pg1", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["archived"] != true {
		t.Errorf("archive output = %v", item)
	}
}

func TestPageUnarchiveOnV3(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("saveTransactions", map[string]any{"recordMap": map[string]any{}})

	out, _, err := runCLI(t, "", "--base-url", url, "page", "unarchive", "pg1")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["archived"] != false {
		t.Errorf("unarchive output = %v", item)
	}
}

func TestPageArchiveOnOfficialGuidance(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t) // official creds only, no v3 session

	_, _, err := runCLI(t, "", "--backend", "official", "page", "archive", "pg1", "--yes")
	if err == nil || !strings.Contains(err.Error(), "requires the v3 backend") {
		t.Errorf("err = %v, want v3-required guidance", err)
	}
}

func TestPageBacklinks(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("getBacklinksForBlock", map[string]any{
		"backlinks": []any{
			map[string]any{"block_id": "target", "mentioned_from": map[string]any{"block_id": "child-1", "table": "block"}},
		},
		"recordMap": map[string]any{
			"block": map[string]any{
				"child-1": map[string]any{"value": map[string]any{"id": "child-1", "parent_id": "page-1"}},
				"page-1": map[string]any{"value": map[string]any{"id": "page-1",
					"properties": map[string]any{"title": []any{[]any{"My Page"}}}}},
			},
		},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "page", "backlinks", "target")
	if err != nil {
		t.Fatal(err)
	}
	lines := decodeLines(t, out)
	var record, meta map[string]any
	for _, l := range lines {
		if _, ok := l["@total"]; ok {
			meta = l
		} else {
			record = l
		}
	}
	if record == nil || record["block_id"] != "child-1" || record["page_id"] != "page-1" || record["page_title"] != "My Page" {
		t.Errorf("backlink record = %v", record)
	}
	if meta == nil || meta["@total"] != float64(1) {
		t.Errorf("meta = %v", meta)
	}
}

func TestPageHistory(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("getSnapshotsList", map[string]any{
		"snapshots": []any{
			map[string]any{"id": "snap-1", "version": 10, "last_version": 8, "timestamp": 1700000000000,
				"authors": []any{map[string]any{"id": "user-1", "table": "notion_user"}}},
		},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "page", "history", "pg1", "--limit", "5")
	if err != nil {
		t.Fatal(err)
	}
	lines := decodeLines(t, out)
	var record map[string]any
	for _, l := range lines {
		if _, ok := l["@total"]; !ok {
			record = l
		}
	}
	if record == nil || record["id"] != "snap-1" || record["version"] != float64(10) || record["last_version"] != float64(8) {
		t.Fatalf("snapshot record = %v", record)
	}
	if ts, _ := record["timestamp"].(string); !strings.HasPrefix(ts, "2023-11-14T22:13:20") {
		t.Errorf("timestamp = %v", record["timestamp"])
	}
	authors, _ := record["authors"].([]any)
	if len(authors) != 1 || authors[0] != "user-1" {
		t.Errorf("authors = %v", record["authors"])
	}
}

func TestPageUsageCard(t *testing.T) {
	isolateState(t)
	out, _, err := runCLI(t, "", "page", "usage")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"page get", "page backlinks", "page history", "--raw-content"} {
		if !strings.Contains(out, want) {
			t.Errorf("page usage missing %q", want)
		}
	}
}
