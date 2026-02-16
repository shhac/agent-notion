import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
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
        // Determine parent type by trying database first
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

        const createParams: Record<string, unknown> = {};

        // Build parent — try as database first (most common), fall back to page
        // We'll let the API decide and handle errors
        if (await isDatabaseParent(parentId)) {
          createParams.parent = { database_id: parentId };
          createParams.properties = buildDatabaseProperties(title, properties);
        } else {
          createParams.parent = { page_id: parentId };
          createParams.properties = {
            title: {
              title: [{ text: { content: title } }],
            },
          };
        }

        if (opts.icon) {
          createParams.icon = { type: "emoji", emoji: opts.icon };
        }

        const result = await withAutoRefresh((client) =>
          client.pages.create(createParams as Parameters<typeof client.pages.create>[0]),
        );

        const p = result as Record<string, unknown>;
        printJson({
          id: p.id,
          url: p.url,
          title,
          parent: p.parent,
          createdAt: p.created_time,
        });
      });
    });
}

function buildDatabaseProperties(
  title: string,
  extra?: Record<string, unknown>,
): Record<string, unknown> {
  const props: Record<string, unknown> = {
    // Title property — Notion databases always have exactly one title property
    // The actual property name varies, but the API accepts "title" type
    Name: {
      title: [{ text: { content: title } }],
    },
  };

  // Merge additional properties with simple value -> Notion API format conversion
  if (extra) {
    for (const [key, value] of Object.entries(extra)) {
      if (key === "Name" || key === "title") continue; // Already set
      props[key] = buildPropertyValue(value);
    }
  }

  return props;
}

function buildPropertyValue(value: unknown): unknown {
  if (typeof value === "string") {
    // Could be select, status, or rich_text — try select first (most common for simple strings)
    return { select: { name: value } };
  }
  if (typeof value === "number") {
    return { number: value };
  }
  if (typeof value === "boolean") {
    return { checkbox: value };
  }
  if (Array.isArray(value)) {
    // Assume multi_select
    return { multi_select: value.map((v) => ({ name: String(v) })) };
  }
  // Pass through as-is for complex objects (user provides Notion format)
  return value;
}

async function isDatabaseParent(id: string): Promise<boolean> {
  try {
    await (await import("../../notion/client.ts")).withAutoRefresh((client) =>
      client.databases.retrieve({ database_id: id }),
    );
    return true;
  } catch {
    return false;
  }
}
