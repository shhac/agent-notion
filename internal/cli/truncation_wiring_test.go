package cli

import (
	"strings"
	"testing"
)

// seedLongContentPage queues a page whose single paragraph renders to
// markdown well past the 200-char default cap.
func seedLongContentPage(t *testing.T) string {
	t.Helper()
	s, url := newMockServer(t)
	long := strings.Repeat("a", 300)
	s.HandleBody("GET /v1/pages/page-1", map[string]any{
		"object": "page", "id": "page-1",
		"url":        "https://www.notion.so/page-1",
		"properties": map[string]any{},
	})
	s.HandleBody("GET /v1/blocks/page-1/children", map[string]any{
		"object": "list",
		"results": []any{map[string]any{
			"object": "block", "id": "b1", "type": "paragraph",
			"has_children": false,
			"paragraph": map[string]any{
				"rich_text": []any{map[string]any{"plain_text": long}},
			},
		}},
		"has_more": false,
	})
	return url
}

func TestOutputTruncatesContentByDefault(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	url := seedLongContentPage(t)

	out, _, err := runCLI(t, "", "--base-url", url, "page", "get", "page-1", "--content")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	content, _ := item["content"].(string)
	if len([]rune(content)) != 201 { // 200 + ellipsis
		t.Errorf("content length = %d, want truncated to 201 runes", len([]rune(content)))
	}
	if item["contentLength"] != float64(300) {
		t.Errorf("contentLength = %v, want 300", item["contentLength"])
	}
}

func TestFullFlagExpandsContent(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	url := seedLongContentPage(t)

	out, _, err := runCLI(t, "", "--base-url", url, "--full", "page", "get", "page-1", "--content")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if content, _ := item["content"].(string); len([]rune(content)) != 300 {
		t.Errorf("--full content length = %d, want 300", len([]rune(content)))
	}
}

func TestTruncationMaxLengthSetting(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	url := seedLongContentPage(t)

	if _, _, err := runCLI(t, "", "config", "set", "truncation.max_length", "50"); err != nil {
		t.Fatal(err)
	}
	out, _, err := runCLI(t, "", "--base-url", url, "page", "get", "page-1", "--content")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if content, _ := item["content"].(string); len([]rune(content)) != 51 {
		t.Errorf("content length = %d, want 51 with max_length 50", len([]rune(content)))
	}
}
