import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerGet(database: Command): void {
  database
    .command("get")
    .description("Get database metadata and property definitions")
    .argument("<database-id>", "Database UUID")
    .action(async (rawDatabaseId: string) => {
      const databaseId = normalizeId(rawDatabaseId);
      await handleAction(async () => {
        const db = await withBackend((backend) =>
          backend.getDatabase(databaseId),
        );

        printJson(db);
      });
    });
}
