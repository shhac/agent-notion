package official

import (
	"reflect"
	"testing"

	"github.com/shhac/agent-notion/internal/notion"
)

// richText builds a rich_text array as it decodes from JSON (map[string]any
// with a plain_text field).
func richText(parts ...string) []any {
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = map[string]any{"plain_text": p}
	}
	return out
}

func TestNormalizeBlock(t *testing.T) {
	checkedTrue := true
	checkedFalse := false

	tests := []struct {
		name  string
		block map[string]any
		want  notion.NormalizedBlock
	}{
		{
			"paragraph",
			map[string]any{"id": "b1", "type": "paragraph", "has_children": false,
				"paragraph": map[string]any{"rich_text": richText("hello ", "world")}},
			notion.NormalizedBlock{ID: "b1", Type: "paragraph", RichText: "hello world"},
		},
		{
			"to_do checked",
			map[string]any{"id": "b2", "type": "to_do", "has_children": false,
				"to_do": map[string]any{"rich_text": richText("task"), "checked": true}},
			notion.NormalizedBlock{ID: "b2", Type: "to_do", RichText: "task", Checked: &checkedTrue},
		},
		{
			"to_do defaults unchecked",
			map[string]any{"id": "b3", "type": "to_do", "has_children": false,
				"to_do": map[string]any{"rich_text": richText("task")}},
			notion.NormalizedBlock{ID: "b3", Type: "to_do", RichText: "task", Checked: &checkedFalse},
		},
		{
			"code with language",
			map[string]any{"id": "b4", "type": "code", "has_children": false,
				"code": map[string]any{"rich_text": richText("x := 1"), "language": "go"}},
			notion.NormalizedBlock{ID: "b4", Type: "code", RichText: "x := 1", Language: "go"},
		},
		{
			"image file url and caption",
			map[string]any{"id": "b5", "type": "image", "has_children": false,
				"image": map[string]any{"file": map[string]any{"url": "https://example.test/i.png"}, "caption": richText("a pic")}},
			notion.NormalizedBlock{ID: "b5", Type: "image", URL: "https://example.test/i.png", Caption: "a pic"},
		},
		{
			"image external url",
			map[string]any{"id": "b6", "type": "image", "has_children": false,
				"image": map[string]any{"external": map[string]any{"url": "https://example.test/e.png"}}},
			notion.NormalizedBlock{ID: "b6", Type: "image", URL: "https://example.test/e.png"},
		},
		{
			"bookmark",
			map[string]any{"id": "b7", "type": "bookmark", "has_children": false,
				"bookmark": map[string]any{"url": "https://example.test", "caption": richText("site")}},
			notion.NormalizedBlock{ID: "b7", Type: "bookmark", URL: "https://example.test", Caption: "site"},
		},
		{
			"equation",
			map[string]any{"id": "b8", "type": "equation", "has_children": false,
				"equation": map[string]any{"expression": "a^2"}},
			notion.NormalizedBlock{ID: "b8", Type: "equation", Expression: "a^2"},
		},
		{
			"child_page",
			map[string]any{"id": "b9", "type": "child_page", "has_children": true,
				"child_page": map[string]any{"title": "Notes"}},
			notion.NormalizedBlock{ID: "b9", Type: "child_page", Title: "Notes", HasChildren: true},
		},
		{
			"callout emoji",
			map[string]any{"id": "b10", "type": "callout", "has_children": false,
				"callout": map[string]any{"rich_text": richText("note"), "icon": map[string]any{"emoji": "💡"}}},
			notion.NormalizedBlock{ID: "b10", Type: "callout", RichText: "note", Emoji: "💡"},
		},
		{
			"embed",
			map[string]any{"id": "b11", "type": "embed", "has_children": false,
				"embed": map[string]any{"url": "https://example.test/embed"}},
			notion.NormalizedBlock{ID: "b11", Type: "embed", URL: "https://example.test/embed"},
		},
		{
			"file with name",
			map[string]any{"id": "b12", "type": "file", "has_children": false,
				"file": map[string]any{"file": map[string]any{"url": "https://example.test/f.pdf"}, "name": "report", "caption": richText("cap")}},
			notion.NormalizedBlock{ID: "b12", Type: "file", URL: "https://example.test/f.pdf", Caption: "cap", Title: "report"},
		},
		{
			"table",
			map[string]any{"id": "b13", "type": "table", "has_children": true,
				"table": map[string]any{"table_width": float64(2), "has_column_header": true, "has_row_header": false}},
			notion.NormalizedBlock{ID: "b13", Type: "table", TableWidth: 2, HasColumnHeader: true, HasChildren: true},
		},
		{
			"table_row cells",
			map[string]any{"id": "b14", "type": "table_row", "has_children": false,
				"table_row": map[string]any{"cells": []any{richText("Name"), richText("State")}}},
			notion.NormalizedBlock{ID: "b14", Type: "table_row", Cells: []string{"Name", "State"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeBlock(tt.block)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normalizeBlock()\n got  = %#v\n want = %#v", got, tt.want)
			}
		})
	}
}

