import type { Command } from "commander";
import { createV3Client } from "../../notion/client.ts";
import { getV3Session } from "../../lib/config.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { getInferenceTranscripts } from "../../notion/v3/ai.ts";

export function registerChatList(chat: Command): void {
  chat
    .command("list")
    .description("List recent AI chat threads")
    .option("--limit <n>", "Max results", "20")
    .action(async (opts: { limit: string }) => {
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
    });
}
