package cli

import (
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
)

func TestCommentList(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/comments", map[string]any{
		"has_more":    false,
		"next_cursor": nil,
		"results": []any{map[string]any{
			"id":         "c1",
			"rich_text":  []any{map[string]any{"plain_text": "hello"}},
			"created_by": map[string]any{"id": "u1", "name": "Al"},
		}},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "comment", "list", "pg1")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["body"] != "hello" {
		t.Errorf("comment list output = %v", item)
	}
}

func TestCommentPage(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("POST /v1/comments", map[string]any{
		"id": "c2", "created_time": "2024-01-01T00:00:00Z",
		"rich_text": []any{map[string]any{"plain_text": "the body"}},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "comment", "page", "pg1", "the body")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["id"] != "c2" || item["body"] != "the body" {
		t.Errorf("comment page output = %v", item)
	}
}

// TestCommentInline drives the v3-only inline-comment path: sync the block,
// inject the decoration, saveTransactions, and echo back the anchor text.
func TestCommentInline(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("syncRecordValuesMain", mocknotion.RecordMapBody(map[string]map[string]any{
		"block": {"block-1": mocknotion.BlockEntity("block-1", "text", map[string]any{
			"properties": map[string]any{"title": []any{[]any{"Hello world"}}},
		})},
	}))
	s.Handle("saveTransactions", mocknotion.Response{Body: map[string]any{"recordMap": map[string]any{}}})

	out, _, err := runCLI(t, "", "--base-url", url, "comment", "inline", "block-1", "Fix this", "--text", "world")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["body"] != "Fix this" || item["anchor_text"] != "world" {
		t.Errorf("inline output = %v", item)
	}
	if item["id"] == nil || item["discussion_id"] == nil {
		t.Errorf("inline output missing ids: %v", item)
	}
	if len(s.CallsFor("saveTransactions")) != 1 {
		t.Errorf("saveTransactions calls = %d", len(s.CallsFor("saveTransactions")))
	}
}

func TestCommentInlineOccurrenceValidation(t *testing.T) {
	isolateState(t)
	seedV3Session(t)

	_, _, err := runCLI(t, "", "comment", "inline", "block-1", "note", "--text", "x", "--occurrence", "0")
	if err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Errorf("err = %v", err)
	}
}
