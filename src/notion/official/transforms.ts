/**
 * Transform official Notion SDK responses to normalized types.
 */
import type {
  ParentRef,
  NormalizedBlock,
  SearchResult,
  DatabaseListItem,
  DatabaseDetail,
  PropertyDefinition,
  DatabaseSchema,
  SchemaProperty,
  QueryRow,
  PageDetail,
  CommentItem,
  UserItem,
} from "../types.ts";

// --- Shared helpers ---

type RichTextItem = { plain_text: string };
type SelectOption = { name: string; color?: string };
type PersonObject = { id: string; name?: string; person?: { email?: string }; type?: string };
type FileObject = { name: string; type: string; file?: { url: string }; external?: { url: string } };
type NotionProperty = { type: string; [key: string]: unknown };

function richTextToPlain(items: RichTextItem[] | undefined): string {
  if (!items || items.length === 0) return "";
  return items.map((t) => t.plain_text).join("");
}

export function formatParent(parent: Record<string, unknown> | undefined): ParentRef | undefined {
  if (!parent) return undefined;
  if (parent.type === "database_id") return { type: "database", id: parent.database_id as string };
  if (parent.type === "page_id") return { type: "page", id: parent.page_id as string };
  if (parent.type === "workspace") return { type: "workspace" };
  if (parent.type === "block_id") return { type: "block", id: parent.block_id as string };
  return undefined;
}

function formatIcon(icon: Record<string, unknown> | null | undefined): PageDetail["icon"] {
  if (!icon) return null;
  if (icon.type === "emoji") return { type: "emoji", emoji: icon.emoji as string };
  if (icon.type === "external") return { type: "external", url: (icon.external as Record<string, unknown>)?.url as string };
  return null;
}

function formatUser(user: Record<string, unknown> | undefined): { id: string; name?: string } | null {
  if (!user) return null;
  return { id: user.id as string, name: user.name as string | undefined };
}

// --- Property flattening (reused from properties.ts logic) ---

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
      return people?.map((p) => ({ id: p.id, name: p.name })) ?? [];
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
      return files?.map((f) => ({ name: f.name, url: f.file?.url ?? f.external?.url ?? null })) ?? [];
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

function flattenProperties(properties: Record<string, NotionProperty>): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  for (const [name, prop] of Object.entries(properties)) {
    result[name] = flattenPropertyValue(prop);
  }
  return result;
}

function extractTitle(properties: Record<string, NotionProperty>): string {
  for (const prop of Object.values(properties)) {
    if (prop.type === "title") {
      const items = prop.title as RichTextItem[] | undefined;
      return items?.map((t) => t.plain_text).join("") ?? "";
    }
  }
  return "";
}

// --- Property schema ---

function buildPropertyDefinition(prop: { id: string; type: string; [key: string]: unknown }): PropertyDefinition {
  const def: PropertyDefinition = { id: prop.id, type: prop.type };

  switch (prop.type) {
    case "select": {
      const config = prop.select as { options?: SelectOption[] } | undefined;
      if (config?.options) def.options = config.options.map((o) => ({ name: o.name, color: o.color }));
      break;
    }
    case "multi_select": {
      const config = prop.multi_select as { options?: SelectOption[] } | undefined;
      if (config?.options) def.options = config.options.map((o) => ({ name: o.name, color: o.color }));
      break;
    }
    case "status": {
      const config = prop.status as { options?: SelectOption[]; groups?: Array<{ name: string; option_ids?: string[] }> } | undefined;
      if (config?.options) def.options = config.options.map((o) => ({ name: o.name, color: o.color }));
      if (config?.groups) {
        def.groups = config.groups.map((g) => ({
          name: g.name,
          options: config.options?.filter((o) => g.option_ids?.includes((o as unknown as { id: string }).id)).map((o) => o.name) ?? [],
        }));
      }
      break;
    }
    case "unique_id": {
      const config = prop.unique_id as { prefix?: string } | undefined;
      if (config?.prefix) def.prefix = config.prefix;
      break;
    }
    case "relation": {
      const config = prop.relation as { database_id?: string } | undefined;
      if (config?.database_id) def.relatedDatabase = config.database_id;
      break;
    }
  }

  return def;
}

function buildSchemaProperty(name: string, prop: { id: string; type: string; [key: string]: unknown }): SchemaProperty {
  const schema: SchemaProperty = { name, id: prop.id, type: prop.type };
  const def = buildPropertyDefinition(prop);
  if (def.options) schema.options = def.options.map((o) => o.name);
  if (def.groups) {
    schema.groups = {};
    for (const g of def.groups) schema.groups[g.name] = g.options;
  }
  if (def.prefix) schema.prefix = def.prefix;
  if (def.relatedDatabase) schema.relatedDatabase = def.relatedDatabase;
  return schema;
}

// --- Block normalization ---

