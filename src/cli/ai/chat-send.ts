import type { Command } from "commander";
import { createV3Client } from "../../notion/client.ts";
import { getV3Session, readConfig } from "../../lib/config.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { normalizeId } from "../../lib/ids.ts";
import {
  runInferenceTranscript,
  getAvailableModels,
} from "../../notion/v3/ai.ts";
import type {
  AgentInferenceEvent,
  TitleEvent,
  AiModel,
} from "../../notion/v3/ai-types.ts";

/**
 * Resolve a model name (codename or display name) to its codename.
 * Falls back to `ai.defaultModel` config if no --model flag provided.
 * Returns undefined if no model specified anywhere (let the API pick).
 */
async function resolveModel(
  models: AiModel[],
  modelFlag: string | undefined,
  configDefault: string | undefined,
): Promise<string | undefined> {
  const input = modelFlag ?? configDefault;
  if (!input) return undefined;

  // Exact codename match
  const byCodename = models.find((m) => m.model === input);
  if (byCodename) return byCodename.model;

  // Case-insensitive display name match
  const lower = input.toLowerCase();
  const byDisplayName = models.find(
    (m) => m.modelMessage.toLowerCase() === lower,
  );
  if (byDisplayName) return byDisplayName.model;

  // Partial match on display name
  const byPartial = models.find((m) =>
    m.modelMessage.toLowerCase().includes(lower),
  );
  if (byPartial) return byPartial.model;

  throw new CliError(
    `Unknown model "${input}". Run 'ai model list --raw' to see available model codenames.`,
  );
}

export function registerChatSend(chat: Command): void {
  chat
    .command("send")
    .description("Send a message to Notion AI")
    .argument("<message>", "Message to send")
    .option("--thread <thread-id>", "Continue an existing chat thread")
    .option("--model <model>", "Model codename or display name")
    .option("--page <page-id>", "Page context for the conversation")
    .option("--no-search", "Disable workspace and web search")
    .option("--stream", "Stream response text to stderr as it arrives")
    .action(
      async (
        message: string,
        opts: {
          thread?: string;
          model?: string;
          page?: string;
          search: boolean;
          stream?: boolean;
        },
      ) => {
        await handleAction(async () => {
          const client = createV3Client();
          const session = getV3Session()!;
          const config = readConfig();

          // Resolve model: --model flag > config ai.defaultModel > API default
          const configDefault = config.settings?.ai?.default_model;
          let modelCodename: string | undefined;
          if (opts.model || configDefault) {
            const models = await getAvailableModels(client, session.space_id);
            modelCodename = await resolveModel(
              models,
              opts.model,
              configDefault,
            );
          }

          // Generate threadId so we can return it in output
          const threadId = opts.thread ?? crypto.randomUUID();

          const events = await runInferenceTranscript(client, {
            message,
            model: modelCodename,
            threadId,
            pageId: opts.page ? normalizeId(opts.page) : undefined,
            noSearch: !opts.search,
            user: {
              id: session.user_id,
              name: session.user_name,
              email: session.user_email,
            },
            space: {
              id: session.space_id,
              name: session.space_name,
            },
          });

          let lastContent = "";
          let streamedLength = 0;
          let title: string | undefined;
          let model: string | undefined;
          let inputTokens: number | undefined;
          let outputTokens: number | undefined;
          let cachedTokens: number | undefined;

          for await (const event of events) {
            if (event.type === "agent-inference") {
              const ae = event as AgentInferenceEvent;
              const content = ae.value?.[0]?.content ?? "";

              if (opts.stream && content.length > streamedLength) {
                process.stderr.write(content.slice(streamedLength));
                streamedLength = content.length;
              }

              lastContent = content;

              if (ae.finishedAt) {
                model = ae.model;
                inputTokens = ae.inputTokens;
                outputTokens = ae.outputTokens;
                cachedTokens = ae.cachedTokensRead;
              }
            } else if (event.type === "title") {
              title = (event as TitleEvent).value;
            }
          }

          if (opts.stream && streamedLength > 0) {
            process.stderr.write("\n");
          }

          printJson({
            threadId,
            title,
            response: lastContent,
            model,
            tokens:
              inputTokens !== undefined
                ? {
                    input: inputTokens,
                    output: outputTokens,
                    cached: cachedTokens,
                  }
                : undefined,
          });
        });
      },
    );
}
