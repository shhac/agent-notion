package cli

import (
	"strings"
	"testing"
)

func TestDatabaseGet(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/databases/db1", map[string]any{
		"id": "db1", "url": "u",
		"title":      []any{map[string]any{"plain_text": "Tasks"}},
		"properties": map[string]any{},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "database", "get", "db1")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["id"] != "db1" || item["title"] != "Tasks" {
		t.Errorf("get output = %v", item)
	}
}

func TestDatabaseSchema(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/databases/db1", map[string]any{
		"id": "db1", "url": "u",
		"title":      []any{map[string]any{"plain_text": "My DB"}},
		"properties": map[string]any{},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "database", "schema", "db1")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["id"] != "db1" || item["title"] != "My DB" {
		t.Errorf("schema output = %v", item)
	}
}

func TestDatabaseList(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("POST /v1/search", map[string]any{
		"object":      "list",
		"has_more":    false,
		"next_cursor": nil,
		"results": []any{map[string]any{
			"object": "database", "id": "db-1", "url": "https://www.notion.so/db-1",
			"title": []any{map[string]any{"plain_text": "Task DB"}},
		}},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "database", "list")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["title"] != "Task DB" {
		t.Errorf("list output = %v", item)
	}
}

func TestDatabaseQueryWithPagination(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("POST /v1/databases/db1/query", map[string]any{
		"has_more":    true,
		"next_cursor": "cur2",
		"results": []any{map[string]any{
			"id": "row1", "url": "u",
			"properties": map[string]any{"Count": map[string]any{"type": "number", "number": 5}},
		}},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "database", "query", "db1")
	if err != nil {
		t.Fatal(err)
	}
	lines := decodeLines(t, out)
	if len(lines) != 2 { // 1 row + @pagination
		t.Fatalf("lines = %d:\n%s", len(lines), out)
	}
	if props := lines[0]["properties"].(map[string]any); props["Count"] != float64(5) {
		t.Errorf("row = %v", lines[0])
	}
	pagination, ok := lines[1]["@pagination"].(map[string]any)
	if !ok || pagination["has_more"] != true || pagination["next_cursor"] != "cur2" {
		t.Errorf("pagination = %v", lines[1])
	}
}

func TestDatabaseQueryInvalidFilter(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "database", "query", "db1", "--filter", "not-json")
	if err == nil || !strings.Contains(err.Error(), "Invalid --filter JSON") {
		t.Errorf("err = %v", err)
	}
}
