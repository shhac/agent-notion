package v3

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/shhac/agent-notion/internal/notion"
)

// --- Synthetic fixtures (placeholders only — never real IDs or content) ---

func newBlock(id string) *Block {
	alive := true
	return &Block{
		ID:             id,
		Type:           "page",
		Version:        1,
		CreatedTime:    1700000000000,
		LastEditedTime: 1700001000000,
		ParentID:       "parent-1",
		ParentTable:    "block",
		Alive:          &alive,
		SpaceID:        "space-1",
	}
}

func newCollection(id string) *Collection {
	return &Collection{
		ID:          id,
		Version:     1,
		Name:        NewRichText("Test DB"),
		Schema:      map[string]PropertySchema{},
		ParentID:    "parent-1",
		ParentTable: "block",
	}
}

func newUser(id string) *User {
	return &User{
		ID:         id,
		Version:    1,
		Email:      "test@example.com",
		GivenName:  "Test",
		FamilyName: "User",
	}
}

func newComment(id string) *Comment {
	alive := true
	return &Comment{
		ID:             id,
		Version:        1,
		Alive:          &alive,
		ParentID:       "disc-1",
		ParentTable:    "discussion",
		Text:           NewRichText("Hello"),
		CreatedByID:    "user-1",
		CreatedByTable: "notion_user",
		CreatedTime:    1700000000000,
		LastEditedTime: 1700000000000,
	}
}

// mention builds a single [text, [[type, id]]] rich-text segment.
func mention(text, decType, id string) Segment {
	return Segment{Text: text, Decorations: []Decoration{{Type: decType, Args: []any{id}}}}
}

func eq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// =============================================================================
// RichText.Plain (v3RichTextToPlain)
// =============================================================================

