package cli

import (
	"reflect"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
)

func TestActivityLog(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("getActivityLog", map[string]any{
		"activityIds": []any{"act-1"},
		"activities": map[string]any{"act-1": map[string]any{
			"id": "act-1", "version": 1, "type": "block-edited",
			"parent_id": "page-1", "parent_table": "block",
			"navigable_block_id": "page-1", "space_id": "space-1",
			"edits": []any{map[string]any{
				"type": "block-changed", "block_id": "page-1", "timestamp": 1700000000000,
				"authors": []any{map[string]any{"id": "user-1", "table": "notion_user"}},
			}},
			"start_time": 1700000000000, "end_time": 1700001000000,
		}},
		"recordMap": map[string]any{
			"block": mocknotion.Table(map[string]any{
				"page-1": mocknotion.BlockEntity("page-1", "page", map[string]any{
					"properties": map[string]any{"title": []any{[]any{"My Page"}}},
				}),
			}),
			"notion_user": mocknotion.Table(map[string]any{
				"user-1": map[string]any{"id": "user-1", "version": 1, "given_name": "Jane", "family_name": "Doe"},
			}),
		},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "activity", "log")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["id"] != "act-1" || item["type"] != "block-edited" || item["page_title"] != "My Page" {
		t.Errorf("activity entry = %v", item)
	}
	if !reflect.DeepEqual(item["authors"], []any{"Jane Doe"}) {
		t.Errorf("authors = %v", item["authors"])
	}
	if !reflect.DeepEqual(item["edit_types"], []any{"block-changed"}) {
		t.Errorf("edit_types = %v", item["edit_types"])
	}
}
