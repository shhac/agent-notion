package v3

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

var opsNow = time.UnixMilli(1700000000000)

func argsMap(t *testing.T, op Operation) map[string]any {
	t.Helper()
	m, ok := op.Args.(map[string]any)
	if !ok {
		t.Fatalf("args is %T, want map", op.Args)
	}
	return m
}

func TestCreateBlockOps(t *testing.T) {
	ops := CreateBlockOps(CreateBlockParams{
		ID: "new-block-id", Type: "page",
		ParentID: "parent-id", ParentTable: "block",
		SpaceID: "space-id", UserID: "user-id",
		Properties: map[string]any{"title": NewRichText("Test")},
	}, opsNow)

	if len(ops) != 3 {
		t.Fatalf("ops = %d, want 3", len(ops))
	}

	set := ops[0]
	if set.Command != "set" || set.Pointer != (Pointer{Table: "block", ID: "new-block-id", SpaceID: "space-id"}) || len(set.Path) != 0 {
		t.Errorf("set op = %+v", set)
	}
	args := argsMap(t, set)
	for key, want := range map[string]any{
		"type": "page", "id": "new-block-id", "parent_id": "parent-id",
		"parent_table": "block", "alive": true, "space_id": "space-id",
		"created_by_id": "user-id", "created_time": opsNow.UnixMilli(),
	} {
		if args[key] != want {
			t.Errorf("args[%s] = %v, want %v", key, args[key], want)
		}
	}
	if _, ok := args["properties"]; !ok {
		t.Error("properties missing from set args")
	}

	listAfter := ops[1]
	if listAfter.Command != "listAfter" || listAfter.Pointer.ID != "parent-id" ||
		!reflect.DeepEqual(listAfter.Path, []string{"content"}) ||
		!reflect.DeepEqual(listAfter.Args, map[string]string{"id": "new-block-id"}) {
		t.Errorf("listAfter op = %+v", listAfter)
	}

	meta := ops[2]
	if meta.Command != "update" || meta.Pointer.ID != "parent-id" {
		t.Errorf("editMeta op = %+v", meta)
	}
	metaArgs := argsMap(t, meta)
	if metaArgs["last_edited_by_id"] != "user-id" || metaArgs["last_edited_by_table"] != "notion_user" {
		t.Errorf("editMeta args = %v", metaArgs)
	}
}

func TestCreateBlockOpsOmitsEmptyPropertiesAndFormat(t *testing.T) {
	ops := CreateBlockOps(CreateBlockParams{
		ID: "id", Type: "divider", ParentID: "p", ParentTable: "block",
		SpaceID: "s", UserID: "u",
	}, opsNow)
	args := argsMap(t, ops[0])
	if _, ok := args["properties"]; ok {
		t.Error("properties should be absent")
	}
	if _, ok := args["format"]; ok {
		t.Error("format should be absent")
	}

	ops = CreateBlockOps(CreateBlockParams{
		ID: "id", Type: "page", ParentID: "p", ParentTable: "block",
		SpaceID: "s", UserID: "u",
		Format: map[string]any{"page_icon": "🎉"},
	}, opsNow)
	if !reflect.DeepEqual(argsMap(t, ops[0])["format"], map[string]any{"page_icon": "🎉"}) {
		t.Errorf("format = %v", argsMap(t, ops[0])["format"])
	}
}

func TestCreateBlockOpsCollectionParentTargetsBlockTable(t *testing.T) {
	ops := CreateBlockOps(CreateBlockParams{
		ID: "id", Type: "page", ParentID: "collection-id", ParentTable: "collection",
		SpaceID: "s", UserID: "u",
	}, opsNow)
	if ops[1].Pointer.Table != "block" || ops[2].Pointer.Table != "block" {
		t.Errorf("listAfter/editMeta tables = %s/%s, want block", ops[1].Pointer.Table, ops[2].Pointer.Table)
	}
	// The block record itself still names the collection parent.
	if argsMap(t, ops[0])["parent_table"] != "collection" {
		t.Errorf("parent_table = %v", argsMap(t, ops[0])["parent_table"])
	}
}