func TestRichTextPlain(t *testing.T) {
	tests := []struct {
		name string
		rt   RichText
		want string
	}{
		{"single segment", RichText{{Text: "Hello"}}, "Hello"},
		{"multiple segments", RichText{{Text: "Hello "}, {Text: "world"}}, "Hello world"},
		{"ignores decorations", RichText{{Text: "bold", Decorations: []Decoration{{Type: "b"}}}}, "bold"},
		{"nil", nil, ""},
		{"empty", RichText{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rt.Plain(); got != tt.want {
				t.Errorf("Plain() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// RichText.Render (inline mention resolution)
// =============================================================================

// mentionRecordMap holds a person (U1) and a page (P1) for mention resolution.
func mentionRecordMap(t *testing.T) RecordMap {
	t.Helper()
	var rm RecordMap
	raw := `{
		"notion_user": {
			"U1": {"value": {"id":"U1","given_name":"Ivan","family_name":"Tchernev","email":"ivan@example.com"}},
			"U2": {"value": {"id":"U2","email":"ops@example.com"}}
		},
		"block": {
			"P1": {"value": {"id":"P1","type":"page","properties":{"title":[["Roadmap"]]}}},
			"P2": {"value": {"id":"P2","type":"page","properties":{"title":[[""]]}}}
		}
	}`
	if err := json.Unmarshal([]byte(raw), &rm); err != nil {
		t.Fatal(err)
	}
	return rm
}

func dateSegment(obj map[string]any) Segment {
	return Segment{Text: "‣", Decorations: []Decoration{{Type: "d", Args: []any{obj}}}}
}

func TestRichTextRender(t *testing.T) {
	rm := mentionRecordMap(t)
	tests := []struct {
		name string
		rt   RichText
		want string
	}{
		{"person mention", RichText{mention("‣", "u", "U1")}, "@Ivan Tchernev"},
		{"person mention without name falls back to email", RichText{mention("‣", "u", "U2")}, "ops@example.com"},
		{"page mention", RichText{mention("‣", "p", "P1")}, "Roadmap"},
		{"page mention with empty title keeps placeholder", RichText{mention("‣", "p", "P2")}, "‣"},
		{"page mention missing from record map keeps placeholder", RichText{mention("‣", "p", "MISSING")}, "‣"},
		{"date mention", RichText{dateSegment(map[string]any{"start_date": "2026-07-06"})}, "2026-07-06"},
		{"date with time", RichText{dateSegment(map[string]any{"start_date": "2026-07-06", "start_time": "14:30"})}, "2026-07-06 14:30"},
		{"date range", RichText{dateSegment(map[string]any{"start_date": "2026-07-06", "end_date": "2026-07-08"})}, "2026-07-06 → 2026-07-08"},
		{"date range with time", RichText{dateSegment(map[string]any{"start_date": "2026-07-06", "start_time": "09:00", "end_date": "2026-07-08"})}, "2026-07-06 09:00 → 2026-07-08"},
		{"text before mention", RichText{{Text: "cc "}, mention("‣", "u", "U1")}, "cc @Ivan Tchernev"},
		{"bold decoration is not a mention", RichText{{Text: "bold", Decorations: []Decoration{{Type: "b"}}}}, "bold"},
		{"unresolvable person keeps placeholder", RichText{mention("‣", "u", "MISSING")}, "‣"},
		{"plain text unchanged", RichText{{Text: "Hello world"}}, "Hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rt.Render(rm); got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNormalizeBlockResolvesMention pins the wiring: a text block whose content
// carries a person mention comes out with the resolved name, not the ‣ glyph.
func TestNormalizeBlockResolvesMention(t *testing.T) {
	rm := mentionRecordMap(t)
	alive := true
	b := &Block{
		ID:         "b1",
		Type:       "text",
		Alive:      &alive,
		Properties: map[string]RichText{"title": {{Text: "assign to "}, mention("‣", "u", "U1")}},
	}
	if got := NormalizeBlock(b, rm).RichText; got != "assign to @Ivan Tchernev" {
		t.Errorf("RichText = %q, want %q", got, "assign to @Ivan Tchernev")
	}
}

// =============================================================================
// ParentFor (v3Parent)
// =============================================================================

func TestParentFor(t *testing.T) {
	eq(t, ParentFor("collection", "col-1"), &notion.ParentRef{Type: "database", ID: "col-1"})
	eq(t, ParentFor("block", "block-1"), &notion.ParentRef{Type: "page", ID: "block-1"})
	eq(t, ParentFor("space", "space-1"), &notion.ParentRef{Type: "workspace", ID: "space-1"})
	if got := ParentFor("something_else", "id-1"); got != nil {
		t.Errorf("ParentFor(unknown) = %#v, want nil", got)
	}
}

// =============================================================================
// FlattenPropertyValue (flattenV3PropertyValue)
// =============================================================================

func TestFlattenPropertyValue(t *testing.T) {
	sch := func(typ string) PropertySchema { return PropertySchema{Name: "prop", Type: typ} }

	tests := []struct {
		name  string
		value RichText
		typ   string
		want  any
	}{
		{"title", RichText{{Text: "My Title"}}, "title", "My Title"},
		{"text", RichText{{Text: "Some text"}}, "text", "Some text"},
		{"number", RichText{{Text: "42"}}, "number", float64(42)},
		{"number empty", nil, "number", nil},
		{"select", RichText{{Text: "Option A"}}, "select", "Option A"},
		{"select empty", RichText{{Text: ""}}, "select", nil},
		{"multi_select splits", RichText{{Text: "A,B,C"}}, "multi_select", []string{"A", "B", "C"}},
		{"multi_select empty", nil, "multi_select", []string{}},
		{"status", RichText{{Text: "Done"}}, "status", "Done"},
		{"status empty", RichText{{Text: ""}}, "status", nil},
		{"checkbox yes", RichText{{Text: "Yes"}}, "checkbox", true},
		{"checkbox no", RichText{{Text: "No"}}, "checkbox", false},
		{"checkbox undefined", nil, "checkbox", false},
		{"url", RichText{{Text: "https://example.com"}}, "url", "https://example.com"},
		{"url empty", nil, "url", nil},
		{"email", RichText{{Text: "a@b.com"}}, "email", "a@b.com"},
		{"phone_number", RichText{{Text: "555-1234"}}, "phone_number", "555-1234"},
		{
			"date with decoration",
			RichText{{Text: "‣", Decorations: []Decoration{{Type: "d", Args: []any{map[string]any{"start_date": "2024-01-15", "end_date": "2024-01-20"}}}}}},
			"date",
			map[string]any{"start": "2024-01-15", "end": "2024-01-20"},
		},
		{
			"date decoration no end",
			RichText{{Text: "‣", Decorations: []Decoration{{Type: "d", Args: []any{map[string]any{"start_date": "2024-01-15"}}}}}},
			"date",
			map[string]any{"start": "2024-01-15", "end": nil},
		},
		{"date no decoration", RichText{{Text: "2024-01-15"}}, "date", map[string]any{"start": "2024-01-15", "end": nil}},
		{"date empty", nil, "date", nil},
		{"person", RichText{mention("‣", "u", "user-1"), mention("‣", "u", "user-2")}, "person", []map[string]string{{"id": "user-1"}, {"id": "user-2"}}},
		{"person empty", nil, "person", []map[string]string{}},
		{"people", RichText{mention("‣", "u", "user-1")}, "people", []map[string]string{{"id": "user-1"}}},
		{"relation", RichText{mention("‣", "p", "page-1"), mention("‣", "p", "page-2")}, "relation", []map[string]string{{"id": "page-1"}, {"id": "page-2"}}},
		{"relation empty", nil, "relation", []map[string]string{}},
		{"created_time", RichText{{Text: "1700000000000"}}, "created_time", "1700000000000"},
		{"last_edited_time", RichText{{Text: "1700000000000"}}, "last_edited_time", "1700000000000"},
		{"created_by", RichText{{Text: "user-1"}}, "created_by", map[string]string{"id": "user-1"}},
		{"last_edited_by", RichText{{Text: "user-1"}}, "last_edited_by", map[string]string{"id": "user-1"}},
		{"formula", RichText{{Text: "42"}}, "formula", "42"},
		{"rollup", RichText{{Text: "100"}}, "rollup", "100"},
		{"files", RichText{{Text: "file.pdf"}}, "files", []map[string]any{{"name": "file.pdf", "url": nil}}},
		{"files empty", nil, "files", []map[string]any{}},
		{"unique_id", RichText{{Text: "PREFIX-42"}}, "unique_id", "PREFIX-42"},
		{"unknown", RichText{{Text: "hello"}}, "custom_type", "hello"},
		{"unknown empty", nil, "custom_type", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eq(t, FlattenPropertyValue(tt.value, sch(tt.typ)), tt.want)
		})
	}
}

// =============================================================================
// FlattenProperties (flattenV3Properties)
// =============================================================================

func TestFlattenProperties(t *testing.T) {
	t.Run("maps schema IDs to names", func(t *testing.T) {
		properties := map[string]RichText{
			"title": NewRichText("My Page"),
			"abc1":  NewRichText("Done"),
		}
		schema := map[string]PropertySchema{
			"title": {Name: "Name", Type: "title"},
			"abc1":  {Name: "Status", Type: "status"},
		}
		eq(t, FlattenProperties(properties, schema), map[string]any{"Name": "My Page", "Status": "Done"})
	})

	t.Run("nil properties", func(t *testing.T) {
		schema := map[string]PropertySchema{"abc1": {Name: "Status", Type: "status"}}
		eq(t, FlattenProperties(nil, schema), map[string]any{})
	})

	t.Run("defaults for missing values", func(t *testing.T) {
		schema := map[string]PropertySchema{
			"title": {Name: "Name", Type: "title"},
			"abc1":  {Name: "Tags", Type: "multi_select"},
		}
		got := FlattenProperties(map[string]RichText{"title": NewRichText("Hello")}, schema)
		eq(t, got["Name"], "Hello")
		eq(t, got["Tags"], []string{})
	})
}

// =============================================================================
// NormalizeBlock (normalizeV3Block)
// =============================================================================

func TestNormalizeBlock(t *testing.T) {
	t.Run("text to paragraph", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "text"
		b.Properties = map[string]RichText{"title": NewRichText("Hello world")}
		got := NormalizeBlock(b, nil)
		eq(t, got.ID, "b1")
		eq(t, got.Type, "paragraph")
		eq(t, got.RichText, "Hello world")
		eq(t, got.HasChildren, false)
	})

	t.Run("header types", func(t *testing.T) {
		for typ, want := range map[string]string{"header": "heading_1", "sub_header": "heading_2", "sub_sub_header": "heading_3"} {
			b := newBlock("b")
			b.Type = typ
			b.Properties = map[string]RichText{"title": NewRichText("H")}
			if got := NormalizeBlock(b, nil).Type; got != want {
				t.Errorf("%s → %q, want %q", typ, got, want)
			}
		}
	})

	t.Run("hasChildren from content", func(t *testing.T) {
		b := newBlock("b1")
		b.Content = []string{"child-1", "child-2"}
		if !NormalizeBlock(b, nil).HasChildren {
			t.Error("expected hasChildren true")
		}
	})

	t.Run("to_do checked", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "to_do"
		b.Properties = map[string]RichText{"title": NewRichText("Task"), "checked": NewRichText("Yes")}
		got := NormalizeBlock(b, nil)
		eq(t, got.Type, "to_do")
		if got.Checked == nil || !*got.Checked {
			t.Errorf("checked = %v, want true", got.Checked)
		}
	})

	t.Run("to_do unchecked", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "to_do"
		b.Properties = map[string]RichText{"title": NewRichText("Task")}
		got := NormalizeBlock(b, nil)
		if got.Checked == nil || *got.Checked {
			t.Errorf("checked = %v, want false", got.Checked)
		}
	})

	t.Run("code with language", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "code"
		b.Properties = map[string]RichText{"title": NewRichText("const x = 1"), "language": NewRichText("typescript")}
		got := NormalizeBlock(b, nil)
		eq(t, got.Type, "code")
		eq(t, got.Language, "typescript")
		eq(t, got.RichText, "const x = 1")
	})

	t.Run("image with display_source", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "image"
		b.Format = map[string]any{"display_source": "https://example.com/img.png"}
		b.Properties = map[string]RichText{"source": NewRichText("https://fallback.com/img.png"), "caption": NewRichText("A caption")}
		got := NormalizeBlock(b, nil)
		eq(t, got.Type, "image")
		eq(t, got.URL, "https://example.com/img.png")
		eq(t, got.Caption, "A caption")
	})

	t.Run("image falls back to source", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "image"
		b.Properties = map[string]RichText{"source": NewRichText("https://fallback.com/img.png")}
		eq(t, NormalizeBlock(b, nil).URL, "https://fallback.com/img.png")
	})

	t.Run("bookmark", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "bookmark"
		b.Properties = map[string]RichText{"link": NewRichText("https://example.com"), "description": NewRichText("A site")}
		got := NormalizeBlock(b, nil)
		eq(t, got.Type, "bookmark")
		eq(t, got.URL, "https://example.com")
		eq(t, got.Caption, "A site")
	})

	t.Run("equation", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "equation"
		b.Properties = map[string]RichText{"title": NewRichText("E = mc^2")}
		got := NormalizeBlock(b, nil)
		eq(t, got.Type, "equation")
		eq(t, got.Expression, "E = mc^2")
	})

	t.Run("page with title", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "page"
		b.Properties = map[string]RichText{"title": NewRichText("Child Page")}
		got := NormalizeBlock(b, nil)
		eq(t, got.Type, "child_page")
		eq(t, got.Title, "Child Page")
	})

	t.Run("collection_view_page", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "collection_view_page"
		b.Properties = map[string]RichText{"title": NewRichText("My DB")}
		got := NormalizeBlock(b, nil)
		eq(t, got.Type, "child_database")
		eq(t, got.Title, "My DB")
	})

	t.Run("callout with emoji", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "callout"
		b.Properties = map[string]RichText{"title": NewRichText("Note")}
		b.Format = map[string]any{"page_icon": "💡"}
		got := NormalizeBlock(b, nil)
		eq(t, got.Type, "callout")
		eq(t, got.Emoji, "💡")
	})

	t.Run("embed", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "embed"
		b.Properties = map[string]RichText{"source": NewRichText("https://youtube.com/embed/abc")}
		eq(t, NormalizeBlock(b, nil).URL, "https://youtube.com/embed/abc")
	})

	t.Run("video", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "video"
		b.Properties = map[string]RichText{"source": NewRichText("https://example.com/video.mp4"), "caption": NewRichText("Video")}
		got := NormalizeBlock(b, nil)
		eq(t, got.Type, "video")
		eq(t, got.URL, "https://example.com/video.mp4")
		eq(t, got.Caption, "Video")
	})

	t.Run("unknown type passes through", func(t *testing.T) {
		b := newBlock("b1")
		b.Type = "fancy_new_block"
		eq(t, NormalizeBlock(b, nil).Type, "fancy_new_block")
	})
}

