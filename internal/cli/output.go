package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/notion"
	"github.com/shhac/agent-notion/internal/notion/markdown"
	"github.com/shhac/agent-notion/internal/truncation"
	libcli "github.com/shhac/lib-agent-cli/cli"
	output "github.com/shhac/lib-agent-output"
)

// emitItem writes a single record per the family's get-output contract:
// NDJSON by default (one compact line), the bare pretty object under
// --format json|yaml. Truncation policy is applied first.
func emitItem(g *GlobalFlags, item any) error {
	return libcli.EmitItem(g.stdout, g.Format, applyTruncation(g, item))
}

// applyTruncation shapes an output record per the truncation convention:
// description/body/content fields are capped (default 200, the
// truncation.max_length setting overrides) with {field}Length companions;
// --expand/--full lift the cap. Typed records round-trip through JSON so the
// walker sees plain maps — the TS applied truncation post-serialization too.
func applyTruncation(g *GlobalFlags, item any) any {
	settings := config.ReadSettings()
	maxLength := 0
	if settings.Truncation != nil {
		maxLength = settings.Truncation.MaxLength
	}
	tr := truncation.New(truncation.Options{Expand: g.Expand, Full: g.Full, MaxLength: maxLength})

	switch item.(type) {
	case map[string]any, []any:
		return tr.Apply(item)
	}
	raw, err := json.Marshal(item)
	if err != nil {
		return item
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return item
	}
	return tr.Apply(tree)
}

// printList writes items NDJSON by default — one record per line, then any
// meta entries as {"@key": value} lines — or a single {"data": […], "@key":…}
// envelope under --format json|yaml.
func printList(g *GlobalFlags, items []any, meta map[string]any) error {
	format, err := output.ResolveFormat(g.Format, output.FormatNDJSON)
	if err != nil {
		return err
	}
	truncated := make([]any, len(items))
	for i, item := range items {
		truncated[i] = applyTruncation(g, item)
	}
	prefixed := make(map[string]any, len(meta))
	for key, value := range meta {
		prefixed["@"+key] = value
	}
	return output.WriteList(g.stdout, format, truncated, prefixed, nil)
}

// printPaginated writes a backend page of results: the items, then the
// {"@pagination": …} trailer when more remain.
func printPaginated[T any](g *GlobalFlags, page notion.Paginated[T]) error {
	items := make([]any, len(page.Items))
	for i, item := range page.Items {
		items[i] = item
	}
	var meta map[string]any
	if page.HasMore || page.NextCursor != "" {
		meta = map[string]any{"pagination": output.Pagination{
			HasMore:    page.HasMore,
			NextCursor: page.NextCursor,
		}}
	}
	return printList(g, items, meta)
}

// renderMarkdown renders blocks to markdown, recursively fetching descendant
// blocks so nested content (e.g. a list item inside a callout) is not dropped —
// the shared body of `page get --content` and `block list`. maxDepth bounds how
// many levels of children are fetched below the top level; 0 means unbounded.
func renderMarkdown(ctx context.Context, b notion.Backend, blocks []notion.NormalizedBlock, maxDepth int) (string, error) {
	childMap := map[string][]notion.NormalizedBlock{}

	var descend func(bs []notion.NormalizedBlock, level int) error
	descend = func(bs []notion.NormalizedBlock, level int) error {
		for _, blk := range bs {
			if !blk.HasChildren {
				continue
			}
			if _, done := childMap[blk.ID]; done {
				continue
			}
			children, err := b.GetAllBlocks(ctx, blk.ID)
			if err != nil {
				return err
			}
			childMap[blk.ID] = children.Blocks
			if maxDepth == 0 || level+1 < maxDepth {
				if err := descend(children.Blocks, level+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := descend(blocks, 0); err != nil {
		return "", err
	}
	return markdown.FromBlocks(blocks, childMap), nil
}

// warnf writes a plain warning line to stderr (structured errors stay on the
// error contract; this is for non-fatal notices).
func warnf(g *GlobalFlags, format string, args ...any) {
	_, _ = fmt.Fprintf(g.stderr, "warning: "+format+"\n", args...)
}
