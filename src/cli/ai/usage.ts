import type { Command } from "commander";

const USAGE_TEXT = `agent-notion ai — Notion AI chat and models (v3 desktop session required)

SUBCOMMANDS:
  ai model list [--raw]                              List available AI models
  ai chat list [--limit <n>]                         List recent chat threads
  ai chat get <thread-id> [--raw]                    Get thread content (messages)
  ai chat send <message> [options]                   Send message to Notion AI
  ai chat mark-read <thread-id>                      Mark a thread as read

CHAT GET OPTIONS:
  --raw                  Include raw record data for debugging

CHAT SEND OPTIONS:
  --thread <thread-id>   Continue an existing thread (omit to start new)
  --model <model>        Model codename or display name (see ai model list --raw)
  --page <page-id>       Set page context for the conversation
  --no-search            Disable workspace and web search
  --stream               Stream response text to stderr as it arrives

MODEL RESOLUTION:
  --model flag > config ai.defaultModel > API default
  Accepts codenames (e.g., "oatmeal-cookie") or display names (e.g., "GPT-5.2").
  Set a default: config set ai.defaultModel <codename>

OUTPUT:
  model list:       { models: [{ name, family, tier }] }
  model list --raw: { models: [<full model objects with codename>] }
  chat list:        { items: [{ id, title, created_at, updated_at, ... }], unreadThreadIds, hasMore }
  chat get:         { title, messages: [{ id, role, content, createdAt }] }
  chat send:        { threadId, title, response, model, tokens: { input, output, cached } }
  chat mark-read:   { ok: true }

  With --stream, AI response text is written incrementally to stderr.
  JSON result always goes to stdout regardless of --stream.

EXAMPLES:
  ai model list                                      List active models
  ai model list --raw                                Include codenames and disabled models
  ai chat list --limit 10                            Recent 10 threads
  ai chat get <thread-id>                            Read thread messages
  ai chat get <thread-id> --raw                      With raw record data
  ai chat send "Summarize my recent projects"        New conversation
  ai chat send "Tell me more" --thread <id>          Continue thread
  ai chat send "Explain this page" --page <id>       With page context
  ai chat send "Quick question" --stream             Stream response
  ai chat send "Hello" --model "GPT-5.2"             Use specific model
  ai chat mark-read <thread-id>                      Mark as read
`;

export function registerUsage(ai: Command): void {
  ai
    .command("usage")
    .description("Print detailed AI documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
