import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";

export function registerMe(user: Command): void {
  user
    .command("me")
    .description("Get the bot user (integration) identity")
    .action(async () => {
      await handleAction(async () => {
        const result = await withAutoRefresh((client) => client.users.me({}));

        const u = result as Record<string, unknown>;
        const bot = u.bot as Record<string, unknown> | undefined;
        const workspace = bot?.workspace_name as string | undefined;

        printJson({
          id: u.id,
          name: u.name,
          type: u.type,
          workspaceName: workspace,
        });
      });
    });
}
