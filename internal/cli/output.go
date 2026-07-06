package cli

import (
	"fmt"

	"github.com/shhac/agent-notion/internal/notion"
	libcli "github.com/shhac/lib-agent-cli/cli"
	output "github.com/shhac/lib-agent-output"
)

// emitItem writes a single record per the family's get-output contract:
// NDJSON by default (one compact line), the bare pretty object under
// --format json|yaml.
func emitItem(g *GlobalFlags, item any) error {
	return libcli.EmitItem(g.stdout, g.Format, item)
}

// printList writes items NDJSON by default — one record per line, then any
// meta entries as {"@key": value} lines — or a single {"data": […], "@key":…}
// envelope under --format json|yaml.
func printList(g *GlobalFlags, items []any, meta map[string]any) error {
	format, err := output.ResolveFormat(g.Format, output.FormatNDJSON)
	if err != nil {
		return err
	}
	prefixed := make(map[string]any, len(meta))
	for key, value := range meta {
		prefixed["@"+key] = value
	}
	return output.WriteList(g.stdout, format, items, prefixed, nil)
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

// warnf writes a plain warning line to stderr (structured errors stay on the
// error contract; this is for non-fatal notices).
func warnf(g *GlobalFlags, format string, args ...any) {
	_, _ = fmt.Fprintf(g.stderr, "warning: "+format+"\n", args...)
}
