package markdown

import (
	"reflect"
	"testing"

	"github.com/shhac/agent-notion/internal/notion"
)

func boolPtr(b bool) *bool { return &b }

// TestFromBlocksSingleBlock exercises one block of each type through the public
// FromBlocks entry point, which renders with an empty top-level indent.
func TestFromBlocksSingleBlock(t *testing.T) {
	tests := []struct {
		name  string
		block notion.NormalizedBlock
		want  string
	}{
		{"paragraph", notion.NormalizedBlock{Type: "paragraph", RichText: "hello world"}, "hello world"},
		{"paragraph empty", notion.NormalizedBlock{Type: "paragraph", RichText: ""}, ""},
		{"heading_1", notion.NormalizedBlock{Type: "heading_1", RichText: "Title"}, "# Title"},
		{"heading_2", notion.NormalizedBlock{Type: "heading_2", RichText: "Sub"}, "## Sub"},
		{"heading_3", notion.NormalizedBlock{Type: "heading_3", RichText: "Sub sub"}, "### Sub sub"},
		{"bulleted", notion.NormalizedBlock{Type: "bulleted_list_item", RichText: "item"}, "- item"},
		{"numbered", notion.NormalizedBlock{Type: "numbered_list_item", RichText: "item"}, "1. item"},
		{"to_do unchecked nil", notion.NormalizedBlock{Type: "to_do", RichText: "task"}, "- [ ] task"},
		{"to_do unchecked false", notion.NormalizedBlock{Type: "to_do", RichText: "task", Checked: boolPtr(false)}, "- [ ] task"},
		{"to_do checked", notion.NormalizedBlock{Type: "to_do", RichText: "task", Checked: boolPtr(true)}, "- [x] task"},
		{"toggle", notion.NormalizedBlock{Type: "toggle", RichText: "more"}, "> ▶ more"},
		{"code no lang", notion.NormalizedBlock{Type: "code", RichText: "x := 1"}, "```\nx := 1\n```"},
		{"code with lang", notion.NormalizedBlock{Type: "code", RichText: "x := 1", Language: "go"}, "```go\nx := 1\n```"},
		{"code multiline", notion.NormalizedBlock{Type: "code", RichText: "a\nb", Language: "go"}, "```go\na\nb\n```"},
		{"quote", notion.NormalizedBlock{Type: "quote", RichText: "wisdom"}, "> wisdom"},
		{"callout default emoji", notion.NormalizedBlock{Type: "callout", RichText: "note"}, "> 💡 note"},
		{"callout custom emoji", notion.NormalizedBlock{Type: "callout", RichText: "note", Emoji: "⚠️"}, "> ⚠️ note"},
		{"divider", notion.NormalizedBlock{Type: "divider"}, "---"},
		{"image with caption", notion.NormalizedBlock{Type: "image", Caption: "a cat", URL: "https://example.test/cat.png"}, "![a cat](https://example.test/cat.png)"},
		{"image no caption", notion.NormalizedBlock{Type: "image", URL: "https://example.test/cat.png"}, "![image](https://example.test/cat.png)"},
		{"image no url", notion.NormalizedBlock{Type: "image"}, "![image]()"},
		{"bookmark caption", notion.NormalizedBlock{Type: "bookmark", Caption: "site", URL: "https://example.test"}, "[site](https://example.test)"},
		{"bookmark url only", notion.NormalizedBlock{Type: "bookmark", URL: "https://example.test"}, "[https://example.test](https://example.test)"},
		{"bookmark empty", notion.NormalizedBlock{Type: "bookmark"}, "[bookmark]()"},
		{"equation", notion.NormalizedBlock{Type: "equation", Expression: "a^2 + b^2"}, "$$a^2 + b^2$$"},
		{"equation empty", notion.NormalizedBlock{Type: "equation"}, "$$$$"},
		{"child_page titled", notion.NormalizedBlock{Type: "child_page", Title: "Notes"}, "📄 Notes"},
		{"child_page untitled", notion.NormalizedBlock{Type: "child_page"}, "📄 Untitled"},
		{"child_database titled", notion.NormalizedBlock{Type: "child_database", Title: "Tasks"}, "📊 Tasks"},
		{"child_database untitled", notion.NormalizedBlock{Type: "child_database"}, "📊 Untitled"},
		{"table_of_contents", notion.NormalizedBlock{Type: "table_of_contents"}, "[Table of Contents]"},
		{"breadcrumb", notion.NormalizedBlock{Type: "breadcrumb"}, "[Breadcrumb]"},
		{"column_list", notion.NormalizedBlock{Type: "column_list"}, ""},
		{"column", notion.NormalizedBlock{Type: "column"}, ""},
		{"synced_block", notion.NormalizedBlock{Type: "synced_block"}, ""},
		{"link_preview url", notion.NormalizedBlock{Type: "link_preview", URL: "https://example.test"}, "[https://example.test](https://example.test)"},
		{"link_preview empty", notion.NormalizedBlock{Type: "link_preview"}, "[link]()"},
		{"embed", notion.NormalizedBlock{Type: "embed", URL: "https://example.test"}, "[embed: https://example.test](https://example.test)"},
		{"embed empty", notion.NormalizedBlock{Type: "embed"}, "[embed: ]()"},
		{"video", notion.NormalizedBlock{Type: "video", URL: "https://example.test/v.mp4"}, "[video](https://example.test/v.mp4)"},
		{"pdf", notion.NormalizedBlock{Type: "pdf", URL: "https://example.test/f.pdf"}, "[pdf](https://example.test/f.pdf)"},
		{"audio", notion.NormalizedBlock{Type: "audio", URL: "https://example.test/a.mp3"}, "[audio](https://example.test/a.mp3)"},
		{"file caption", notion.NormalizedBlock{Type: "file", Caption: "doc", URL: "https://example.test/f"}, "[doc](https://example.test/f)"},
		{"file title fallback", notion.NormalizedBlock{Type: "file", Title: "report", URL: "https://example.test/f"}, "[report](https://example.test/f)"},
		{"file default", notion.NormalizedBlock{Type: "file", URL: "https://example.test/f"}, "[file](https://example.test/f)"},
		{"unsupported with text", notion.NormalizedBlock{Type: "mystery", RichText: "raw text"}, "raw text"},
		{"unsupported no text", notion.NormalizedBlock{Type: "mystery"}, "[unsupported: mystery]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromBlocks([]notion.NormalizedBlock{tt.block}, nil)
			if got != tt.want {
				t.Errorf("FromBlocks() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFromBlocksJoining verifies that non-empty lines are separated by blank
// lines and empty renders are dropped entirely.
func TestFromBlocksJoining(t *testing.T) {
	blocks := []notion.NormalizedBlock{
		{Type: "heading_1", RichText: "Title"},
		{Type: "paragraph", RichText: "First paragraph."},
		{Type: "column_list"}, // renders empty, dropped
		{Type: "paragraph", RichText: "Second paragraph."},
	}
	want := "# Title\n\nFirst paragraph.\n\nSecond paragraph."
	if got := FromBlocks(blocks, nil); got != want {
		t.Errorf("FromBlocks() = %q, want %q", got, want)
	}
}

// TestFromBlocksEmpty covers the empty-input edge case.
func TestFromBlocksEmpty(t *testing.T) {
	if got := FromBlocks(nil, nil); got != "" {
		t.Errorf("FromBlocks(nil) = %q, want empty", got)
	}
	if got := FromBlocks([]notion.NormalizedBlock{}, nil); got != "" {
		t.Errorf("FromBlocks(empty) = %q, want empty", got)
	}
}

// TestFromBlocksNesting checks child indentation and recursive nesting via
// childBlocksMap.
func TestFromBlocksNesting(t *testing.T) {
	blocks := []notion.NormalizedBlock{
		{ID: "parent", Type: "bulleted_list_item", RichText: "top", HasChildren: true},
	}
	children := map[string][]notion.NormalizedBlock{
		"parent": {
			{ID: "child", Type: "bulleted_list_item", RichText: "nested", HasChildren: true},
		},
		"child": {
			{Type: "bulleted_list_item", RichText: "deep"},
		},
	}
	// Parent at indent "", child at "  ", grandchild at "    ", each joined
	// with a blank line.
	want := "- top\n\n  - nested\n\n    - deep"
	if got := FromBlocks(blocks, children); got != want {
		t.Errorf("FromBlocks() nesting = %q, want %q", got, want)
	}
}

// TestFromBlocksHasChildrenNoMap ensures a block flagged HasChildren but absent
// from the map renders on its own without error.
func TestFromBlocksHasChildrenNoMap(t *testing.T) {
	blocks := []notion.NormalizedBlock{
		{ID: "p", Type: "paragraph", RichText: "solo", HasChildren: true},
	}
	if got := FromBlocks(blocks, nil); got != "solo" {
		t.Errorf("FromBlocks() = %q, want %q", got, "solo")
	}
}

// TestFromBlocksIndentAppliesToAllTypes spot-checks that the indent prefix is
// applied across block types when nested.
func TestFromBlocksIndentAppliesToAllTypes(t *testing.T) {
	blocks := []notion.NormalizedBlock{
		{ID: "parent", Type: "toggle", RichText: "group", HasChildren: true},
	}
	children := map[string][]notion.NormalizedBlock{
		"parent": {
			{Type: "code", RichText: "code line", Language: "go"},
			{Type: "divider"},
		},
	}
	want := "> ▶ group\n\n  ```go\ncode line\n  ```\n\n  ---"
	if got := FromBlocks(blocks, children); got != want {
		t.Errorf("FromBlocks() = %q, want %q", got, want)
	}
}

func TestFlattenBlock(t *testing.T) {
	tests := []struct {
		name  string
		block notion.NormalizedBlock
		want  map[string]any
	}{
		{
			"with content",
			notion.NormalizedBlock{ID: "b1", Type: "paragraph", RichText: "hello", HasChildren: true},
			map[string]any{"id": "b1", "type": "paragraph", "has_children": true, "content": "hello"},
		},
		{
			"empty content omits key",
			notion.NormalizedBlock{ID: "b2", Type: "divider", RichText: "", HasChildren: false},
			map[string]any{"id": "b2", "type": "divider", "has_children": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FlattenBlock(tt.block)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FlattenBlock() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestToBlocks(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		want     []map[string]any
	}{
		{"empty string", "", []map[string]any{}},
		{"blank lines only", "\n\n  \n", []map[string]any{}},
		{
			"paragraph",
			"just text",
			[]map[string]any{para("just text")},
		},
		{
			"heading levels",
			"# One\n## Two\n### Three",
			[]map[string]any{
				heading(1, "One"),
				heading(2, "Two"),
				heading(3, "Three"),
			},
		},
		{
			"heading four hashes is paragraph",
			"#### Four",
			[]map[string]any{para("#### Four")},
		},
		{
			"divider dashes",
			"---",
			[]map[string]any{{"type": "divider", "divider": map[string]any{}}},
		},
		{
			"divider stars",
			"***",
			[]map[string]any{{"type": "divider", "divider": map[string]any{}}},
		},
		{
			"todo unchecked",
			"- [ ] task",
			[]map[string]any{todo("task", false)},
		},
		{
			"todo checked lowercase",
			"- [x] done",
			[]map[string]any{todo("done", true)},
		},
		{
			"todo checked uppercase",
			"* [X] done",
			[]map[string]any{todo("done", true)},
		},
		{
			"bulleted dash",
			"- item",
			[]map[string]any{listItem("bulleted_list_item", "item")},
		},
		{
			"bulleted star",
			"* item",
			[]map[string]any{listItem("bulleted_list_item", "item")},
		},
		{
			"numbered",
			"1. first\n2. second",
			[]map[string]any{
				listItem("numbered_list_item", "first"),
				listItem("numbered_list_item", "second"),
			},
		},
		{
			"blockquote",
			"> quoted",
			[]map[string]any{listItem("quote", "quoted")},
		},
		{
			"fenced code with language",
			"```go\nx := 1\ny := 2\n```",
			[]map[string]any{code("x := 1\ny := 2", "go")},
		},
		{
			"fenced code no language",
			"```\nplain\n```",
			[]map[string]any{code("plain", "plain text")},
		},
		{
			"fenced code unterminated",
			"```go\nx := 1",
			[]map[string]any{code("x := 1", "go")},
		},
		{
			"mixed document",
			"# Title\n\nA paragraph.\n\n- bullet\n1. number\n\n> quote",
			[]map[string]any{
				heading(1, "Title"),
				para("A paragraph."),
				listItem("bulleted_list_item", "bullet"),
				listItem("numbered_list_item", "number"),
				listItem("quote", "quote"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToBlocks(tt.markdown)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToBlocks(%q)\n got  = %#v\n want = %#v", tt.markdown, got, tt.want)
			}
		})
	}
}

// Helpers building the expected official-API block payloads.

func rt(content string) []map[string]any {
	return []map[string]any{
		{"type": "text", "text": map[string]any{"content": content}},
	}
}

func para(text string) map[string]any {
	return map[string]any{
		"type":      "paragraph",
		"paragraph": map[string]any{"rich_text": rt(text)},
	}
}

func heading(level int, text string) map[string]any {
	key := "heading_" + string(rune('0'+level))
	return map[string]any{
		"type": key,
		key:    map[string]any{"rich_text": rt(text)},
	}
}

func listItem(blockType, text string) map[string]any {
	return map[string]any{
		"type":    blockType,
		blockType: map[string]any{"rich_text": rt(text)},
	}
}

func todo(text string, checked bool) map[string]any {
	return map[string]any{
		"type": "to_do",
		"to_do": map[string]any{
			"rich_text": rt(text),
			"checked":   checked,
		},
	}
}

func code(content, language string) map[string]any {
	return map[string]any{
		"type": "code",
		"code": map[string]any{
			"rich_text": rt(content),
			"language":  language,
		},
	}
}
