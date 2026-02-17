import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerUpdate(page: Command): void {
  page
    .command("update")
    .description("Update page properties")
    .argument("<page-id>", "Page UUID")
    .option("--title <title>", "Update the page title")
    .option("--properties <json>", "Property values to update (JSON)")
    .option("--icon <emoji>", "Update the page icon emoji")
    .action(async (rawPageId: string, opts: Record<string, string | undefined>) => {
      const pageId = normalizeId(rawPageId);
      await handleAction(async () => {
        if (!opts.title && !opts.properties && !opts.icon) {
          throw new CliError(
            "Nothing to update. Provide --title, --properties, or --icon.",
          );
        }

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
          backend.updatePage({
            id: pageId,
            title: opts.title,
            properties,
            icon: opts.icon,
          }),
        );

        printJson(result);
      });
    });
}
