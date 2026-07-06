package cli

import (
	"github.com/shhac/agent-notion/internal/errors"
	"github.com/shhac/agent-notion/internal/ids"
	"github.com/shhac/agent-notion/internal/notion"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// registerComment wires the `comment` command group (list, page, inline) plus
// its usage card.
func registerComment(root *cobra.Command, g *GlobalFlags) {
	comment := &cobra.Command{
		Use:   "comment",
		Short: "Comment operations",
	}
	comment.AddCommand(commentListCmd(g), commentPageCmd(g), commentInlineCmd(g))
	addDomainUsage("comment", commentUsageText)
	root.AddCommand(comment)
}

func commentListCmd(g *GlobalFlags) *cobra.Command {
	var (
		limit  int
		cursor string
	)
	cmd := &cobra.Command{
		Use:   "list <page-id>",
		Short: "List comments on a page or block",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.Paginated[notion.CommentItem], error) {
				return b.ListComments(ctx, notion.ListCommentsParams{PageID: pageID, Limit: g.pageSize(limit), Cursor: cursor})
			})
			if err != nil {
				return errors.Classify(err)
			}
			return printPaginated(g, result)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results (default: page_size setting, else 50)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous page")
	return cmd
}

func commentPageCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "page <page-id> <body>",
		Short: "Add a page-level comment",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID := ids.Normalize(args[0])
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.CommentCreateResult, error) {
				return b.AddComment(ctx, notion.AddCommentParams{PageID: pageID, Body: args[1]})
			})
			if err != nil {
				return errors.Classify(err)
			}
			return emitItem(g, result)
		},
	}
}

// inlineCommentOutput is a comment-create result plus the anchor text the CLI
// echoes back (mirrors the TS `{ ...result, anchorText }`).
type inlineCommentOutput struct {
	notion.CommentCreateResult
	AnchorText string `json:"anchor_text"`
}

func commentInlineCmd(g *GlobalFlags) *cobra.Command {
	var (
		text       string
		occurrence int
	)
	cmd := &cobra.Command{
		Use:   "inline <block-id> <body>",
		Short: "Add an inline comment anchored to specific text in a block",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if occurrence < 1 {
				return output.New("--occurrence must be a positive integer", output.FixableByAgent)
			}
			blockID := ids.Normalize(args[0])
			ctx := cmd.Context()
			result, err := withBackend(ctx, g, func(b notion.Backend) (notion.CommentCreateResult, error) {
				return b.AddInlineComment(ctx, notion.AddInlineCommentParams{
					BlockID:    blockID,
					Body:       args[1],
					Text:       text,
					Occurrence: occurrence,
				})
			})
			if err != nil {
				return errors.Classify(err)
			}
			return emitItem(g, inlineCommentOutput{CommentCreateResult: result, AnchorText: text})
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "Text substring to anchor the comment to")
	cmd.Flags().IntVar(&occurrence, "occurrence", 1, "Which occurrence if text appears multiple times (default: 1)")
	_ = cmd.MarkFlagRequired("text")
	return cmd
}

const commentUsageText = `agent-notion comment — Page and inline comments

SUBCOMMANDS:
  comment list <page-id> [--limit <n>] [--cursor <cursor>]
                                                   List comments on a page or block
  comment page <page-id> <body>                    Add a page-level comment
  comment inline <block-id> <body> --text <target> [--occurrence <n>]
                                                   Add an inline comment on specific text

INLINE COMMENTS:
  Inline comments are anchored to a specific text substring within a block.
  --text is required and specifies the target text to annotate.
  --occurrence selects which match when the text appears multiple times (default: 1).

  Example:
    comment inline <block-id> "Great point!" --text "hello"
    # Adds a comment on the word "hello" within the block

    comment inline <block-id> "Second one" --text "the" --occurrence 2
    # Adds a comment on the second occurrence of "the"

LIST OUTPUT:
  { "items": [{ id, body, author: { id, name }, created_at }], "pagination"?: ... }

PAGE OUTPUT:
  { id, discussion_id, body, created_at }

INLINE OUTPUT:
  { id, discussion_id, body, created_at, anchor_text }

LIMITATIONS:
  Inline comments require the v3 backend (desktop session).
  Discussion threads are not supported — all comments are top-level.
  The API does not support editing or deleting comments.`