export function normalizeBlock(block: Record<string, unknown>): NormalizedBlock {
  const type = block.type as string;
  const data = block[type] as Record<string, unknown> | undefined;
  const richText = richTextToPlain(data?.rich_text as RichTextItem[] | undefined);

  const normalized: NormalizedBlock = {
    id: block.id as string,
    type,
    richText,
    hasChildren: block.has_children as boolean,
  };

  switch (type) {
    case "to_do":
      normalized.checked = (data as { checked?: boolean })?.checked ?? false;
      break;
    case "code":
      normalized.language = (data as { language?: string })?.language;
      break;
    case "image": {
      const img = data as { type?: string; file?: { url: string }; external?: { url: string }; caption?: RichTextItem[] };
      normalized.url = img?.file?.url ?? img?.external?.url;
      normalized.caption = richTextToPlain(img?.caption);
      break;
    }
    case "bookmark": {
      const bm = data as { url?: string; caption?: RichTextItem[] };
      normalized.url = bm?.url;
      normalized.caption = richTextToPlain(bm?.caption);
      break;
    }
    case "equation":
      normalized.expression = (data as { expression?: string })?.expression;
      break;
    case "child_page":
      normalized.title = (data as { title?: string })?.title;
      break;
    case "child_database":
      normalized.title = (data as { title?: string })?.title;
      break;
    case "callout": {
      const co = data as { icon?: { emoji?: string } };
      normalized.emoji = co?.icon?.emoji;
      break;
    }
    case "link_preview":
    case "embed":
      normalized.url = (data as { url?: string })?.url;
      break;
    case "video":
    case "pdf":
    case "audio":
    case "file": {
      const media = data as { type?: string; file?: { url: string }; external?: { url: string }; name?: string; caption?: RichTextItem[] };
      normalized.url = media?.file?.url ?? media?.external?.url;
      normalized.caption = richTextToPlain(media?.caption);
      normalized.title = media?.name;
      break;
    }
  }

  return normalized;
}

// --- High-level transforms for OfficialBackend ---

export function transformSearchResult(item: Record<string, unknown>): SearchResult {
  const obj = item.object as string;
  if (obj === "page") {
    const props = item.properties as Record<string, NotionProperty> | undefined;
    return {
      id: item.id as string,
      type: "page",
      title: props ? extractTitle(props) : "",
      url: item.url as string,
      parent: formatParent(item.parent as Record<string, unknown> | undefined),
      lastEditedAt: item.last_edited_time as string | undefined,
    };
  }
  // database
  const titleArr = item.title as RichTextItem[] | undefined;
  return {
    id: item.id as string,
    type: "database",
    title: titleArr?.map((t) => t.plain_text).join("") ?? "",
    url: item.url as string,
    parent: formatParent(item.parent as Record<string, unknown> | undefined),
    lastEditedAt: item.last_edited_time as string | undefined,
  };
}

export function transformDatabaseListItem(db: Record<string, unknown>): DatabaseListItem {
  const titleArr = db.title as RichTextItem[] | undefined;
  const props = db.properties as Record<string, unknown> | undefined;
  return {
    id: db.id as string,
    title: titleArr?.map((t) => t.plain_text).join("") ?? "",
    url: db.url as string,
    parent: formatParent(db.parent as Record<string, unknown> | undefined),
    propertyCount: props ? Object.keys(props).length : 0,
    lastEditedAt: db.last_edited_time as string | undefined,
  };
}

export function transformDatabaseDetail(db: Record<string, unknown>): DatabaseDetail {
  const titleArr = db.title as RichTextItem[] | undefined;
  const rawProps = db.properties as Record<string, { id: string; type: string; [key: string]: unknown }>;
  const descItems = db.description as RichTextItem[] | undefined;

  const properties: Record<string, PropertyDefinition> = {};
  for (const [name, prop] of Object.entries(rawProps)) {
    properties[name] = buildPropertyDefinition(prop);
  }

  return {
    id: db.id as string,
    title: titleArr?.map((t) => t.plain_text).join("") ?? "",
    description: richTextToPlain(descItems) || undefined,
    url: db.url as string,
    parent: formatParent(db.parent as Record<string, unknown> | undefined),
    properties,
    isInline: db.is_inline as boolean | undefined,
    createdAt: db.created_time as string | undefined,
    lastEditedAt: db.last_edited_time as string | undefined,
  };
}

export function transformDatabaseSchema(db: Record<string, unknown>): DatabaseSchema {
  const titleArr = db.title as RichTextItem[] | undefined;
  const rawProps = db.properties as Record<string, { id: string; type: string; [key: string]: unknown }>;
  return {
    id: db.id as string,
    title: titleArr?.map((t) => t.plain_text).join("") ?? "",
    properties: Object.entries(rawProps).map(([name, prop]) => buildSchemaProperty(name, prop)),
  };
}

export function transformQueryRow(page: Record<string, unknown>): QueryRow {
  const props = page.properties as Record<string, NotionProperty>;
  return {
    id: page.id as string,
    url: page.url as string,
    properties: flattenProperties(props),
    createdAt: page.created_time as string | undefined,
    lastEditedAt: page.last_edited_time as string | undefined,
  };
}

export function transformPageDetail(page: Record<string, unknown>): PageDetail {
  const props = page.properties as Record<string, NotionProperty>;
  return {
    id: page.id as string,
    url: page.url as string,
    parent: formatParent(page.parent as Record<string, unknown> | undefined),
    properties: flattenProperties(props),
    icon: formatIcon(page.icon as Record<string, unknown> | null | undefined),
    createdAt: page.created_time as string | undefined,
    createdBy: formatUser(page.created_by as Record<string, unknown> | undefined),
    lastEditedAt: page.last_edited_time as string | undefined,
    lastEditedBy: formatUser(page.last_edited_by as Record<string, unknown> | undefined),
    archived: page.archived as boolean | undefined,
  };
}

export function transformComment(comment: Record<string, unknown>): CommentItem {
  const richText = comment.rich_text as RichTextItem[] | undefined;
  const createdBy = comment.created_by as Record<string, unknown> | undefined;
  return {
    id: comment.id as string,
    body: richTextToPlain(richText),
    author: createdBy ? { id: createdBy.id as string, name: createdBy.name as string | undefined } : null,
    createdAt: comment.created_time as string | undefined,
  };
}

export function transformUser(user: Record<string, unknown>): UserItem {
  const personData = user.person as { email?: string } | undefined;
  return {
    id: user.id as string,
    name: user.name as string | undefined,
    type: user.type as "person" | "bot",
    email: personData?.email,
    avatarUrl: user.avatar_url as string | undefined,
  };
}
