import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerSchema(database: Command): void {
  database.addCommand(
    defineCommand({
      use: "schema <database-id>",
      short: "Get property definitions (compact, LLM-optimized)",
      args: ExactArgs(1),
      action: async ([rawDatabaseId]) => {
        const databaseId = normalizeId(rawDatabaseId!);
        await handleAction(async () => {
          const schema = await withBackend((backend) =>
            backend.getDatabaseSchema(databaseId),
          );
          printJson(schema);
        });
      },
    }),
  );
}
