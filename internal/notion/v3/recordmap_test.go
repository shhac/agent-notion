package v3

import (
	"encoding/json"
	"fmt"
	"testing"
)

// blockJSON builds a minimal v3 block entity as JSON.
func blockJSON(id, blockType string, alive bool) string {
	return fmt.Sprintf(`{
		"id": %q, "type": %q, "version": 1,
		"created_time": 1700000000000, "last_edited_time": 1700000001000,
		"parent_id": "parent-1", "parent_table": "space",
		"alive": %v, "space_id": "space-1",
		"properties": {"title": [["Hello"]]}
	}`, id, blockType, alive)
}

func decodeRecordMap(t *testing.T, jsonStr string) RecordMap {
	t.Helper()
	var rm RecordMap
	if err := json.Unmarshal([]byte(jsonStr), &rm); err != nil {
		t.Fatalf("decode record map: %v", err)
	}
	return rm
}

func TestEntryUnwrapsNewSpaceIDWrappedFormat(t *testing.T) {
	rm := decodeRecordMap(t, `{
		"block": {
			"page-1": {"spaceId": "space-1", "value": {"value": `+blockJSON("page-1", "page", true)+`, "role": "reader"}}
		}
	}`)

	entry := rm["block"]["page-1"]
	if entry.Role != "reader" {
		t.Errorf("role = %q, want reader", entry.Role)
	}
	b, ok := rm.GetBlock("page-1")
	if !ok || b.ID != "page-1" || b.Type != "page" {
		t.Errorf("block = %+v ok=%v", b, ok)
	}
}

func TestEntryPassesThroughOldFormat(t *testing.T) {
	rm := decodeRecordMap(t, `{
		"block": {
			"page-1": {"value": `+blockJSON("page-1", "page", true)+`, "role": "reader"}
		}
	}`)

	entry := rm["block"]["page-1"]
	if entry.Role != "reader" {
		t.Errorf("role = %q", entry.Role)
	}
	if b, ok := rm.GetBlock("page-1"); !ok || b.ID != "page-1" {
		t.Errorf("block = %+v ok=%v", b, ok)
	}
}

func TestEntryUnwrapsNestedWithoutSpaceID(t *testing.T) {
	rm := decodeRecordMap(t, `{
		"block": {
			"child-1": {"value": {"value": `+blockJSON("child-1", "text", true)+`, "role": "reader"}}
		}
	}`)

	b, ok := rm.GetBlock("child-1")
	if !ok || b.Type != "text" {
		t.Errorf("block = %+v ok=%v", b, ok)
	}
	if rm["block"]["child-1"].Role != "reader" {
		t.Errorf("role = %q", rm["block"]["child-1"].Role)
	}
}

func TestMixedFormatsAcrossTables(t *testing.T) {
	rm := decodeRecordMap(t, `{
		"block": {
			"page-1": {"spaceId": "space-1", "value": {"value": `+blockJSON("page-1", "page", true)+`, "role": "reader"}},
			"child-1": {"value": `+blockJSON("child-1", "text", true)+`, "role": "editor"}
		},
		"collection": {
			"col-1": {"spaceId": "space-1", "value": {"value": {"id": "col-1", "version": 1, "name": [["Test"]], "schema": {}, "parent_id": "page-1", "parent_table": "block"}, "role": "reader"}}
		}
	}`)

	if b, ok := rm.GetBlock("page-1"); !ok || b.Type != "page" {
		t.Errorf("page-1 = %+v ok=%v", b, ok)
	}
	if b, ok := rm.GetBlock("child-1"); !ok || b.Type != "text" {
		t.Errorf("child-1 = %+v ok=%v", b, ok)
	}
	c, ok := rm.GetCollection("col-1")
	if !ok || c.ID != "col-1" || c.Name.Plain() != "Test" {
		t.Errorf("collection = %+v ok=%v", c, ok)
	}
}

func TestRecordMapSkipsVersionMetadata(t *testing.T) {
	rm := decodeRecordMap(t, `{
		"__version__": 3,
		"block": {
			"__version__": 3,
			"page-1": {"spaceId": "space-1", "value": {"value": `+blockJSON("page-1", "page", true)+`, "role": "reader"}}
		}
	}`)

	if _, ok := rm["__version__"]; ok {
		t.Error("__version__ leaked into the record map")
	}
	if _, ok := rm["block"]["__version__"]; ok {
		t.Error("__version__ leaked into the block table")
	}
	if b, ok := rm.GetBlock("page-1"); !ok || b.Type != "page" {
		t.Errorf("block = %+v ok=%v", b, ok)
	}
}