// =============================================================================
// ToSearchResult (transformV3SearchResult)
// =============================================================================

func TestToSearchResult(t *testing.T) {
	t.Run("page block", func(t *testing.T) {
		b := newBlock("page-1")
		b.Type = "page"
		b.Properties = map[string]RichText{"title": NewRichText("My Page")}
		b.ParentID = "parent-1"
		b.ParentTable = "block"
		got := ToSearchResult(b)
		eq(t, got.ID, "page-1")
		eq(t, got.Type, "page")
		eq(t, got.Title, "My Page")
		eq(t, got.URL, "https://www.notion.so/page1")
		eq(t, got.Parent, &notion.ParentRef{Type: "page", ID: "parent-1"})
		eq(t, got.LastEditedAt, "2023-11-14T22:30:00.000Z")
	})

	t.Run("collection_view_page is database", func(t *testing.T) {
		b := newBlock("db-1")
		b.Type = "collection_view_page"
		b.Properties = map[string]RichText{"title": NewRichText("My DB")}
		eq(t, ToSearchResult(b).Type, "database")
	})

	t.Run("collection_view is database", func(t *testing.T) {
		b := newBlock("db-2")
		b.Type = "collection_view"
		b.Properties = map[string]RichText{"title": NewRichText("Inline DB")}
		eq(t, ToSearchResult(b).Type, "database")
	})
}

