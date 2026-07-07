package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/notion"
)

// childFetchBackend is a minimal notion.Backend that only answers GetAllBlocks
// from a fixed parent→children map; every other method is unused by
// renderMarkdown. It also counts fetches so tests can assert descent bounds, and
// can be told to fail on a specific block ID to exercise error propagation.
type childFetchBackend struct {
	notion.Backend
	children map[string][]notion.NormalizedBlock
	failOn   string
	fetches  int
}

func (f *childFetchBackend) GetAllBlocks(_ context.Context, id string) (notion.BlockListResult, error) {
	f.fetches++
	if id == f.failOn {
		return notion.BlockListResult{}, errors.New("boom")
	}
	return notion.BlockListResult{Blocks: f.children[id]}, nil
}

// TestRenderMarkdownRecursesDeep pins the fix: a grandchild block (page →
// callout → list item → list item) is fetched and rendered, not dropped.
func TestRenderMarkdownRecursesDeep(t *testing.T) {
	fb := &childFetchBackend{children: map[string][]notion.NormalizedBlock{
		"C": {{ID: "A", Type: "bulleted_list_item", RichText: "item A", HasChildren: true}},
		"A": {{ID: "B", Type: "bulleted_list_item", RichText: "item B"}},
	}}
	top := []notion.NormalizedBlock{{ID: "C", Type: "callout", RichText: "note", Emoji: "📢", HasChildren: true}}

	md, err := renderMarkdown(context.Background(), fb, top, 0) // 0 = unbounded
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "item B") {
		t.Errorf("grandchild not rendered:\n%s", md)
	}
	if !strings.Contains(md, "item A") {
		t.Errorf("child not rendered:\n%s", md)
	}
}

// TestRenderMarkdownDepthBound pins that max_depth stops the descent: with a
// bound of 1, only the top level's direct children are fetched.
func TestRenderMarkdownDepthBound(t *testing.T) {
	fb := &childFetchBackend{children: map[string][]notion.NormalizedBlock{
		"C": {{ID: "A", Type: "bulleted_list_item", RichText: "item A", HasChildren: true}},
		"A": {{ID: "B", Type: "bulleted_list_item", RichText: "item B"}},
	}}
	top := []notion.NormalizedBlock{{ID: "C", Type: "callout", RichText: "note", HasChildren: true}}

	md, err := renderMarkdown(context.Background(), fb, top, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "item A") {
		t.Errorf("direct child should still render at depth 1:\n%s", md)
	}
	if strings.Contains(md, "item B") {
		t.Errorf("grandchild should be omitted at max_depth=1:\n%s", md)
	}
	if fb.fetches != 1 {
		t.Errorf("max_depth=1 should fetch exactly the top level's children (1 call), got %d", fb.fetches)
	}
}

// TestRenderMarkdownPropagatesFetchError pins that an error from a deep child
// fetch aborts and surfaces rather than being swallowed or partially rendered.
func TestRenderMarkdownPropagatesFetchError(t *testing.T) {
	fb := &childFetchBackend{
		failOn: "A", // grandchild fetch fails
		children: map[string][]notion.NormalizedBlock{
			"C": {{ID: "A", Type: "bulleted_list_item", RichText: "item A", HasChildren: true}},
		},
	}
	top := []notion.NormalizedBlock{{ID: "C", Type: "callout", RichText: "note", HasChildren: true}}

	md, err := renderMarkdown(context.Background(), fb, top, 0)
	if err == nil {
		t.Fatal("expected the deep fetch error to propagate")
	}
	if md != "" {
		t.Errorf("expected empty output on error, got %q", md)
	}
}
