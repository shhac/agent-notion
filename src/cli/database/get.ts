import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";

export function registerGet(database: Command): void {
  database
    .command("get")
    .description("Get database metadata and property definitions")
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
        const description = (result.description as Array<{ plain_text: string }> | undefined)
          ?.map((t) => t.plain_text)
          .join("") ?? "";

        const properties: Record<string, unknown> = {};
        const rawProps = result.properties as Record<string, Record<string, unknown>> | undefined;
        if (rawProps) {
          for (const [name, prop] of Object.entries(rawProps)) {
            const entry: Record<string, unknown> = {
              id: prop.id,
              type: prop.type,
            };
            addPropertyOptions(entry, prop);
            properties[name] = entry;
          }
        }

        printJson({
          id: result.id,
          title,
          description: description || undefined,
          url: result.url,
          parent: formatParent(result.parent as Record<string, unknown> | undefined),
          properties,
          isInline: result.is_inline,
          createdAt: result.created_time,
          lastEditedAt: result.last_edited_time,
        });
      });
    });
}

function addPropertyOptions(entry: Record<string, unknown>, prop: Record<string, unknown>): void {
  const type = prop.type as string;
  const config = prop[type] as Record<string, unknown> | undefined;
  if (!config) return;

  switch (type) {
    case "select":
    case "multi_select": {
      const options = config.options as Array<{ name: string; color?: string }> | undefined;
      if (options) {
        entry.options = options.map((o) => ({ name: o.name, color: o.color }));
      }
      break;
    }
    case "status": {
      const options = config.options as Array<{ id: string; name: string; color?: string }> | undefined;
      const groups = config.groups as Array<{ name: string; option_ids?: string[] }> | undefined;
      if (options) {
        entry.options = options.map((o) => ({ name: o.name, color: o.color }));
      }
      if (groups && options) {
        entry.groups = groups.map((g) => ({
          name: g.name,
          options: options.filter((o) => g.option_ids?.includes(o.id)).map((o) => o.name),
        }));
      }
      break;
    }
    case "unique_id": {
      if (config.prefix) entry.prefix = config.prefix;
      break;
    }
    case "relation": {
      if (config.database_id) entry.relatedDatabase = config.database_id;
      break;
    }
  }
}

function formatParent(parent: Record<string, unknown> | undefined): Record<string, unknown> | undefined {
  if (!parent) return undefined;
  if (parent.type === "page_id") return { type: "page", id: parent.page_id as string };
  if (parent.type === "workspace") return { type: "workspace" };
  if (parent.type === "block_id") return { type: "block", id: parent.block_id as string };
  return undefined;
}