// =============================================================================
// ToDatabaseListItem (transformV3DatabaseListItem)
// =============================================================================

func TestToDatabaseListItem(t *testing.T) {
	t.Run("with view page ID", func(t *testing.T) {
		c := newCollection("col-1")
		c.Name = NewRichText("Tasks")
		c.Schema = map[string]PropertySchema{
			"title": {Name: "Name", Type: "title"},
			"abc1":  {Name: "Status", Type: "status"},
		}
		c.ParentID = "parent-1"
		c.ParentTable = "block"
		got := ToDatabaseListItem(c, "view-page-1")
		eq(t, got.ID, "view-page-1")
		eq(t, got.Title, "Tasks")
		eq(t, got.PropertyCount, 2)
		eq(t, got.URL, "https://www.notion.so/viewpage1")
	})

	t.Run("falls back to parent_id", func(t *testing.T) {
		c := newCollection("col-1")
		c.ParentID = "parent-1"
		eq(t, ToDatabaseListItem(c, "").ID, "parent-1")
	})

	t.Run("empty schema", func(t *testing.T) {
		c := newCollection("col-1")
		eq(t, ToDatabaseListItem(c, "").PropertyCount, 0)
	})
}

// =============================================================================
// ToDatabaseDetail (transformV3DatabaseDetail)
// =============================================================================

