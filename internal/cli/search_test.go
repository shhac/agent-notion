package cli

import (
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
)

// seedSearchFixture queues one official-API search page with two hits and a
// next cursor.
func seedSearchFixture(t *testing.T) (serverURL string) {
	t.Helper()
	s, url := newMockServer(t)
	s.HandleBody("POST /v1/search", map[string]any{
		"object": "list",
		"results": []any{
			map[string]any{
				"object": "page", "id": "page-1",
				"url":              "https://www.notion.so/page-1",
				"last_edited_time": "2026-01-02T03:04:05.000Z",
				"parent":           map[string]any{"type": "workspace", "workspace": true},
				"properties": map[string]any{
					"title": map[string]any{
						"type": "title",
						"title": []any{
							map[string]any{"plain_text": "Project Roadmap"},
						},
					},
				},
			},
			map[string]any{
				"object": "database", "id": "db-1",
				"url":   "https://www.notion.so/db-1",
				"title": []any{map[string]any{"plain_text": "Task DB"}},
			},
		},
		"has_more":    true,
		"next_cursor": "cursor-2",
	})
	return url
}

func TestSearchQueryEndToEnd(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	url := seedSearchFixture(t)

	out, _, err := runCLI(t, "", "--base-url", url, "search", "query", "Project")
	if err != nil {
		t.Fatal(err)
	}
	lines := decodeLines(t, out)
	if len(lines) != 3 { // 2 hits + @pagination
		t.Fatalf("lines = %d:\n%s", len(lines), out)
	}
	if lines[0]["title"] != "Project Roadmap" || lines[0]["type"] != "page" {
		t.Errorf("hit 0 = %v", lines[0])
	}
	if lines[1]["type"] != "database" {
		t.Errorf("hit 1 = %v", lines[1])
	}
	pagination, ok := lines[2]["@pagination"].(map[string]any)
	if !ok || pagination["has_more"] != true || pagination["next_cursor"] != "cursor-2" {
		t.Errorf("pagination line = %v", lines[2])
	}
}

func TestSearchQueryFormatJSONEnvelope(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	url := seedSearchFixture(t)

	out, _, err := runCLI(t, "", "--base-url", url, "search", "query", "Project", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"data"`) || !strings.Contains(out, `"@pagination"`) {
		t.Errorf("json envelope missing keys:\n%s", out)
	}
}

func TestSearchQueryInvalidFilter(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "search", "query", "x", "--filter", "bogus")
	if err == nil || !strings.Contains(err.Error(), "invalid --filter") {
		t.Errorf("err = %v", err)
	}
}

func TestSearchQueryUnauthenticated(t *testing.T) {
	isolateState(t)

	_, _, err := runCLI(t, "", "search", "query", "x")
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("err = %v", err)
	}
}

func TestSearchQueryClassifiesAPIErrors(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.Handle("POST /v1/search", mocknotion.Response{
		Status: 429,
		Body: map[string]any{
			"object": "error", "status": 429, "code": "rate_limited", "message": "slow down",
		},
	})

	_, _, err := runCLI(t, "", "--base-url", url, "search", "query", "x")
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("err = %v", err)
	}
}
