/**
 * Flatten Notion property values to human/LLM-readable formats.
 * Input: Notion API property objects. Output: simplified JSON values.
 */

type NotionProperty = {
  type: string;
  [key: string]: unknown;
};

type RichTextItem = {
  plain_text: string;
};

type SelectOption = {
  name: string;
  color?: string;
};

type PersonObject = {
  id: string;
  name?: string;
  person?: { email?: string };
  type?: string;
};

type FileObject = {
  name: string;
  type: string;
  file?: { url: string; expiry_time?: string };
  external?: { url: string };
};

/**
 * Flatten a single Notion property value to a simple JSON value.
 */
export function flattenPropertyValue(prop: NotionProperty): unknown {
  switch (prop.type) {
    case "title": {
      const items = prop.title as RichTextItem[] | undefined;
      return items?.map((t) => t.plain_text).join("") ?? "";
    }
    case "rich_text": {
      const items = prop.rich_text as RichTextItem[] | undefined;
      return items?.map((t) => t.plain_text).join("") ?? "";
    }
    case "number":
      return prop.number ?? null;
    case "select": {
      const sel = prop.select as SelectOption | null;
      return sel?.name ?? null;
    }
    case "multi_select": {
      const items = prop.multi_select as SelectOption[] | undefined;
      return items?.map((s) => s.name) ?? [];
    }
    case "status": {
      const status = prop.status as SelectOption | null;
      return status?.name ?? null;
    }
    case "date": {
      const date = prop.date as { start?: string; end?: string | null } | null;
      if (!date) return null;
      return { start: date.start, end: date.end ?? null };
    }
    case "people": {
      const people = prop.people as PersonObject[] | undefined;
      return (
        people?.map((p) => ({
          id: p.id,
          name: p.name,
        })) ?? []
      );
    }
    case "checkbox":
      return prop.checkbox as boolean;
    case "url":
      return (prop.url as string) ?? null;
    case "email":
      return (prop.email as string) ?? null;
    case "phone_number":
      return (prop.phone_number as string) ?? null;
    case "relation": {
      const rels = prop.relation as { id: string }[] | undefined;
      return rels?.map((r) => ({ id: r.id })) ?? [];
    }
    case "rollup": {
      const rollup = prop.rollup as NotionProperty | undefined;
      if (!rollup) return null;
      // Recursively flatten the rollup result
      if (rollup.type === "array") {
        const items = rollup.array as NotionProperty[] | undefined;
        return items?.map((item) => flattenPropertyValue(item)) ?? [];
      }
      return flattenPropertyValue(rollup);
    }
    case "formula": {
      const formula = prop.formula as NotionProperty | undefined;
      if (!formula) return null;
      return formula[formula.type as string] ?? null;
    }
    case "files": {
      const files = prop.files as FileObject[] | undefined;
      return (
        files?.map((f) => ({
          name: f.name,
          url: f.file?.url ?? f.external?.url ?? null,
        })) ?? []
      );
    }
    case "created_time":
      return (prop.created_time as string) ?? null;
    case "last_edited_time":
      return (prop.last_edited_time as string) ?? null;
    case "created_by": {
      const user = prop.created_by as PersonObject | undefined;
      return user ? { id: user.id, name: user.name } : null;
    }
    case "last_edited_by": {
      const user = prop.last_edited_by as PersonObject | undefined;
      return user ? { id: user.id, name: user.name } : null;
    }
    case "unique_id": {
      const uid = prop.unique_id as { prefix?: string; number?: number } | undefined;
      if (!uid) return null;
      return uid.prefix ? `${uid.prefix}-${uid.number}` : String(uid.number);
    }
    case "verification": {
      const v = prop.verification as { state?: string } | undefined;
      return v?.state ?? null;
    }
    default:
      return null;
  }
}

/**
 * Flatten all properties from a Notion page to simple key-value pairs.
 */
export function flattenProperties(
  properties: Record<string, NotionProperty>,
): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  for (const [name, prop] of Object.entries(properties)) {
    result[name] = flattenPropertyValue(prop);
  }
  return result;
}

/**
 * Extract the title from a Notion page's properties.
 */
export function extractTitle(
  properties: Record<string, NotionProperty>,
): string {
  for (const prop of Object.values(properties)) {
    if (prop.type === "title") {
      const items = prop.title as RichTextItem[] | undefined;
      return items?.map((t) => t.plain_text).join("") ?? "";
    }
  }
  return "";
}

/**
 * Flatten database property schema definitions for LLM consumption.
 */
export function flattenPropertySchema(
  properties: Record<string, { id: string; type: string; [key: string]: unknown }>,
): Array<Record<string, unknown>> {
  return Object.entries(properties).map(([name, prop]) => {
    const schema: Record<string, unknown> = {
      name,
      id: prop.id,
      type: prop.type,
    };

    // Include options for select/multi_select/status
    switch (prop.type) {
      case "select": {
        const config = prop.select as { options?: SelectOption[] } | undefined;
        if (config?.options) {
          schema.options = config.options.map((o) => o.name);
        }
        break;
      }
      case "multi_select": {
        const config = prop.multi_select as { options?: SelectOption[] } | undefined;
        if (config?.options) {
          schema.options = config.options.map((o) => o.name);
        }
        break;
      }
      case "status": {
        const config = prop.status as {
          options?: SelectOption[];
          groups?: Array<{ name: string; option_ids?: string[]; color?: string }>;
        } | undefined;
        if (config?.options) {
          schema.options = config.options.map((o) => o.name);
        }
        if (config?.groups) {
          const groups: Record<string, string[]> = {};
          for (const group of config.groups) {
            const optionNames = config.options
              ?.filter((o) => {
                const optId = (o as unknown as { id: string }).id;
                return group.option_ids?.includes(optId);
              })
              .map((o) => o.name) ?? [];
            groups[group.name] = optionNames;
          }
          schema.groups = groups;
        }
        break;
      }
      case "unique_id": {
        const config = prop.unique_id as { prefix?: string } | undefined;
        if (config?.prefix) {
          schema.prefix = config.prefix;
        }
        break;
      }
      case "relation": {
        const config = prop.relation as { database_id?: string } | undefined;
        if (config?.database_id) {
          schema.relatedDatabase = config.database_id;
        }
        break;
      }
    }

    return schema;
  });
}