func TestToDatabaseDetail(t *testing.T) {
	t.Run("schema properties", func(t *testing.T) {
		c := newCollection("col-1")
		c.Name = NewRichText("Projects")
		c.Description = NewRichText("All projects")
		c.Schema = map[string]PropertySchema{
			"title": {Name: "Name", Type: "title"},
			"abc1": {
				Name: "Status",
				Type: "select",
				Options: []PropertySchemaOption{
					{ID: "opt-1", Value: "Active", Color: "green"},
					{ID: "opt-2", Value: "Done", Color: "gray"},
				},
			},
		}
		got := ToDatabaseDetail(c, "view-page-1")
		eq(t, got.ID, "view-page-1")
		eq(t, got.Title, "Projects")
		eq(t, got.Description, "All projects")
		eq(t, got.Properties["Name"], notion.PropertyDefinition{ID: "title", Type: "title"})
		eq(t, got.Properties["Status"].Options, []notion.PropertyOption{
			{Name: "Active", Color: "green"},
			{Name: "Done", Color: "gray"},
		})
	})

	t.Run("groups", func(t *testing.T) {
		c := newCollection("col-1")
		c.Schema = map[string]PropertySchema{
			"abc1": {
				Name: "Status",
				Type: "status",
				Options: []PropertySchemaOption{
					{ID: "opt-1", Value: "Todo", Color: "gray"},
					{ID: "opt-2", Value: "In Progress", Color: "blue"},
					{ID: "opt-3", Value: "Done", Color: "green"},
				},
				Groups: []PropertySchemaGroup{
					{ID: "g1", Name: "Not Started", OptionIDs: []string{"opt-1"}},
					{ID: "g2", Name: "Active", OptionIDs: []string{"opt-2"}},
				},
			},
		}
		eq(t, ToDatabaseDetail(c, "").Properties["Status"].Groups, []notion.PropertyGroup{
			{Name: "Not Started", Options: []string{"Todo"}},
			{Name: "Active", Options: []string{"In Progress"}},
		})
	})

	t.Run("relatedDatabase", func(t *testing.T) {
		c := newCollection("col-1")
		c.Schema = map[string]PropertySchema{
			"abc1": {Name: "Related", Type: "relation", CollectionID: "col-2"},
		}
		eq(t, ToDatabaseDetail(c, "").Properties["Related"].RelatedDatabase, "col-2")
	})
}

