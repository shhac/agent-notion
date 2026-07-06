import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { createV3Client } from "../../notion/client.ts";
import { getV3Session } from "../../lib/config.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { getThreadContent } from "../../notion/v3/ai.ts";

export function registerChatGet(chat: Command): void {
  chat.addCommand(
    defineCommand({
      use: "get <thread-id>",
      short: "Get the content of an AI chat thread",
      args: ExactArgs(1),
      options: {
        raw: { type: "bool", description: "Include raw record data for debugging" },
      },
      action: async ([threadId], opts) => {
        await handleAction(async () => {
          const client = createV3Client();
          const session = getV3Session()!;
          const result = await getThreadContent(client, threadId!, session.space_id);

          if (opts.raw) {
            console.log(JSON.stringify(result, null, 2));
          } else {
            printJson({
              title: result.title,
              messages: result.messages,
            });
          }
        });
      },
    }),
  );
}