func TestEntryKeepsEntityWithPrimitiveValueField(t *testing.T) {
	// An old-format entity that itself has a primitive "value" field must not
	// be mistaken for a role wrapper.
	var entry Entry
	if err := json.Unmarshal([]byte(`{"value": {"id": "e1", "value": 42}, "role": "reader"}`), &entry); err != nil {
		t.Fatal(err)
	}
	var entity map[string]any
	if err := json.Unmarshal(entry.Value, &entity); err != nil {
		t.Fatal(err)
	}
	if entity["id"] != "e1" || entity["value"] != float64(42) {
		t.Errorf("entity = %v", entity)
	}
	if entry.Role != "reader" {
		t.Errorf("role = %q", entry.Role)
	}
}

func TestMerge(t *testing.T) {
	target := decodeRecordMap(t, `{
		"block": {"b1": {"value": `+blockJSON("b1", "text", true)+`, "role": "reader"}}
	}`)
	source := decodeRecordMap(t, `{
		"block": {
			"b1": {"value": `+blockJSON("b1", "page", true)+`, "role": "editor"},
			"b2": {"value": `+blockJSON("b2", "text", true)+`, "role": "reader"}
		},
		"notion_user": {"u1": {"value": {"id": "u1", "version": 1, "email": "u1@example.com", "given_name": "Test", "family_name": "User"}, "role": "reader"}}
	}`)

	target.Merge(source)

	if b, _ := target.GetBlock("b1"); b == nil || b.Type != "page" {
		t.Errorf("b1 not overwritten: %+v", b)
	}
	if b, _ := target.GetBlock("b2"); b == nil {
		t.Error("b2 not merged")
	}
	if u, ok := target.GetUser("u1"); !ok || u.ID != "u1" {
		t.Errorf("user table not created: %+v ok=%v", u, ok)
	}
}

func accessorFixture(t *testing.T) RecordMap {
	t.Helper()
	return decodeRecordMap(t, `{
		"block": {
			"b1": {"value": `+blockJSON("b1", "text", true)+`, "role": "reader"},
			"b2": {"value": `+blockJSON("b2", "text", false)+`, "role": "reader"},
			"b3": {"value": `+blockJSON("b3", "text", true)+`, "role": "reader"}
		},
		"collection": {
			"col-1": {"value": {"id": "col-1", "version": 1, "name": [["DB"]], "schema": {}, "parent_id": "p", "parent_table": "block"}, "role": "reader"}
		},
		"collection_view": {
			"view-1": {"value": {"id": "view-1", "version": 1, "type": "table", "parent_id": "p", "parent_table": "block", "alive": true}, "role": "reader"}
		},
		"notion_user": {
			"u1": {"value": {"id": "u1", "version": 1, "email": "u1@example.com", "given_name": "Test", "family_name": "User"}, "role": "reader"},
			"u2": {"value": {"id": "u2", "version": 1, "email": "u2@example.com", "given_name": "Jane", "family_name": "Doe"}, "role": "reader"}
		},
		"space": {
			"s1": {"value": {"id": "s1", "version": 1, "name": "Example Workspace"}, "role": "reader"}
		},
		"discussion": {
			"d1": {"value": {"id": "d1", "version": 1, "parent_id": "b1", "parent_table": "block", "resolved": false, "comments": ["c1"]}, "role": "reader"}
		},
		"comment": {
			"c1": {"value": {"id": "c1", "version": 1, "alive": true, "parent_id": "d1", "parent_table": "discussion", "text": [["A comment"]], "created_by_id": "u1", "created_by_table": "notion_user", "created_time": 1700000000000, "last_edited_time": 1700000000000}, "role": "reader"}
		}
	}`)
}