// =============================================================================
// ToDatabaseSchema (transformV3DatabaseSchema)
// =============================================================================

func TestToDatabaseSchema(t *testing.T) {
	t.Run("property list", func(t *testing.T) {
		c := newCollection("col-1")
		c.Name = NewRichText("My DB")
		c.Schema = map[string]PropertySchema{
			"title": {Name: "Name", Type: "title"},
			"abc1": {
				Name: "Tags",
				Type: "multi_select",
				Options: []PropertySchemaOption{
					{ID: "opt-1", Value: "Frontend"},
					{ID: "opt-2", Value: "Backend"},
				},
			},
		}
		got := ToDatabaseSchema(c, "view-1")
		eq(t, got.ID, "view-1")
		eq(t, got.Title, "My DB")
		if len(got.Properties) != 2 {
			t.Fatalf("properties len = %d, want 2", len(got.Properties))
		}

		nameP := findSchemaProp(got.Properties, "Name")
		if nameP == nil || nameP.Type != "title" || nameP.ID != "title" {
			t.Errorf("Name prop = %#v", nameP)
		}
		tagsP := findSchemaProp(got.Properties, "Tags")
		if tagsP == nil {
			t.Fatal("Tags prop missing")
		}
		eq(t, tagsP.Options, []string{"Frontend", "Backend"})
	})

	t.Run("groups as record", func(t *testing.T) {
		c := newCollection("col-1")
		c.Schema = map[string]PropertySchema{
			"abc1": {
				Name: "Status",
				Type: "status",
				Options: []PropertySchemaOption{
					{ID: "o1", Value: "Todo"},
					{ID: "o2", Value: "Done"},
				},
				Groups: []PropertySchemaGroup{
					{ID: "g1", Name: "Open", OptionIDs: []string{"o1"}},
					{ID: "g2", Name: "Closed", OptionIDs: []string{"o2"}},
				},
			},
		}
		got := ToDatabaseSchema(c, "")
		eq(t, got.Properties[0].Groups, map[string][]string{
			"Open":   {"Todo"},
			"Closed": {"Done"},
		})
	})
}

func findSchemaProp(props []notion.SchemaProperty, name string) *notion.SchemaProperty {
	for i := range props {
		if props[i].Name == name {
			return &props[i]
		}
	}
	return nil
}

// =============================================================================
// ToQueryRow (transformV3QueryRow)
// =============================================================================

func TestToQueryRow(t *testing.T) {
	b := newBlock("row-1")
	b.Type = "page"
	b.Properties = map[string]RichText{
		"title": NewRichText("Task 1"),
		"abc1":  NewRichText("Done"),
	}
	schema := map[string]PropertySchema{
		"title": {Name: "Name", Type: "title"},
		"abc1":  {Name: "Status", Type: "status"},
	}
	got := ToQueryRow(b, schema)
	eq(t, got.ID, "row-1")
	eq(t, got.URL, "https://www.notion.so/row1")
	eq(t, got.Properties, map[string]any{"Name": "Task 1", "Status": "Done"})
	eq(t, got.CreatedAt, "2023-11-14T22:13:20.000Z")
	eq(t, got.LastEditedAt, "2023-11-14T22:30:00.000Z")
}

// =============================================================================
// ToPageDetail (transformV3PageDetail)
// =============================================================================

