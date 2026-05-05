import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { createV3Client } from "../../notion/client.ts";
import { getV3Session } from "../../lib/config.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { markTranscriptSeen } from "../../notion/v3/ai.ts";

export function registerChatMarkRead(chat: Command): void {
  chat.addCommand(
    defineCommand({
      use: "mark-read <thread-id>",
      short: "Mark a chat thread as read",
      args: ExactArgs(1),
      action: async ([threadId]) => {
        await handleAction(async () => {
          const client = createV3Client();
          const session = getV3Session()!;
          const result = await markTranscriptSeen(
            client,
            session.space_id,
            threadId!,
          );
          printJson(result);
        });
      },
    }),
  );
}
