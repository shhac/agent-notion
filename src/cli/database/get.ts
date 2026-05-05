import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerGet(database: Command): void {
  database.addCommand(
    defineCommand({
      use: "get <database-id>",
      short: "Get database metadata and property definitions",
      args: ExactArgs(1),
      action: async ([rawDatabaseId]) => {
        const databaseId = normalizeId(rawDatabaseId!);
        await handleAction(async () => {
          const db = await withBackend((backend) => backend.getDatabase(databaseId));
          printJson(db);
        });
      },
    }),
  );
}
