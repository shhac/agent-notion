import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";

export function registerMe(user: Command): void {
  user
    .command("me")
    .description("Get the bot user (integration) identity")
    .action(async () => {
      await handleAction(async () => {
        const result = await withBackend((backend) =>
          backend.getMe(),
        );

        printJson(result);
      });
    });
}
