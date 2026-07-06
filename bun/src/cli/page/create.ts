import { defineCommand, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerCreate(page: Command): void {
  page.addCommand(
    defineCommand({
      use: "create",
      short: "Create a new page",
      options: {
        parent: {
          type: "string",
          required: true,
          description: "Parent page ID or database ID",
        },
        title: { type: "string", required: true, description: "Page title" },
        properties: {
          type: "string",
          description: "Property values (JSON, for database parents)",
        },
        icon: { type: "string", description: "Page icon emoji" },
      },
      action: async (_args, opts) => {
        await handleAction(async () => {
          const parentId = normalizeId(opts.parent);
          const title = opts.title;

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
      },
    }),
  );
}
