import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { flattenPropertySchema } from "../../notion/properties.ts";

export function registerSchema(database: Command): void {
  database
    .command("schema")
    .description("Get property definitions (compact, LLM-optimized)")
    .argument("<database-id>", "Database UUID")
    .action(async (databaseId: string) => {
      await handleAction(async () => {
        const db = await withAutoRefresh((client) =>
          client.databases.retrieve({ database_id: databaseId }),
        );

        const result = db as Record<string, unknown>;
        const title = (result.title as Array<{ plain_text: string }> | undefined)
          ?.map((t) => t.plain_text)
          .join("") ?? "";

        const properties = flattenPropertySchema(
          result.properties as Parameters<typeof flattenPropertySchema>[0],
        );

        printJson({
          id: result.id,
          title,
          properties,
        });
      });
    });
}