func TestToPageDetail(t *testing.T) {
	t.Run("without schema", func(t *testing.T) {
		b := newBlock("page-1")
		b.Type = "page"
		b.Properties = map[string]RichText{"title": NewRichText("My Page")}
		b.ParentTable = "block"
		b.ParentID = "parent-1"
		got := ToPageDetail(b, nil)
		eq(t, got.ID, "page-1")
		eq(t, got.Properties, map[string]any{"title": "My Page"})
		eq(t, got.Parent, &notion.ParentRef{Type: "page", ID: "parent-1"})
		eq(t, got.Archived, false)
		if got.CreatedBy != nil || got.LastEditedBy != nil {
			t.Errorf("createdBy=%v lastEditedBy=%v, want nil", got.CreatedBy, got.LastEditedBy)
		}
	})

	t.Run("with schema", func(t *testing.T) {
		b := newBlock("row-1")
		b.ParentTable = "collection"
		b.ParentID = "col-1"
		b.Properties = map[string]RichText{"title": NewRichText("Row"), "abc1": NewRichText("Active")}
		schema := map[string]PropertySchema{
			"title": {Name: "Name", Type: "title"},
			"abc1":  {Name: "Status", Type: "status"},
		}
		got := ToPageDetail(b, schema)
		eq(t, got.Properties, map[string]any{"Name": "Row", "Status": "Active"})
		eq(t, got.Parent, &notion.ParentRef{Type: "database", ID: "col-1"})
	})

	t.Run("icon from format", func(t *testing.T) {
		b := newBlock("page-1")
		b.Format = map[string]any{"page_icon": "🎯"}
		eq(t, ToPageDetail(b, nil).Icon, &notion.Icon{Type: "emoji", Emoji: "🎯"})
	})

	t.Run("icon nil when no format", func(t *testing.T) {
		b := newBlock("page-1")
		if got := ToPageDetail(b, nil).Icon; got != nil {
			t.Errorf("icon = %#v, want nil", got)
		}
	})

	t.Run("archived when not alive", func(t *testing.T) {
		b := newBlock("page-1")
		alive := false
		b.Alive = &alive
		eq(t, ToPageDetail(b, nil).Archived, true)
	})
}

// =============================================================================
// ToUserItem / ToUserMe (transformV3User / transformV3UserMe)
// =============================================================================

func TestToUserItem(t *testing.T) {
	t.Run("full name", func(t *testing.T) {
		u := newUser("u1")
		u.GivenName = "Jane"
		u.FamilyName = "Doe"
		u.Email = "jane@example.com"
		u.ProfilePhoto = "https://photo.com/jane.jpg"
		eq(t, ToUserItem(u), notion.UserItem{
			ID:        "u1",
			Name:      "Jane Doe",
			Type:      "person",
			Email:     "jane@example.com",
			AvatarURL: "https://photo.com/jane.jpg",
		})
	})

	t.Run("only given name", func(t *testing.T) {
		u := newUser("u1")
		u.GivenName = "Jane"
		u.FamilyName = ""
		eq(t, ToUserItem(u).Name, "Jane")
	})

	t.Run("no name", func(t *testing.T) {
		u := newUser("u1")
		u.GivenName = ""
		u.FamilyName = ""
		eq(t, ToUserItem(u).Name, "")
	})
}

func TestToUserMe(t *testing.T) {
	t.Run("with space name", func(t *testing.T) {
		got := ToUserMe(newUser("u1"), "My Workspace")
		eq(t, got.ID, "u1")
		eq(t, got.Name, "Test User")
		eq(t, got.Type, "person")
		eq(t, got.WorkspaceName, "My Workspace")
	})

	t.Run("no space name", func(t *testing.T) {
		eq(t, ToUserMe(newUser("u1"), "").WorkspaceName, "")
	})
}

// =============================================================================
// ToCommentItem (transformV3Comment)
// =============================================================================

func TestToCommentItem(t *testing.T) {
	t.Run("with user", func(t *testing.T) {
		c := newComment("c1")
		c.Text = NewRichText("Great work!")
		c.CreatedByID = "u1"
		u := newUser("u1")
		u.GivenName = "Jane"
		u.FamilyName = "Doe"
		got := ToCommentItem(c, u, "")
		eq(t, got.ID, "c1")
		eq(t, got.Body, "Great work!")
		eq(t, got.Author, &notion.UserRef{ID: "u1", Name: "Jane Doe"})
		eq(t, got.CreatedAt, "2023-11-14T22:13:20.000Z")
	})

	t.Run("without user falls back to created_by_id", func(t *testing.T) {
		c := newComment("c1")
		c.CreatedByID = "u1"
		eq(t, ToCommentItem(c, nil, "").Author, &notion.UserRef{ID: "u1"})
	})

	t.Run("no author info", func(t *testing.T) {
		c := newComment("c1")
		c.CreatedByID = ""
		if got := ToCommentItem(c, nil, "").Author; got != nil {
			t.Errorf("author = %#v, want nil", got)
		}
	})

	t.Run("includes anchorText", func(t *testing.T) {
		c := newComment("c1")
		c.CreatedByID = "u1"
		eq(t, ToCommentItem(c, nil, "highlighted text").AnchorText, "highlighted text")
	})

	t.Run("omits anchorText", func(t *testing.T) {
		c := newComment("c1")
		c.CreatedByID = "u1"
		eq(t, ToCommentItem(c, nil, "").AnchorText, "")
	})
}

