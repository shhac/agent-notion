import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";

export function registerCreate(page: Command): void {
  page
    .command("create")
    .description("Create a new page")
    .requiredOption("--parent <id>", "Parent page ID or database ID")
    .requiredOption("--title <title>", "Page title")
    .option("--properties <json>", "Property values (JSON, for database parents)")
    .option("--icon <emoji>", "Page icon emoji")
    .action(async (opts: Record<string, string | undefined>) => {
      await handleAction(async () => {
        const parentId = opts.parent!;
        const title = opts.title!;

        let properties: Record<string, unknown> | undefined;
        if (opts.properties) {
          try {
            properties = JSON.parse(opts.properties);
          } catch {
            throw new CliError(
              "Invalid --properties JSON. Expected an object with property names as keys.",
            );
          }
        }

        const result = await withBackend((backend) =>
          backend.createPage({
            parentId,
            title,
            properties,
            icon: opts.icon,
          }),
        );

        printJson(result);
      });
    });
}
