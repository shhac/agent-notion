package cli

import (
	"fmt"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/ids"
	v3 "github.com/shhac/agent-notion/internal/notion/v3"
	"github.com/spf13/cobra"
)

// registerAI wires the `ai` command group: `ai model list` and the `ai chat`
// subcommands. All leaves are v3-only.
func registerAI(root *cobra.Command, g *GlobalFlags) {
	ai := &cobra.Command{
		Use:   "ai",
		Short: "Notion AI chat and models (v3 desktop session required)",
	}

	model := &cobra.Command{Use: "model", Short: "AI model operations"}
	model.AddCommand(aiModelListCmd(g))
	ai.AddCommand(model)

	chat := &cobra.Command{Use: "chat", Short: "AI chat conversations"}
	chat.AddCommand(aiChatListCmd(g), aiChatGetCmd(g), aiChatSendCmd(g), aiChatMarkReadCmd(g))
	ai.AddCommand(chat)

	addDomainUsage("ai", aiUsageText)
	root.AddCommand(ai)
}

// modelSummary is the compact model listing (non-raw).
type modelSummary struct {
	Name   string `json:"name"`
	Family string `json:"family"`
	Tier   string `json:"tier"`
}

func aiModelListCmd(g *GlobalFlags) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available AI models",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			models, err := withV3Client(g, func(c *v3.Client, sess *config.V3Session) ([]v3.AIModel, error) {
				return v3.GetAvailableModels(cmd.Context(), c, sess.SpaceID)
			})
			if err != nil {
				return err
			}

			if raw {
				items := make([]any, len(models))
				for i, m := range models {
					items[i] = m
				}
				return printList(g, items, nil)
			}

			items := []any{}
			for _, m := range models {
				if m.IsDisabled {
					continue
				}
				items = append(items, modelSummary{Name: m.ModelMessage, Family: m.ModelFamily, Tier: m.DisplayGroup})
			}
			return printList(g, items, nil)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Return full model objects including codename")
	return cmd
}

func aiChatListCmd(g *GlobalFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent AI chat threads",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := withV3Client(g, func(c *v3.Client, sess *config.V3Session) (v3.TranscriptList, error) {
				return v3.GetInferenceTranscripts(cmd.Context(), c, sess.SpaceID, limit)
			})
			if err != nil {
				return err
			}
			items := make([]any, len(result.Transcripts))
			for i, tr := range result.Transcripts {
				items[i] = tr
			}
			meta := map[string]any{"meta": map[string]any{
				"unread_thread_ids": result.UnreadThreadIDs,
				"has_more":          result.HasMore,
			}}
			return printList(g, items, meta)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}

func aiChatGetCmd(g *GlobalFlags) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "get <thread-id>",
		Short: "Get the content of an AI chat thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := withV3Client(g, func(c *v3.Client, sess *config.V3Session) (v3.ThreadContent, error) {
				return v3.GetThreadContent(cmd.Context(), c, args[0], sess.SpaceID)
			})
			if err != nil {
				return err
			}
			if raw {
				return emitItem(g, result)
			}
			return emitItem(g, map[string]any{"title": result.Title, "messages": result.Messages})
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Include raw record data for debugging")
	return cmd
}

func aiChatMarkReadCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "mark-read <thread-id>",
		Short: "Mark a chat thread as read",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ok, err := withV3Client(g, func(c *v3.Client, sess *config.V3Session) (bool, error) {
				return v3.MarkTranscriptSeen(cmd.Context(), c, sess.SpaceID, args[0])
			})
			if err != nil {
				return err
			}
			return emitItem(g, map[string]any{"ok": ok})
		},
	}
}

// chatSendOutput is the final structured record: the thread ID plus the
// accumulated chat result.
type chatSendOutput struct {
	ThreadID string `json:"thread_id"`
	v3.ChatResult
}

