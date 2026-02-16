import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";

export function registerUpdate(page: Command): void {
  page
    .command("update")
    .description("Update page properties")
    .argument("<page-id>", "Page UUID")
    .option("--title <title>", "Update the page title")
    .option("--properties <json>", "Property values to update (JSON)")
    .option("--icon <emoji>", "Update the page icon emoji")
    .action(async (pageId: string, opts: Record<string, string | undefined>) => {
      await handleAction(async () => {
        if (!opts.title && !opts.properties && !opts.icon) {
          throw new CliError(
            "Nothing to update. Provide --title, --properties, or --icon.",
          );
        }

        const updateParams: Record<string, unknown> = {
          page_id: pageId,
        };

        const properties: Record<string, unknown> = {};

        if (opts.title) {
          // Find and update the title property
          properties.title = {
            title: [{ text: { content: opts.title } }],
          };
        }

        if (opts.properties) {
          let extra: Record<string, unknown>;
          try {
            extra = JSON.parse(opts.properties);
          } catch {
            throw new CliError(
              "Invalid --properties JSON. Expected an object with property names as keys.",
            );
          }
          for (const [key, value] of Object.entries(extra)) {
            properties[key] = buildPropertyValue(value);
          }
        }

        if (Object.keys(properties).length > 0) {
          updateParams.properties = properties;
        }

        if (opts.icon) {
          updateParams.icon = { type: "emoji", emoji: opts.icon };
        }

        const result = await withAutoRefresh((client) =>
          client.pages.update(updateParams as Parameters<typeof client.pages.update>[0]),
        );

        const p = result as Record<string, unknown>;
        printJson({
          id: p.id,
          url: p.url,
          lastEditedAt: p.last_edited_time,
        });
      });
    });
}

function buildPropertyValue(value: unknown): unknown {
  if (typeof value === "string") {
    return { select: { name: value } };
  }
  if (typeof value === "number") {
    return { number: value };
  }
  if (typeof value === "boolean") {
    return { checkbox: value };
  }
  if (Array.isArray(value)) {
    return { multi_select: value.map((v) => ({ name: String(v) })) };
  }
  return value;
}
