import type { Command } from "commander";
import { createV3Client } from "../../notion/client.ts";
import { getV3Session } from "../../lib/config.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { getThreadContent } from "../../notion/v3/ai.ts";

export function registerChatGet(chat: Command): void {
  chat
    .command("get")
    .description("Get the content of an AI chat thread")
    .argument("<thread-id>", "Thread UUID")
    .option("--raw", "Include raw record data for debugging")
    .action(async (threadId: string, opts: { raw?: boolean }) => {
      await handleAction(async () => {
        const client = createV3Client();
        const session = getV3Session()!;
        const result = await getThreadContent(client, threadId, session.space_id);

        if (opts.raw) {
          // Bypass pruneEmpty so we can see the actual record structure
          console.log(JSON.stringify(result, null, 2));
        } else {
          printJson({
            title: result.title,
            messages: result.messages,
          });
        }
      });
    });
}
