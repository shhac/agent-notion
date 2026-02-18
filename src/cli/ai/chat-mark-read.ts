import type { Command } from "commander";
import { createV3Client } from "../../notion/client.ts";
import { getV3Session } from "../../lib/config.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { markTranscriptSeen } from "../../notion/v3/ai.ts";

export function registerChatMarkRead(chat: Command): void {
  chat
    .command("mark-read")
    .description("Mark a chat thread as read")
    .argument("<thread-id>", "Thread UUID")
    .action(async (threadId: string) => {
      await handleAction(async () => {
        const client = createV3Client();
        const session = getV3Session()!;
        const result = await markTranscriptSeen(
          client,
          session.space_id,
          threadId,
        );
        printJson(result);
      });
    });
}