func TestTrashBlockOps(t *testing.T) {
	ops := TrashBlockOps(TrashBlockParams{
		ID: "block-id", ParentID: "parent-id", ParentTable: "block",
		SpaceID: "space-id", UserID: "user-id",
	}, opsNow)

	if len(ops) != 3 {
		t.Fatalf("ops = %d, want 3", len(ops))
	}
	if ops[0].Command != "update" || ops[0].Pointer.ID != "block-id" || argsMap(t, ops[0])["alive"] != false {
		t.Errorf("update op = %+v", ops[0])
	}
	if ops[1].Command != "listRemove" || ops[1].Pointer.ID != "parent-id" ||
		!reflect.DeepEqual(ops[1].Path, []string{"content"}) ||
		!reflect.DeepEqual(ops[1].Args, map[string]string{"id": "block-id"}) {
		t.Errorf("listRemove op = %+v", ops[1])
	}
	if ops[2].Command != "update" || ops[2].Pointer.ID != "parent-id" {
		t.Errorf("editMeta op = %+v", ops[2])
	}
}

func TestArchivePageOps(t *testing.T) {
	ops := ArchivePageOps(ArchivePageParams{
		ID: "page-id", SpaceID: "space-id", UserID: "user-id", Archive: true,
	}, opsNow)
	if len(ops) != 2 {
		t.Fatalf("ops = %d, want 2", len(ops))
	}
	args := argsMap(t, ops[0])
	if args["archived_by_id"] != "user-id" || args["archived_by_table"] != "notion_user" ||
		args["archived_time"] != opsNow.UnixMilli() {
		t.Errorf("archive args = %v", args)
	}
	// alive must not be touched — Archive is distinct from Trash.
	if _, ok := args["alive"]; ok {
		t.Error("archive must not set alive")
	}
	if ops[1].Command != "update" || ops[1].Pointer.ID != "page-id" {
		t.Errorf("editMeta op = %+v", ops[1])
	}
}

