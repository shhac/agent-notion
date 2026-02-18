import type { Command } from "commander";
import { createV3Client } from "../../notion/client.ts";
import { getV3Session, readConfig } from "../../lib/config.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { normalizeId } from "../../lib/ids.ts";
import {
  runInferenceTranscript,
  getAvailableModels,
  resolveModel,
  processInferenceStream,
} from "../../notion/v3/ai.ts";

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
    .option("--debug", "Dump raw NDJSON events to stderr")
    .action(
      async (
        message: string,
        opts: {
          thread?: string;
          model?: string;
          page?: string;
          search: boolean;
          stream?: boolean;
          debug?: boolean;
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
          const isNewThread = !opts.thread;
          const threadId = opts.thread ?? crypto.randomUUID();

          const events = await runInferenceTranscript(client, {
            message,
            model: modelCodename,
            threadId,
            isNewThread,
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
          }, { debug: opts.debug });

          let didStream = false;
          const result = await processInferenceStream(
            events,
            opts.stream
              ? (text) => {
                  didStream = true;
                  process.stderr.write(text);
                }
              : undefined,
          );

          if (didStream) {
            process.stderr.write("\n");
          }

          printJson({
            threadId,
            ...result,
          });
        });
      },
    );
}
