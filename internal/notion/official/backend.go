// Official backend — implements notion.Backend over the official REST Client.
// The Client's method signatures already match the interface, so Backend just
// embeds it; only the backend identity (Kind) and the four operations the
// official REST API cannot perform (guidance errors) live here.

package official

import (
	"context"

	"github.com/shhac/agent-notion/internal/notion"
	output "github.com/shhac/lib-agent-output"
)

// Backend implements notion.Backend using the official REST API. Every data
// operation is promoted from the embedded Client; the methods below are the
// only ones that differ.
type Backend struct {
	Client
}

var _ notion.Backend = (*Backend)(nil)

// NewBackend wraps a REST Client as a notion.Backend.
func NewBackend(client Client) *Backend { return &Backend{Client: client} }

// Kind reports the backend kind.
func (b *Backend) Kind() string { return "official" }

// v3ImportHint points users at the command that enables the v3 backend, which
// implements the operations the official REST API cannot.
const v3ImportHint = "Run 'agent-notion auth import-desktop' to set up the v3 backend."

// v3RequiredErr builds the guidance error for a v3-only operation.
func v3RequiredErr(msg string) error {
	return output.New(msg, output.FixableByHuman).WithHint(v3ImportHint)
}

// ArchivePage is unsupported on the official REST API (real Archive is a v3
// capability, distinct from Trash).
func (b *Backend) ArchivePage(_ context.Context, _ string) (notion.PageArchiveResult, error) {
	return notion.PageArchiveResult{}, v3RequiredErr(
		"Real Archive (distinct from Trash) requires the v3 backend. To move a page to Trash, use 'page trash' instead.")
}

// UnarchivePage is unsupported on the official REST API (real Archive is a v3
// capability, distinct from Trash).
func (b *Backend) UnarchivePage(_ context.Context, _ string) (notion.PageArchiveResult, error) {
	return notion.PageArchiveResult{}, v3RequiredErr(
		"Unarchive (real Archive, distinct from Trash) requires the v3 backend. To restore a page from Trash, use 'page restore' instead.")
}

// MoveBlock is unsupported on the official REST API (block reordering is a v3
// capability).
func (b *Backend) MoveBlock(_ context.Context, _ notion.MoveBlockParams) (notion.BlockMoveResult, error) {
	return notion.BlockMoveResult{}, v3RequiredErr("Block reordering requires the v3 backend.")
}

// AddInlineComment is unsupported on the official REST API (inline comments
// anchored to text are a v3 capability).
func (b *Backend) AddInlineComment(_ context.Context, _ notion.AddInlineCommentParams) (notion.CommentCreateResult, error) {
	return notion.CommentCreateResult{}, v3RequiredErr("Inline comments require the v3 backend.")
}
