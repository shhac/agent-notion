import { defineCommand, type Command } from "../../lib/cli.ts";
import { createV3Client } from "../../notion/client.ts";
import { getV3Session } from "../../lib/config.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { getInferenceTranscripts } from "../../notion/v3/ai.ts";

export function registerChatList(chat: Command): void {
  chat.addCommand(
    defineCommand({
      use: "list",
      short: "List recent AI chat threads",
      options: {
        limit: { type: "string", default: "20", description: "Max results" },
      },
      action: async (_args, opts) => {
        await handleAction(async () => {
          const client = createV3Client();
          const session = getV3Session()!;
          const limit = parseInt(opts.limit, 10);

          const result = await getInferenceTranscripts(
            client,
            session.space_id,
            limit,
          );

          printJson({
            items: result.transcripts,
            unreadThreadIds: result.unreadThreadIds,
            hasMore: result.hasMore,
          });
        });
      },
    }),
  );
}