// =============================================================================
// ExtractAnchorText (extractAnchorText)
// =============================================================================

func TestExtractAnchorText(t *testing.T) {
	t.Run("matching discussion ID", func(t *testing.T) {
		rt := RichText{
			{Text: "Hello "},
			mention("world", "m", "disc-1"),
			{Text: "!"},
		}
		eq(t, ExtractAnchorText(rt, "disc-1"), "world")
	})

	t.Run("concatenates multiple segments", func(t *testing.T) {
		rt := RichText{
			{Text: "Hello "},
			{Text: "beautiful", Decorations: []Decoration{{Type: "b"}, {Type: "m", Args: []any{"disc-1"}}}},
			{Text: " ", Decorations: []Decoration{{Type: "m", Args: []any{"disc-1"}}}},
			{Text: "world", Decorations: []Decoration{{Type: "m", Args: []any{"disc-1"}}}},
			{Text: "!"},
		}
		eq(t, ExtractAnchorText(rt, "disc-1"), "beautiful world")
	})

	t.Run("non-matching discussion ID", func(t *testing.T) {
		rt := RichText{
			{Text: "Hello "},
			mention("world", "m", "disc-other"),
		}
		eq(t, ExtractAnchorText(rt, "disc-1"), "")
	})

	t.Run("nil rich text", func(t *testing.T) {
		eq(t, ExtractAnchorText(nil, "disc-1"), "")
	})

	t.Run("empty rich text", func(t *testing.T) {
		eq(t, ExtractAnchorText(RichText{}, "disc-1"), "")
	})

	t.Run("no decorations", func(t *testing.T) {
		eq(t, ExtractAnchorText(RichText{{Text: "Hello world"}}, "disc-1"), "")
	})
}

// =============================================================================
// AddDecorationToRange (addDecorationToRange) — not covered by the TS test
// suite, but exercised here for the split/error behavior.
// =============================================================================

func TestAddDecorationToRange(t *testing.T) {
	t.Run("splits a single segment", func(t *testing.T) {
		rt := RichText{{Text: "Hello world"}}
		got, err := AddDecorationToRange(rt, 0, 5, Decoration{Type: "m", Args: []any{"disc-1"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		eq(t, got, RichText{
			{Text: "Hello", Decorations: []Decoration{{Type: "m", Args: []any{"disc-1"}}}},
			{Text: " world"},
		})
	})

	t.Run("invalid range errors", func(t *testing.T) {
		if _, err := AddDecorationToRange(RichText{{Text: "x"}}, 2, 1, Decoration{Type: "m"}); err == nil {
			t.Error("expected error for invalid range")
		}
	})
}

// =============================================================================
// BuildPropertyValue / MapPropertiesToSchema — write direction, no TS test
// coverage; verified here for the contract-bearing error messages.
// =============================================================================

func TestBuildPropertyValue(t *testing.T) {
	t.Run("checkbox true", func(t *testing.T) {
		got, err := BuildPropertyValue(true, "checkbox")
		if err != nil {
			t.Fatal(err)
		}
		eq(t, got, NewRichText("Yes"))
	})

	t.Run("multi_select joins", func(t *testing.T) {
		got, err := BuildPropertyValue([]string{"A", "B"}, "multi_select")
		if err != nil {
			t.Fatal(err)
		}
		eq(t, got, NewRichText("A,B"))
	})

	t.Run("date errors", func(t *testing.T) {
		if _, err := BuildPropertyValue("2024-01-01", "date"); err == nil {
			t.Error("expected error for date type")
		}
	})
}

func TestMapPropertiesToSchema(t *testing.T) {
	schema := map[string]PropertySchema{
		"title": {Name: "Name", Type: "title"},
		"abc1":  {Name: "Status", Type: "status"},
	}
	got, err := MapPropertiesToSchema(map[string]any{"Name": "ignored", "Status": "Done"}, schema)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, got, map[string]RichText{"abc1": NewRichText("Done")})
}