func TestAccessors(t *testing.T) {
	rm := accessorFixture(t)

	if _, ok := rm.GetBlock("missing"); ok {
		t.Error("GetBlock(missing) should fail")
	}

	blocks := rm.AllBlocks()
	if len(blocks) != 2 {
		t.Fatalf("AllBlocks = %d, want 2 (dead b2 filtered)", len(blocks))
	}
	if blocks[0].ID != "b1" || blocks[1].ID != "b3" {
		t.Errorf("AllBlocks order = %s, %s", blocks[0].ID, blocks[1].ID)
	}
	if got := (RecordMap{}).AllBlocks(); len(got) != 0 {
		t.Errorf("AllBlocks on empty = %v", got)
	}

	if c, ok := rm.FirstCollection(); !ok || c.ID != "col-1" {
		t.Errorf("FirstCollection = %+v ok=%v", c, ok)
	}
	if _, ok := (RecordMap{}).FirstCollection(); ok {
		t.Error("FirstCollection on empty should fail")
	}

	if id, ok := rm.FirstCollectionViewID(); !ok || id != "view-1" {
		t.Errorf("FirstCollectionViewID = %q ok=%v", id, ok)
	}
	if _, ok := (RecordMap{}).FirstCollectionViewID(); ok {
		t.Error("FirstCollectionViewID on empty should fail")
	}

	if u, ok := rm.FirstUser(); !ok || u.ID != "u1" {
		t.Errorf("FirstUser = %+v ok=%v", u, ok)
	}
	if s, ok := rm.FirstSpace(); !ok || s.Name != "Example Workspace" {
		t.Errorf("FirstSpace = %+v ok=%v", s, ok)
	}
	if users := rm.AllUsers(); len(users) != 2 {
		t.Errorf("AllUsers = %d, want 2", len(users))
	}
	if d, ok := rm.GetDiscussion("d1"); !ok || d.Comments[0] != "c1" {
		t.Errorf("GetDiscussion = %+v ok=%v", d, ok)
	}
	if c, ok := rm.GetComment("c1"); !ok || c.Text.Plain() != "A comment" {
		t.Errorf("GetComment = %+v ok=%v", c, ok)
	}
	if u, ok := rm.GetUser("u1"); !ok || u.Email != "u1@example.com" {
		t.Errorf("GetUser = %+v ok=%v", u, ok)
	}
}

func TestRoleWrappedWireFormatEndToEnd(t *testing.T) {
	rm := decodeRecordMap(t, `{
		"block": {
			"b1": {"value": {"value": `+blockJSON("b1", "text", true)+`, "role": "reader"}},
			"b2": {"value": {"value": `+blockJSON("b2", "text", false)+`, "role": "reader"}}
		},
		"notion_user": {
			"u1": {"value": {"value": {"id": "u1", "version": 1, "email": "u1@example.com", "given_name": "T", "family_name": "U"}, "role": "reader"}}
		}
	}`)

	if b, ok := rm.GetBlock("b1"); !ok || b.ID != "b1" {
		t.Errorf("b1 = %+v ok=%v", b, ok)
	}
	if u, ok := rm.GetUser("u1"); !ok || u.ID != "u1" {
		t.Errorf("u1 = %+v ok=%v", u, ok)
	}
	// A dead block nested inside a role wrapper is still filtered out.
	blocks := rm.AllBlocks()
	if len(blocks) != 1 || blocks[0].ID != "b1" {
		t.Errorf("AllBlocks = %+v", blocks)
	}
}

func TestRichTextRoundTrip(t *testing.T) {
	wire := `[["Hello "],["world",[["b"],["a","https://example.com"]]],["!",[["m","disc-1"]]]]`
	var rt RichText
	if err := json.Unmarshal([]byte(wire), &rt); err != nil {
		t.Fatal(err)
	}

	if rt.Plain() != "Hello world!" {
		t.Errorf("Plain = %q", rt.Plain())
	}
	if len(rt) != 3 || len(rt[1].Decorations) != 2 {
		t.Fatalf("segments = %+v", rt)
	}
	if rt[1].Decorations[0].Type != "b" || rt[1].Decorations[1].StringArg(0) != "https://example.com" {
		t.Errorf("decorations = %+v", rt[1].Decorations)
	}

	out, err := json.Marshal(rt)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != wire {
		t.Errorf("round trip:\n got %s\nwant %s", out, wire)
	}
}

func TestDecorationArgHelpers(t *testing.T) {
	var d Decoration
	if err := json.Unmarshal([]byte(`["d",{"start_date":"2026-01-15","end_date":"2026-01-20"}]`), &d); err != nil {
		t.Fatal(err)
	}
	obj := d.ObjectArg(0)
	if obj == nil || obj["start_date"] != "2026-01-15" {
		t.Errorf("ObjectArg = %v", obj)
	}
	if d.StringArg(0) != "" || d.StringArg(5) != "" {
		t.Error("StringArg should be empty for non-string/missing args")
	}
	if d.ObjectArg(5) != nil {
		t.Error("ObjectArg out of range should be nil")
	}
}
