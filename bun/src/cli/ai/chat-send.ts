import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
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
  chat.addCommand(
    defineCommand({
      use: "send <message>",
      short: "Send a message to Notion AI",
      args: ExactArgs(1),
      options: {
        thread: { type: "string", description: "Continue an existing chat thread" },
        model: { type: "string", description: "Model codename or display name" },
        page: { type: "string", description: "Page context for the conversation" },
        // Boolean defaults to true; user passes --no-search to disable.
        search: {
          type: "bool",
          default: true,
          negatable: true,
          description: "Enable workspace and web search (use --no-search to disable)",
        },
        stream: {
          type: "bool",
          description: "Stream response text to stderr as it arrives",
        },
        debug: { type: "bool", description: "Dump raw NDJSON events to stderr" },
      },
      action: async ([message], opts) => {
        await handleAction(async () => {
          const client = createV3Client();
          const session = getV3Session()!;
          const config = readConfig();

          const configDefault = config.settings?.ai?.default_model;
          let modelCodename: string | undefined;
          if (opts.model || configDefault) {
            const models = await getAvailableModels(client, session.space_id);
            modelCodename = await resolveModel(models, opts.model, configDefault);
          }

          const isNewThread = !opts.thread;
          const threadId = opts.thread ?? crypto.randomUUID();

          const events = await runInferenceTranscript(
            client,
            {
              message: message!,
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
                viewId: session.space_view_id,
              },
            },
            { debug: opts.debug },
          );

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
    }),
  );
}