func TestFlattenPropertyValue(t *testing.T) {
	tests := []struct {
		name string
		prop map[string]any
		want any
	}{
		{"title", map[string]any{"type": "title", "title": richText("My ", "Title")}, "My Title"},
		{"rich_text", map[string]any{"type": "rich_text", "rich_text": richText("body")}, "body"},
		{"number present", map[string]any{"type": "number", "number": float64(42)}, float64(42)},
		{"number null", map[string]any{"type": "number", "number": nil}, nil},
		{"select present", map[string]any{"type": "select", "select": map[string]any{"name": "High"}}, "High"},
		{"select null", map[string]any{"type": "select", "select": nil}, nil},
		{"multi_select", map[string]any{"type": "multi_select", "multi_select": []any{
			map[string]any{"name": "a"}, map[string]any{"name": "b"}}}, []string{"a", "b"}},
		{"multi_select empty", map[string]any{"type": "multi_select", "multi_select": []any{}}, []string{}},
		{"status", map[string]any{"type": "status", "status": map[string]any{"name": "Done"}}, "Done"},
		{"checkbox", map[string]any{"type": "checkbox", "checkbox": true}, true},
		{"url", map[string]any{"type": "url", "url": "https://example.test"}, "https://example.test"},
		{"url null", map[string]any{"type": "url", "url": nil}, nil},
		{"email", map[string]any{"type": "email", "email": "a@example.test"}, "a@example.test"},
		{"phone", map[string]any{"type": "phone_number", "phone_number": "555-0100"}, "555-0100"},
		{"created_time", map[string]any{"type": "created_time", "created_time": "2024-01-01T00:00:00Z"}, "2024-01-01T00:00:00Z"},
		{"verification", map[string]any{"type": "verification", "verification": map[string]any{"state": "verified"}}, "verified"},
		{"unique_id with prefix", map[string]any{"type": "unique_id", "unique_id": map[string]any{"prefix": "TASK", "number": float64(42)}}, "TASK-42"},
		{"unique_id no prefix", map[string]any{"type": "unique_id", "unique_id": map[string]any{"number": float64(7)}}, "7"},
		{"formula number", map[string]any{"type": "formula", "formula": map[string]any{"type": "number", "number": float64(3)}}, float64(3)},
		{"unknown type", map[string]any{"type": "made_up"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flattenPropertyValue(tt.prop)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("flattenPropertyValue() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestFlattenPropertyValueDate(t *testing.T) {
	prop := map[string]any{"type": "date", "date": map[string]any{"start": "2024-01-01", "end": nil}}
	got := flattenPropertyValue(prop)
	want := map[string]any{"start": "2024-01-01", "end": nil}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("date = %#v, want %#v", got, want)
	}

	if flattenPropertyValue(map[string]any{"type": "date", "date": nil}) != nil {
		t.Error("null date should flatten to nil")
	}
}

func TestFlattenPropertyValuePeople(t *testing.T) {
	prop := map[string]any{"type": "people", "people": []any{
		map[string]any{"id": "u1", "name": "Alice"},
		map[string]any{"id": "u2"}, // no name -> name key omitted
	}}
	got := flattenPropertyValue(prop)
	want := []any{
		map[string]any{"id": "u1", "name": "Alice"},
		map[string]any{"id": "u2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("people = %#v, want %#v", got, want)
	}
}

func TestFlattenPropertyValueRelationAndFiles(t *testing.T) {
	rel := map[string]any{"type": "relation", "relation": []any{
		map[string]any{"id": "p1"}, map[string]any{"id": "p2"}}}
	if got, want := flattenPropertyValue(rel), []any{
		map[string]any{"id": "p1"}, map[string]any{"id": "p2"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("relation = %#v, want %#v", got, want)
	}

	files := map[string]any{"type": "files", "files": []any{
		map[string]any{"name": "hosted", "file": map[string]any{"url": "https://example.test/h"}},
		map[string]any{"name": "linked", "external": map[string]any{"url": "https://example.test/e"}},
	}}
	if got, want := flattenPropertyValue(files), []any{
		map[string]any{"name": "hosted", "url": "https://example.test/h"},
		map[string]any{"name": "linked", "url": "https://example.test/e"},
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("files = %#v, want %#v", got, want)
	}
}

func TestFlattenPropertyValueRollupArray(t *testing.T) {
	prop := map[string]any{"type": "rollup", "rollup": map[string]any{"type": "array", "array": []any{
		map[string]any{"type": "number", "number": float64(1)},
		map[string]any{"type": "number", "number": float64(2)},
	}}}
	got := flattenPropertyValue(prop)
	want := []any{float64(1), float64(2)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rollup array = %#v, want %#v", got, want)
	}
}

func TestFormatParent(t *testing.T) {
	tests := []struct {
		name   string
		parent map[string]any
		want   *notion.ParentRef
	}{
		{"database", map[string]any{"type": "database_id", "database_id": "db1"}, &notion.ParentRef{Type: "database", ID: "db1"}},
		{"page", map[string]any{"type": "page_id", "page_id": "pg1"}, &notion.ParentRef{Type: "page", ID: "pg1"}},
		{"workspace", map[string]any{"type": "workspace", "workspace": true}, &notion.ParentRef{Type: "workspace"}},
		{"block", map[string]any{"type": "block_id", "block_id": "bl1"}, &notion.ParentRef{Type: "block", ID: "bl1"}},
		{"unknown", map[string]any{"type": "mystery"}, nil},
		{"nil", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatParent(tt.parent); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("formatParent() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestFormatIcon(t *testing.T) {
	if got := formatIcon(map[string]any{"type": "emoji", "emoji": "🚀"}); !reflect.DeepEqual(got, &notion.Icon{Type: "emoji", Emoji: "🚀"}) {
		t.Errorf("emoji icon = %#v", got)
	}
	if got := formatIcon(map[string]any{"type": "external", "external": map[string]any{"url": "https://example.test/i.png"}}); !reflect.DeepEqual(got, &notion.Icon{Type: "external", URL: "https://example.test/i.png"}) {
		t.Errorf("external icon = %#v", got)
	}
	if formatIcon(nil) != nil {
		t.Error("nil icon should be nil")
	}
}

func TestTransformSearchResult(t *testing.T) {
	page := map[string]any{
		"object":           "page",
		"id":               "pg1",
		"url":              "https://example.test/pg1",
		"last_edited_time": "2024-01-02T00:00:00Z",
		"parent":           map[string]any{"type": "database_id", "database_id": "db1"},
		"properties": map[string]any{
			"Name": map[string]any{"type": "title", "title": richText("Hello")},
		},
	}
	got := transformSearchResult(page)
	want := notion.SearchResult{
		ID: "pg1", Type: "page", Title: "Hello", URL: "https://example.test/pg1",
		Parent: &notion.ParentRef{Type: "database", ID: "db1"}, LastEditedAt: "2024-01-02T00:00:00Z",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("page result\n got  = %#v\n want = %#v", got, want)
	}

	db := map[string]any{
		"object": "database", "id": "db1", "url": "https://example.test/db1",
		"title": richText("My DB"),
	}
	gotDB := transformSearchResult(db)
	if gotDB.Type != "database" || gotDB.Title != "My DB" || gotDB.ID != "db1" {
		t.Errorf("database result = %#v", gotDB)
	}
}

func TestTransformDatabaseDetailAndSchema(t *testing.T) {
	db := map[string]any{
		"id": "db1", "url": "https://example.test/db1",
		"title":       richText("Tasks"),
		"description": richText("All tasks"),
		"is_inline":   true,
		"properties": map[string]any{
			"Status": map[string]any{
				"id": "s1", "type": "status",
				"status": map[string]any{
					"options": []any{
						map[string]any{"id": "o1", "name": "Todo", "color": "gray"},
						map[string]any{"id": "o2", "name": "Done", "color": "green"},
					},
					"groups": []any{
						map[string]any{"name": "Complete", "option_ids": []any{"o2"}},
					},
				},
			},
			"Priority": map[string]any{
				"id": "p1", "type": "select",
				"select": map[string]any{"options": []any{map[string]any{"id": "x", "name": "High", "color": "red"}}},
			},
		},
	}

	detail := transformDatabaseDetail(db)
	if detail.Title != "Tasks" || detail.Description != "All tasks" || !detail.IsInline {
		t.Errorf("detail top-level = %#v", detail)
	}
	status := detail.Properties["Status"]
	if len(status.Options) != 2 || status.Options[0].Name != "Todo" {
		t.Errorf("status options = %#v", status.Options)
	}
	if len(status.Groups) != 1 || status.Groups[0].Name != "Complete" ||
		len(status.Groups[0].Options) != 1 || status.Groups[0].Options[0] != "Done" {
		t.Errorf("status groups = %#v", status.Groups)
	}

	schema := transformDatabaseSchema(db)
	// sortedKeys => Priority before Status
	if len(schema.Properties) != 2 || schema.Properties[0].Name != "Priority" || schema.Properties[1].Name != "Status" {
		t.Fatalf("schema order = %#v", schema.Properties)
	}
	if got := schema.Properties[1].Groups["Complete"]; !reflect.DeepEqual(got, []string{"Done"}) {
		t.Errorf("schema status group = %#v", got)
	}
}

func TestTransformPageDetail(t *testing.T) {
	page := map[string]any{
		"id": "pg1", "url": "https://example.test/pg1",
		"parent":           map[string]any{"type": "page_id", "page_id": "parent1"},
		"icon":             map[string]any{"type": "emoji", "emoji": "📄"},
		"created_time":     "2024-01-01T00:00:00Z",
		"created_by":       map[string]any{"id": "u1", "name": "Alice"},
		"last_edited_time": "2024-01-02T00:00:00Z",
		"last_edited_by":   map[string]any{"id": "u2"},
		"archived":         false,
		"properties": map[string]any{
			"Name": map[string]any{"type": "title", "title": richText("Doc")},
			"Tags": map[string]any{"type": "multi_select", "multi_select": []any{map[string]any{"name": "x"}}},
		},
	}
	got := transformPageDetail(page)
	if got.ID != "pg1" || got.Parent.Type != "page" || got.Icon.Emoji != "📄" {
		t.Errorf("page detail = %#v", got)
	}
	if got.CreatedBy == nil || got.CreatedBy.Name != "Alice" || got.LastEditedBy == nil || got.LastEditedBy.ID != "u2" {
		t.Errorf("page users = %#v %#v", got.CreatedBy, got.LastEditedBy)
	}
	if got.Properties["Name"] != "Doc" {
		t.Errorf("flattened Name = %#v", got.Properties["Name"])
	}
	if tags, ok := got.Properties["Tags"].([]string); !ok || len(tags) != 1 || tags[0] != "x" {
		t.Errorf("flattened Tags = %#v", got.Properties["Tags"])
	}
}

func TestTransformCommentAndUser(t *testing.T) {
	comment := map[string]any{
		"id": "c1", "created_time": "2024-01-01T00:00:00Z",
		"rich_text":  richText("Nice ", "work"),
		"created_by": map[string]any{"id": "u1", "name": "Alice"},
	}
	gotC := transformComment(comment)
	if gotC.Body != "Nice work" || gotC.Author == nil || gotC.Author.Name != "Alice" {
		t.Errorf("comment = %#v", gotC)
	}

	user := map[string]any{
		"id": "u1", "name": "Alice", "type": "person",
		"person":     map[string]any{"email": "alice@example.test"},
		"avatar_url": "https://example.test/a.png",
	}
	gotU := transformUser(user)
	want := notion.UserItem{ID: "u1", Name: "Alice", Type: "person", Email: "alice@example.test", AvatarURL: "https://example.test/a.png"}
	if !reflect.DeepEqual(gotU, want) {
		t.Errorf("user = %#v, want %#v", gotU, want)
	}
}