func TestUnarchivePageOpsSendsExplicitNulls(t *testing.T) {
	ops := ArchivePageOps(ArchivePageParams{
		ID: "page-id", SpaceID: "space-id", UserID: "user-id", Archive: false,
	}, opsNow)
	args := argsMap(t, ops[0])
	for _, key := range []string{"archived_by_id", "archived_by_table", "archived_time"} {
		v, present := args[key]
		if !present || v != nil {
			t.Errorf("args[%s] = %v (present=%v), want explicit null", key, v, present)
		}
	}
	if _, ok := args["alive"]; ok {
		t.Error("unarchive must not set alive")
	}

	// The nulls must survive JSON marshaling — the server clears fields only
	// when they are present in args.
	raw, err := json.Marshal(ops[0])
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Args map[string]any `json:"args"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if v, present := decoded.Args["archived_time"]; !present || v != nil {
		t.Errorf("marshaled archived_time = %v (present=%v)", v, present)
	}
}

func TestMoveBlockOpsSameParent(t *testing.T) {
	ops := MoveBlockOps(MoveBlockParams{
		ID: "b1", OldParentID: "p1", OldParentTable: "block",
		NewParentID: "p1", NewParentTable: "block",
		SpaceID: "s", UserID: "u", AfterID: "b0",
	}, opsNow)

	if len(ops) != 3 {
		t.Fatalf("ops = %d, want 3 (no cross-parent update)", len(ops))
	}
	if ops[0].Command != "listRemove" {
		t.Errorf("ops[0] = %+v", ops[0])
	}
	if ops[1].Command != "listAfter" ||
		!reflect.DeepEqual(ops[1].Args, map[string]string{"id": "b1", "after": "b0"}) {
		t.Errorf("ops[1] = %+v", ops[1])
	}
	if ops[2].Command != "update" || ops[2].Pointer.ID != "p1" {
		t.Errorf("ops[2] = %+v", ops[2])
	}
}

func TestMoveBlockOpsCrossParent(t *testing.T) {
	ops := MoveBlockOps(MoveBlockParams{
		ID: "b1", OldParentID: "p1", OldParentTable: "block",
		NewParentID: "p2", NewParentTable: "block",
		SpaceID: "s", UserID: "u",
	}, opsNow)

	if len(ops) != 5 {
		t.Fatalf("ops = %d, want 5", len(ops))
	}
	// No AfterID → prepend via listBefore.
	if ops[1].Command != "listBefore" ||
		!reflect.DeepEqual(ops[1].Args, map[string]string{"id": "b1"}) {
		t.Errorf("insert op = %+v", ops[1])
	}
	reparent := argsMap(t, ops[2])
	if reparent["parent_id"] != "p2" || reparent["parent_table"] != "block" {
		t.Errorf("reparent args = %v", reparent)
	}
	if ops[3].Pointer.ID != "p1" || ops[4].Pointer.ID != "p2" {
		t.Errorf("editMeta targets = %s, %s", ops[3].Pointer.ID, ops[4].Pointer.ID)
	}
}

func TestUpdatePropertyOps(t *testing.T) {
	ops := UpdatePropertyOps(UpdatePropertyParams{
		ID: "page-id", SpaceID: "space-id", UserID: "user-id",
		Properties: map[string]any{"title": NewRichText("New Title"), "abc1": NewRichText("Done")},
	}, opsNow)

	if len(ops) != 3 {
		t.Fatalf("ops = %d, want 3 (2 properties + editMeta)", len(ops))
	}
	// Sorted key order: abc1 before title.
	if !reflect.DeepEqual(ops[0].Path, []string{"properties", "abc1"}) ||
		!reflect.DeepEqual(ops[1].Path, []string{"properties", "title"}) {
		t.Errorf("paths = %v, %v", ops[0].Path, ops[1].Path)
	}
	if ops[2].Command != "update" {
		t.Errorf("ops[2] = %+v", ops[2])
	}

	ops = UpdatePropertyOps(UpdatePropertyParams{
		ID: "page-id", SpaceID: "s", UserID: "u",
		Format: map[string]any{"page_icon": "🎯"},
	}, opsNow)
	if len(ops) != 2 || !reflect.DeepEqual(ops[0].Path, []string{"format", "page_icon"}) || ops[0].Args != "🎯" {
		t.Errorf("format ops = %+v", ops)
	}

	ops = UpdatePropertyOps(UpdatePropertyParams{ID: "page-id", SpaceID: "s", UserID: "u"}, opsNow)
	if len(ops) != 1 || ops[0].Command != "update" {
		t.Errorf("empty update ops = %+v", ops)
	}
}

func TestCreateCommentOps(t *testing.T) {
	ops := CreateCommentOps(CreateCommentParams{
		DiscussionID: "disc-1", CommentID: "comm-1", PageID: "page-1",
		SpaceID: "space-1", UserID: "user-1", Text: "This is a comment",
	}, opsNow)

	if len(ops) != 6 {
		t.Fatalf("ops = %d, want 6", len(ops))
	}

	disc := argsMap(t, ops[0])
	if ops[0].Pointer.Table != "discussion" || disc["parent_id"] != "page-1" ||
		disc["parent_table"] != "block" || disc["resolved"] != false || disc["alive"] != true {
		t.Errorf("discussion op = %+v args=%v", ops[0], disc)
	}

	if ops[1].Command != "listAfter" || ops[1].Pointer.Table != "block" ||
		!reflect.DeepEqual(ops[1].Path, []string{"discussions"}) {
		t.Errorf("link-discussion op = %+v", ops[1])
	}

	comment := argsMap(t, ops[2])
	if ops[2].Pointer.Table != "comment" || comment["parent_id"] != "disc-1" ||
		comment["created_by_id"] != "user-1" {
		t.Errorf("comment op = %+v args=%v", ops[2], comment)
	}
	if text, ok := comment["text"].(RichText); !ok || text.Plain() != "This is a comment" {
		t.Errorf("comment text = %v", comment["text"])
	}

	if ops[3].Command != "listAfter" || ops[3].Pointer.Table != "discussion" ||
		!reflect.DeepEqual(ops[3].Path, []string{"comments"}) {
		t.Errorf("link-comment op = %+v", ops[3])
	}
	if !reflect.DeepEqual(ops[4].Path, []string{"created_time"}) || ops[4].Args != opsNow.UnixMilli() {
		t.Errorf("created_time op = %+v", ops[4])
	}
	if !reflect.DeepEqual(ops[5].Path, []string{"last_edited_time"}) {
		t.Errorf("last_edited_time op = %+v", ops[5])
	}

	for i, op := range ops {
		if op.Pointer.SpaceID != "space-1" {
			t.Errorf("ops[%d] spaceId = %q", i, op.Pointer.SpaceID)
		}
	}
}

func TestCreateInlineCommentOps(t *testing.T) {
	updatedTitle := RichText{
		{Text: "hello", Decorations: []Decoration{{Type: "m", Args: []any{"disc-1"}}}},
		{Text: " world"},
	}
	ops := CreateInlineCommentOps(CreateInlineCommentParams{
		DiscussionID: "disc-1", CommentID: "comm-1", BlockID: "block-1",
		SpaceID: "space-1", UserID: "user-1", Text: "Great point!",
		UpdatedTitle: updatedTitle,
	}, opsNow)

	if len(ops) != 8 {
		t.Fatalf("ops = %d, want 8", len(ops))
	}

	disc := argsMap(t, ops[0])
	if disc["parent_id"] != "block-1" || disc["parent_table"] != "block" || disc["type"] != "default" {
		t.Errorf("discussion args = %v", disc)
	}
	context, ok := disc["context"].(RichText)
	if !ok || len(context) != 1 || context[0].Text != "hello" {
		t.Errorf("context = %v", disc["context"])
	}

	if ops[1].Command != "listAfter" || ops[1].Pointer.ID != "block-1" ||
		!reflect.DeepEqual(ops[1].Path, []string{"discussions"}) {
		t.Errorf("link op = %+v", ops[1])
	}

	comment := argsMap(t, ops[2])
	if text, ok := comment["text"].(RichText); !ok || text.Plain() != "Great point!" {
		t.Errorf("comment text = %v", comment["text"])
	}

	title := ops[6]
	if title.Command != "set" || title.Pointer.ID != "block-1" ||
		!reflect.DeepEqual(title.Path, []string{"properties", "title"}) ||
		!reflect.DeepEqual(title.Args, updatedTitle) {
		t.Errorf("title op = %+v", title)
	}
	if ops[7].Command != "update" || ops[7].Pointer.ID != "block-1" {
		t.Errorf("editMeta op = %+v", ops[7])
	}
}

func TestInlineCommentContextExtraction(t *testing.T) {
	// A segment carrying two discussion marks keeps all its decorations.
	title := RichText{
		{Text: "hello", Decorations: []Decoration{
			{Type: "m", Args: []any{"disc-old"}},
			{Type: "m", Args: []any{"disc-2"}},
		}},
		{Text: " world"},
	}
	ops := CreateInlineCommentOps(CreateInlineCommentParams{
		DiscussionID: "disc-2", CommentID: "c", BlockID: "b",
		SpaceID: "s", UserID: "u", Text: "x", UpdatedTitle: title,
	}, opsNow)
	context := argsMap(t, ops[0])["context"].(RichText)
	if len(context) != 1 || len(context[0].Decorations) != 2 {
		t.Errorf("context = %+v", context)
	}

	// Multiple contiguous anchored segments are all extracted.
	title = RichText{
		{Text: "hello ", Decorations: []Decoration{{Type: "m", Args: []any{"disc-3"}}}},
		{Text: "world", Decorations: []Decoration{{Type: "b"}, {Type: "m", Args: []any{"disc-3"}}}},
		{Text: "!"},
	}
	ops = CreateInlineCommentOps(CreateInlineCommentParams{
		DiscussionID: "disc-3", CommentID: "c", BlockID: "b",
		SpaceID: "s", UserID: "u", Text: "x", UpdatedTitle: title,
	}, opsNow)
	context = argsMap(t, ops[0])["context"].(RichText)
	if len(context) != 2 || context[0].Text != "hello " || context[1].Text != "world" {
		t.Errorf("context = %+v", context)
	}
}

func TestOfficialBlockToV3Args(t *testing.T) {
	richText := func(contents ...string) []any {
		segments := make([]any, len(contents))
		for i, c := range contents {
			segments[i] = map[string]any{"type": "text", "text": map[string]any{"content": c}}
		}
		return segments
	}
	title := func(text string) map[string]any {
		return map[string]any{"title": NewRichText(text)}
	}

	cases := []struct {
		name     string
		block    map[string]any
		wantType string
		wantProp map[string]any
		wantFmt  map[string]any
	}{
		{"paragraph", map[string]any{"type": "paragraph", "paragraph": map[string]any{"rich_text": richText("Hello world")}}, "text", title("Hello world"), nil},
		{"heading_1", map[string]any{"type": "heading_1", "heading_1": map[string]any{"rich_text": richText("Title")}}, "header", title("Title"), nil},
		{"heading_2", map[string]any{"type": "heading_2", "heading_2": map[string]any{"rich_text": richText("Subtitle")}}, "sub_header", title("Subtitle"), nil},
		{"heading_3", map[string]any{"type": "heading_3", "heading_3": map[string]any{"rich_text": richText("Sub-subtitle")}}, "sub_sub_header", title("Sub-subtitle"), nil},
		{"bulleted", map[string]any{"type": "bulleted_list_item", "bulleted_list_item": map[string]any{"rich_text": richText("Item")}}, "bulleted_list", title("Item"), nil},
		{"numbered", map[string]any{"type": "numbered_list_item", "numbered_list_item": map[string]any{"rich_text": richText("Step 1")}}, "numbered_list", title("Step 1"), nil},
		{"to_do checked", map[string]any{"type": "to_do", "to_do": map[string]any{"rich_text": richText("Task"), "checked": true}}, "to_do",
			map[string]any{"title": NewRichText("Task"), "checked": NewRichText("Yes")}, nil},
		{"to_do unchecked", map[string]any{"type": "to_do", "to_do": map[string]any{"rich_text": richText("Task"), "checked": false}}, "to_do", title("Task"), nil},
		{"code", map[string]any{"type": "code", "code": map[string]any{"rich_text": richText("const x = 1;"), "language": "typescript"}}, "code",
			map[string]any{"title": NewRichText("const x = 1;"), "language": NewRichText("typescript")}, nil},
		{"quote", map[string]any{"type": "quote", "quote": map[string]any{"rich_text": richText("Famous quote")}}, "quote", title("Famous quote"), nil},
		{"divider", map[string]any{"type": "divider", "divider": map[string]any{}}, "divider", nil, nil},
		{"callout", map[string]any{"type": "callout", "callout": map[string]any{"rich_text": richText("Note"), "icon": map[string]any{"emoji": "💡"}}}, "callout",
			title("Note"), map[string]any{"page_icon": "💡"}},
		{"bookmark", map[string]any{"type": "bookmark", "bookmark": map[string]any{"url": "https://example.com"}}, "bookmark",
			map[string]any{"link": NewRichText("https://example.com")}, nil},
		{"image", map[string]any{"type": "image", "image": map[string]any{"url": "https://example.com/img.png"}}, "image",
			map[string]any{"source": NewRichText("https://example.com/img.png")}, nil},
		{"equation", map[string]any{"type": "equation", "equation": map[string]any{"expression": "E = mc^2"}}, "equation", title("E = mc^2"), nil},
		{"multi-segment", map[string]any{"type": "paragraph", "paragraph": map[string]any{"rich_text": richText("Hello ", "world")}}, "text", title("Hello world"), nil},
		{"unknown type", map[string]any{"type": "unknown_custom_type"}, "unknown_custom_type", nil, nil},
	}

	for _, c := range cases {
		got := OfficialBlockToV3Args(c.block)
		if got.Type != c.wantType {
			t.Errorf("%s: type = %q, want %q", c.name, got.Type, c.wantType)
		}
		if !reflect.DeepEqual(got.Properties, c.wantProp) {
			t.Errorf("%s: properties = %#v, want %#v", c.name, got.Properties, c.wantProp)
		}
		if !reflect.DeepEqual(got.Format, c.wantFmt) {
			t.Errorf("%s: format = %#v, want %#v", c.name, got.Format, c.wantFmt)
		}
	}
}