func aiChatSendCmd(g *GlobalFlags) *cobra.Command {
	var (
		thread   string
		model    string
		page     string
		noSearch bool
		stream   bool
		readOnly bool
	)
	cmd := &cobra.Command{
		Use:   "send <message>",
		Short: "Send a message to Notion AI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			isNew := thread == ""
			threadID := thread
			if threadID == "" {
				threadID = v3.NewUUID()
			}
			pageID := ""
			if page != "" {
				pageID = ids.Normalize(page)
			}
			configDefault := ""
			if ai := config.ReadSettings().AI; ai != nil {
				configDefault = ai.DefaultModel
			}

			var onChunk func(string)
			if stream {
				onChunk = func(text string) { _, _ = fmt.Fprint(g.stderr, text) }
			}

			result, err := withV3Client(g, func(c *v3.Client, sess *config.V3Session) (v3.ChatResult, error) {
				modelCodename := ""
				if model != "" || configDefault != "" {
					models, err := v3.GetAvailableModels(ctx, c, sess.SpaceID)
					if err != nil {
						return v3.ChatResult{}, err
					}
					if modelCodename, err = v3.ResolveModel(models, model, configDefault); err != nil {
						return v3.ChatResult{}, err
					}
				}

				params := v3.RunInferenceParams{
					Message:     args[0],
					Model:       modelCodename,
					ThreadID:    threadID,
					IsNewThread: &isNew,
					PageID:      pageID,
					NoSearch:    noSearch,
					ReadOnly:    readOnly,
					User:        v3.AIUser{ID: sess.UserID, Name: sess.UserName, Email: sess.UserEmail},
					Space:       v3.AISpace{ID: sess.SpaceID, Name: sess.SpaceName, ViewID: sess.SpaceViewID},
				}

				if g.Debug {
					// Buffered path: dump each raw event, then accumulate.
					var events []v3.NdjsonEvent
					if err := v3.RunInferenceStream(ctx, c, params, func(e v3.NdjsonEvent) error {
						_, _ = fmt.Fprintf(g.stderr, "[debug] %s\n", e.Raw)
						events = append(events, e)
						return nil
					}); err != nil {
						return v3.ChatResult{}, err
					}
					return v3.ProcessInferenceStream(events, onChunk), nil
				}
				return v3.RunInferenceChat(ctx, c, params, onChunk)
			})
			if err != nil {
				return err
			}

			if stream {
				_, _ = fmt.Fprintln(g.stderr)
			}
			return emitItem(g, chatSendOutput{ThreadID: threadID, ChatResult: result})
		},
	}
	cmd.Flags().StringVar(&thread, "thread", "", "Continue an existing chat thread")
	cmd.Flags().StringVar(&model, "model", "", "Model codename or display name")
	cmd.Flags().StringVar(&page, "page", "", "Page context for the conversation")
	cmd.Flags().BoolVar(&noSearch, "no-search", false, "Disable workspace and web search")
	cmd.Flags().BoolVar(&stream, "stream", false, "Stream response text to stderr as it arrives")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Ask/answer only: request Notion disable the AI's document-editing tools")
	return cmd
}

const aiUsageText = `agent-notion ai — Notion AI chat and models (v3 desktop session required)

SUBCOMMANDS:
  ai model list [--raw]                    List available AI models
  ai chat list [--limit <n>]               List recent chat threads
  ai chat get <thread-id> [--raw]          Get thread content (messages)
  ai chat send <message> [options]         Send message to Notion AI
  ai chat mark-read <thread-id>            Mark a thread as read

CHAT SEND OPTIONS:
  --thread <thread-id>   Continue an existing thread (omit to start new)
  --model <model>        Model codename or display name (see 'ai model list --raw')
  --page <page-id>       Set page context for the conversation
  --no-search            Disable workspace and web search
  --read-only            Ask/answer only: request Notion disable the AI's editing tools
  --stream               Stream response text to stderr as it arrives
  --debug                (global flag) also dumps raw NDJSON events to stderr

READ-ONLY:
  By default the AI has its full document-editing tools (matching Notion),
  so a prompt can modify a page. Pass --read-only to request ask/answer mode:
  it asks Notion's backend to disable those tools. Note this is a server-side
  request, not a client-enforced guarantee.

MODEL RESOLUTION:
  --model flag > config ai.default_model > API default
  Accepts codenames (e.g. "oatmeal-cookie") or display names (e.g. "GPT-5.2").
  Set a default: config set ai.default_model <codename>

OUTPUT:
  model list:       one NDJSON record per model {name, family, tier}
  model list --raw: one NDJSON record per full model object (with codename)
  chat list:        one NDJSON record per thread, then {"@meta": {unread_thread_ids, has_more}}
  chat get:         { title, messages: [{ id, role, content, created_at }] }
  chat send:        { thread_id, response, title, model, tokens: { input, output, cached } }
  chat mark-read:   { ok: true }

  With --stream, AI response text is written incrementally to stderr.
  The JSON result always goes to stdout regardless of --stream.

EXAMPLES:
  ai model list                                   List active models
  ai model list --raw                             Include codenames and disabled models
  ai chat list --limit 10                         Recent 10 threads
  ai chat get <thread-id>                         Read thread messages
  ai chat send "Summarize my projects"            New conversation
  ai chat send "Tell me more" --thread <id>       Continue thread
  ai chat send "Explain this page" --page <id>    With page context
  ai chat send "Summarize, don't edit" --page <id> --read-only   Ask/answer only
  ai chat send "Quick question" --stream          Stream response
  ai chat send "Hello" --model "GPT-5.2"          Use a specific model
  ai chat mark-read <thread-id>                   Mark as read`
