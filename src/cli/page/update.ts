import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerUpdate(page: Command): void {
  page.addCommand(
    defineCommand({
      use: "update <page-id>",
      short: "Update page properties",
      args: ExactArgs(1),
      options: {
        title: { type: "string", description: "Update the page title" },
        properties: { type: "string", description: "Property values to update (JSON)" },
        icon: { type: "string", description: "Update the page icon emoji" },
      },
      action: async ([rawPageId], opts) => {
        const pageId = normalizeId(rawPageId!);
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
      },
    }),
  );
}
