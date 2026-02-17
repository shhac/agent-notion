import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerSchema(database: Command): void {
  database
    .command("schema")
    .description("Get property definitions (compact, LLM-optimized)")
    .argument("<database-id>", "Database UUID")
    .action(async (rawDatabaseId: string) => {
      const databaseId = normalizeId(rawDatabaseId);
      await handleAction(async () => {
        const schema = await withBackend((backend) =>
          backend.getDatabaseSchema(databaseId),
        );

        printJson(schema);
      });
    });
}
