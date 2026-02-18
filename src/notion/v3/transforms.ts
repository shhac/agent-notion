/**
 * Transform v3 RecordMap responses to normalized types.
 * Handles: rich text conversion, property ID→name resolution,
 * timestamp conversion, parent format normalization.
 */
import type {
  ParentRef,
  NormalizedBlock,
  SearchResult,
  DatabaseListItem,
  DatabaseDetail,
  PropertyDefinition,
  SchemaProperty,
  DatabaseSchema,
  QueryRow,
  PageDetail,
  CommentItem,
  UserItem,
  UserMe,
} from "../types.ts";
import type {
  RecordMap,
  V3Block,
  V3RichText,
  V3Decoration,
  V3Collection,
  V3PropertySchema,
  V3User,
  V3Discussion,
  V3Comment,
} from "./client.ts";

// --- Rich text ---

/** Convert v3 rich text array to plain text string. */
export function v3RichTextToPlain(richText: V3RichText | undefined): string {
  if (!richText || richText.length === 0) return "";
  return richText.map((segment) => segment[0]).join("");
}

// --- Timestamps ---

/** Convert unix milliseconds to ISO 8601 string. */
function msToIso(ms: number | undefined): string | undefined {
  if (ms === undefined) return undefined;
  return new Date(ms).toISOString();
}

// --- Parent ---

/** Convert v3 parent_table/parent_id to normalized ParentRef. */
export function v3Parent(parentTable: string, parentId: string): ParentRef | undefined {
  switch (parentTable) {
    case "collection":
      return { type: "database", id: parentId };
    case "block":
      return { type: "page", id: parentId };
    case "space":
      return { type: "workspace", id: parentId };
    default:
      return undefined;
  }
}

// --- Notion URL ---

function notionUrl(id: string): string {
  return `https://www.notion.so/${id.replace(/-/g, "")}`;
}

// --- Property value flattening ---

/**
 * Flatten a v3 property value using the schema to determine type.
 * V3 properties are keyed by internal ID with rich text values.
 * Schema maps ID → { name, type, options }.
 */
export function flattenV3PropertyValue(
  value: V3RichText | undefined,
  schema: V3PropertySchema,
): unknown {
  const text = v3RichTextToPlain(value);

  switch (schema.type) {
    case "title":
      return text;
    case "text":
      return text;
    case "number":
      return text ? Number(text) : null;
    case "select":
      return text || null;
    case "multi_select":
      return text ? text.split(",") : [];
    case "status":
      return text || null;
    case "date": {
      if (!value || value.length === 0) return null;
      // v3 dates are stored as decorations: [["‣", [["d", { start_date, end_date, ... }]]]]
      for (const segment of value) {
        if (segment.length > 1) {
          const decorations = segment[1] as Array<[string, ...unknown[]]>;
          for (const dec of decorations) {
            if (dec[0] === "d") {
              const dateObj = dec[1] as { start_date?: string; end_date?: string } | undefined;
              if (dateObj) {
                return { start: dateObj.start_date ?? null, end: dateObj.end_date ?? null };
              }
            }
          }
        }
      }
      return text ? { start: text, end: null } : null;
    }
    case "person":
    case "people": {
      if (!value || value.length === 0) return [];
      // v3 people are stored as mentions: [["‣", [["u", "user-id"]]]]
      const people: Array<{ id: string }> = [];
      for (const segment of value) {
        if (segment.length > 1) {
          const decorations = segment[1] as Array<[string, ...unknown[]]>;
          for (const dec of decorations) {
            if (dec[0] === "u" && dec[1]) {
              people.push({ id: dec[1] as string });
            }
          }
        }
      }
      return people;
    }
    case "checkbox":
      return text === "Yes";
    case "url":
      return text || null;
    case "email":
      return text || null;
    case "phone_number":
      return text || null;
    case "relation": {
      if (!value || value.length === 0) return [];
      // v3 relations are stored as page mentions: [["‣", [["p", "page-id"]]]]
      const relations: Array<{ id: string }> = [];
      for (const segment of value) {
        if (segment.length > 1) {
          const decorations = segment[1] as Array<[string, ...unknown[]]>;
          for (const dec of decorations) {
            if (dec[0] === "p" && dec[1]) {
              relations.push({ id: dec[1] as string });
            }
          }
        }
      }
      return relations;
    }
    case "created_time":
      return text || null;
    case "last_edited_time":
      return text || null;
    case "created_by":
      return text ? { id: text } : null;
    case "last_edited_by":
      return text ? { id: text } : null;
    case "formula":
      return text || null;
    case "rollup":
      return text || null;
    case "files":
      // v3 files are complex — return raw text for now
      return text ? [{ name: text, url: null }] : [];
    case "unique_id":
      return text || null;
    default:
      return text || null;
  }
}

/**
 * Flatten all v3 properties of a block using collection schema.
 * Resolves internal IDs to human-readable property names.
 */
export function flattenV3Properties(
  properties: Record<string, V3RichText> | undefined,
  schema: Record<string, V3PropertySchema>,
): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  if (!properties) return result;

  for (const [propId, propSchema] of Object.entries(schema)) {
    const value = properties[propId];
    result[propSchema.name] = flattenV3PropertyValue(value, propSchema);
  }

  return result;
}

// --- Block normalization ---

/** v3 block type → official API block type mapping */
const V3_BLOCK_TYPE_MAP: Record<string, string> = {
  text: "paragraph",
  header: "heading_1",
  sub_header: "heading_2",
  sub_sub_header: "heading_3",
  bulleted_list: "bulleted_list_item",
  numbered_list: "numbered_list_item",
  to_do: "to_do",
  toggle: "toggle",
  code: "code",
  quote: "quote",
  callout: "callout",
  divider: "divider",
  image: "image",
  bookmark: "bookmark",
  equation: "equation",
  page: "child_page",
  collection_view_page: "child_database",
  collection_view: "child_database",
  table_of_contents: "table_of_contents",
  breadcrumb: "breadcrumb",
  column_list: "column_list",
  column: "column",
  synced_block: "synced_block",
  link_preview: "link_preview",
  embed: "embed",
  video: "video",
  pdf: "pdf",
  audio: "audio",
  file: "file",
};

/** Convert a v3 block to a NormalizedBlock. */
export function normalizeV3Block(block: V3Block): NormalizedBlock {
  const type = V3_BLOCK_TYPE_MAP[block.type] ?? block.type;
  const titleText = v3RichTextToPlain(block.properties?.title);
  const hasChildren = Boolean(block.content && block.content.length > 0);

  const normalized: NormalizedBlock = {
    id: block.id,
    type,
    richText: titleText,
    hasChildren,
  };

  switch (block.type) {
    case "to_do": {
      const checked = block.properties?.checked;
      normalized.checked = checked ? v3RichTextToPlain(checked) === "Yes" : false;
      break;
    }
    case "code": {
      const lang = block.properties?.language;
      normalized.language = lang ? v3RichTextToPlain(lang) : undefined;
      break;
    }
    case "image": {
      const format = block.format as { display_source?: string } | undefined;
      const source = block.properties?.source;
      normalized.url = format?.display_source ?? (source ? v3RichTextToPlain(source) : undefined);
      const caption = block.properties?.caption;
      normalized.caption = caption ? v3RichTextToPlain(caption) : undefined;
      break;
    }
    case "bookmark": {
      const link = block.properties?.link;
      normalized.url = link ? v3RichTextToPlain(link) : undefined;
      const desc = block.properties?.description;
      normalized.caption = desc ? v3RichTextToPlain(desc) : undefined;
      break;
    }
    case "equation": {
      // v3 equations use title property for the expression
      normalized.expression = titleText;
      break;
    }
    case "page":
    case "collection_view_page":
    case "collection_view": {
      normalized.title = titleText || undefined;
      break;
    }
    case "callout": {
      const format = block.format as { page_icon?: string } | undefined;
      normalized.emoji = format?.page_icon;
      break;
    }
    case "embed":
    case "link_preview": {
      const source = block.properties?.source;
      normalized.url = source ? v3RichTextToPlain(source) : undefined;
      break;
    }
    case "video":
    case "pdf":
    case "audio":
    case "file": {
      const source = block.properties?.source;
      normalized.url = source ? v3RichTextToPlain(source) : undefined;
      const caption = block.properties?.caption;
      normalized.caption = caption ? v3RichTextToPlain(caption) : undefined;
      normalized.title = block.properties?.title ? v3RichTextToPlain(block.properties.title) : undefined;
      break;
    }
  }

  return normalized;
}

// --- High-level transforms ---

/** Extract title from a v3 block's properties. */
function extractBlockTitle(block: V3Block): string {
  return v3RichTextToPlain(block.properties?.title);
}

/** Transform v3 search result + recordMap block to SearchResult. */
export function transformV3SearchResult(
  block: V3Block,
): SearchResult {
  const isCollection = block.type === "collection_view_page" || block.type === "collection_view";
  return {
    id: block.id,
    type: isCollection ? "database" : "page",
    title: extractBlockTitle(block),
    url: notionUrl(block.id),
    parent: v3Parent(block.parent_table, block.parent_id),
    lastEditedAt: msToIso(block.last_edited_time),
  };
}

/** Transform a v3 collection to DatabaseListItem. */
export function transformV3DatabaseListItem(
  collection: V3Collection,
  collectionViewPageId?: string,
): DatabaseListItem {
  return {
    id: collectionViewPageId ?? collection.parent_id,
    title: v3RichTextToPlain(collection.name),
    url: notionUrl(collectionViewPageId ?? collection.parent_id),
    parent: v3Parent(collection.parent_table, collection.parent_id),
    propertyCount: Object.keys(collection.schema ?? {}).length,
    lastEditedAt: undefined, // collections don't have last_edited_time
  };
}

/** Transform a v3 collection to DatabaseDetail. */
export function transformV3DatabaseDetail(
  collection: V3Collection,
  collectionViewPageId?: string,
): DatabaseDetail {
  const properties: Record<string, PropertyDefinition> = {};

  for (const [propId, propSchema] of Object.entries(collection.schema ?? {})) {
    const def: PropertyDefinition = {
      id: propId,
      type: propSchema.type,
    };

    if (propSchema.options) {
      def.options = propSchema.options.map((o) => ({ name: o.value, color: o.color }));
    }
    if (propSchema.groups) {
      def.groups = propSchema.groups.map((g) => ({
        name: g.name,
        options: g.optionIds
          ? (propSchema.options ?? [])
              .filter((o) => g.optionIds!.includes(o.id))
              .map((o) => o.value)
          : [],
      }));
    }
    if (propSchema.collection_id) {
      def.relatedDatabase = propSchema.collection_id;
    }

    properties[propSchema.name] = def;
  }

  return {
    id: collectionViewPageId ?? collection.parent_id,
    title: v3RichTextToPlain(collection.name),
    description: collection.description ? v3RichTextToPlain(collection.description) : undefined,
    url: notionUrl(collectionViewPageId ?? collection.parent_id),
    parent: v3Parent(collection.parent_table, collection.parent_id),
    properties,
    createdAt: undefined,
    lastEditedAt: undefined,
  };
}

/** Transform a v3 collection to DatabaseSchema. */
export function transformV3DatabaseSchema(
  collection: V3Collection,
  collectionViewPageId?: string,
): DatabaseSchema {
  return {
    id: collectionViewPageId ?? collection.parent_id,
    title: v3RichTextToPlain(collection.name),
    properties: Object.entries(collection.schema ?? {}).map(([propId, propSchema]) => {
      const schema: SchemaProperty = {
        name: propSchema.name,
        id: propId,
        type: propSchema.type,
      };

      if (propSchema.options) {
        schema.options = propSchema.options.map((o) => o.value);
      }
      if (propSchema.groups) {
        schema.groups = {};
        for (const g of propSchema.groups) {
          schema.groups[g.name] = g.optionIds
            ? (propSchema.options ?? [])
                .filter((o) => g.optionIds!.includes(o.id))
                .map((o) => o.value)
            : [];
        }
      }
      if (propSchema.collection_id) {
        schema.relatedDatabase = propSchema.collection_id;
      }

      return schema;
    }),
  };
}

/** Transform a v3 block (page row) + collection schema to QueryRow. */
export function transformV3QueryRow(
  block: V3Block,
  schema: Record<string, V3PropertySchema>,
): QueryRow {
  return {
    id: block.id,
    url: notionUrl(block.id),
    properties: flattenV3Properties(block.properties, schema),
    createdAt: msToIso(block.created_time),
    lastEditedAt: msToIso(block.last_edited_time),
  };
}

/** Transform a v3 page block to PageDetail. */
export function transformV3PageDetail(
  block: V3Block,
  schema?: Record<string, V3PropertySchema>,
): PageDetail {
  const properties = schema
    ? flattenV3Properties(block.properties, schema)
    : { title: extractBlockTitle(block) };

  const format = block.format as { page_icon?: string } | undefined;

  return {
    id: block.id,
    url: notionUrl(block.id),
    parent: v3Parent(block.parent_table, block.parent_id),
    properties,
    icon: format?.page_icon
      ? { type: "emoji", emoji: format.page_icon }
      : null,
    createdAt: msToIso(block.created_time),
    createdBy: null, // v3 blocks don't embed user details
    lastEditedAt: msToIso(block.last_edited_time),
    lastEditedBy: null,
    archived: !block.alive,
  };
}

/** Transform a v3 user to UserItem. */
export function transformV3User(user: V3User): UserItem {
  return {
    id: user.id,
    name: [user.given_name, user.family_name].filter(Boolean).join(" ") || undefined,
    type: "person",
    email: user.email,
    avatarUrl: user.profile_photo,
  };
}

/** Transform v3 user content to UserMe. */
export function transformV3UserMe(user: V3User, spaceName?: string): UserMe {
  return {
    id: user.id,
    name: [user.given_name, user.family_name].filter(Boolean).join(" ") || undefined,
    type: "person",
    workspaceName: spaceName,
  };
}

// --- Rich text decoration injection ---

/**
 * Add a decoration to a character range within a V3RichText array.
 * Splits segments at range boundaries and preserves existing decorations.
 *
 * @param richText - The existing rich text segments
 * @param start - Start character offset (inclusive)
 * @param end - End character offset (exclusive)
 * @param decoration - The decoration to add, e.g. ["m", discussionId]
 * @returns New V3RichText with the decoration applied to the range
 */
export function addDecorationToRange(
  richText: V3RichText,
  start: number,
  end: number,
  decoration: V3Decoration,
): V3RichText {
  if (start < 0 || end <= start) {
    throw new Error(`Invalid range: [${start}, ${end})`);
  }

  const result: V3RichText = [];
  let offset = 0;

  for (const segment of richText) {
    const segText = segment[0];
    const segDecorations = segment.length > 1 ? (segment[1] as V3Decoration[]) : undefined;
    const segStart = offset;
    const segEnd = offset + segText.length;

    // No overlap with target range — pass through unchanged
    if (segEnd <= start || segStart >= end) {
      result.push(segment);
      offset = segEnd;
      continue;
    }

    // Split into up to 3 parts: before, overlap, after
    const overlapStart = Math.max(segStart, start);
    const overlapEnd = Math.min(segEnd, end);

    // Before part (keeps original decorations, no new decoration)
    if (overlapStart > segStart) {
      const beforeText = segText.slice(0, overlapStart - segStart);
      result.push(segDecorations ? [beforeText, [...segDecorations]] : [beforeText]);
    }

    // Overlap part (add new decoration alongside existing ones)
    const overlapText = segText.slice(overlapStart - segStart, overlapEnd - segStart);
    const overlapDecs = segDecorations ? [...segDecorations, decoration] : [decoration];
    result.push([overlapText, overlapDecs]);

    // After part (keeps original decorations, no new decoration)
    if (overlapEnd < segEnd) {
      const afterText = segText.slice(overlapEnd - segStart);
      result.push(segDecorations ? [afterText, [...segDecorations]] : [afterText]);
    }

    offset = segEnd;
  }

  return result;
}

/**
 * Convenience wrapper: inject an ["m", discussionId] decoration on the first
 * occurrence of `targetText` within a V3RichText array.
 * Throws if targetText is empty or not found.
 */
export function injectCommentDecoration(
  richText: V3RichText,
  targetText: string,
  discussionId: string,
): V3RichText {
  if (!targetText) throw new Error("Target text for inline comment cannot be empty");

  const fullText = richText.map((seg) => seg[0]).join("");
  const start = fullText.indexOf(targetText);
  if (start === -1) {
    throw new Error(`Target text "${targetText}" not found in block text "${fullText}"`);
  }

  return addDecorationToRange(richText, start, start + targetText.length, ["m", discussionId]);
}

// --- Reverse transforms (write direction) ---

/** Convert a plain text string to v3 rich text format. */
export function toV3RichText(text: string): V3RichText {
  return [[text]];
}

/** Reverse of V3_BLOCK_TYPE_MAP: official API block type → v3 block type. */
const OFFICIAL_TO_V3_BLOCK_TYPE: Record<string, string> = {};
for (const [v3Type, officialType] of Object.entries(V3_BLOCK_TYPE_MAP)) {
  // First entry wins (avoids collection_view overwriting collection_view_page for child_database)
  if (!(officialType in OFFICIAL_TO_V3_BLOCK_TYPE)) {
    OFFICIAL_TO_V3_BLOCK_TYPE[officialType] = v3Type;
  }
}

/** Convert an official API block type to its v3 equivalent. */
export function officialBlockTypeToV3(type: string): string {
  return OFFICIAL_TO_V3_BLOCK_TYPE[type] ?? type;
}

/**
 * Convert a property value to v3 rich text format based on schema type.
 * Supports simple types; throws clear errors for complex types that need
 * special v3 decoration formats.
 */
export function buildV3PropertyValue(
  value: unknown,
  schemaType: string,
): V3RichText {
  switch (schemaType) {
    case "title":
    case "text":
    case "url":
    case "email":
    case "phone_number":
      return [[String(value ?? "")]];
    case "number":
      return [[String(value ?? 0)]];
    case "select":
    case "status":
      return [[String(value ?? "")]];
    case "multi_select":
      if (Array.isArray(value)) return [[value.join(",")]];
      return [[String(value ?? "")]];
    case "checkbox":
      return value ? [["Yes"]] : [["No"]];
    case "date":
      throw new Error(
        `Property type "date" requires v3 decoration format (‣ with "d" decoration). ` +
        `Use the official API backend for date properties, or pass pre-formatted v3 rich text.`,
      );
    case "relation":
      throw new Error(
        `Property type "relation" requires v3 decoration format (‣ with "p" decoration). ` +
        `Use the official API backend for relation properties, or pass pre-formatted v3 rich text.`,
      );
    case "person":
    case "people":
      throw new Error(
        `Property type "${schemaType}" requires v3 decoration format (‣ with "u" decoration). ` +
        `Use the official API backend for people properties, or pass pre-formatted v3 rich text.`,
      );
    case "files":
      throw new Error(
        `Property type "files" requires complex v3 format. ` +
        `Use the official API backend for file properties.`,
      );
    default:
      // For unknown types, attempt plain text conversion
      return [[String(value ?? "")]];
  }
}

// --- RecordMap helpers ---

/** Extract a block from a RecordMap by ID. */
export function getBlock(recordMap: RecordMap, id: string): V3Block | undefined {
  return recordMap.block?.[id]?.value as V3Block | undefined;
}

/** Extract a collection from a RecordMap by ID. */
export function getCollection(recordMap: RecordMap, id: string): V3Collection | undefined {
  return recordMap.collection?.[id]?.value as V3Collection | undefined;
}

/** Get all blocks from a RecordMap. */
export function getAllBlocks(recordMap: RecordMap): V3Block[] {
  if (!recordMap.block) return [];
  return Object.values(recordMap.block)
    .map((entry) => entry.value as V3Block)
    .filter((block) => block && block.alive !== false);
}

/** Get the first collection from a RecordMap. */
export function getFirstCollection(recordMap: RecordMap): V3Collection | undefined {
  if (!recordMap.collection) return undefined;
  const entries = Object.values(recordMap.collection);
  return entries[0]?.value as V3Collection | undefined;
}

/** Get the first collection view ID from a RecordMap. */
export function getFirstCollectionViewId(recordMap: RecordMap): string | undefined {
  if (!recordMap.collection_view) return undefined;
  return Object.keys(recordMap.collection_view)[0];
}

/** Get the first user from a RecordMap. */
export function getFirstUser(recordMap: RecordMap): V3User | undefined {
  if (!recordMap.notion_user) return undefined;
  const entries = Object.values(recordMap.notion_user);
  return entries[0]?.value as V3User | undefined;
}

/** Get all users from a RecordMap. */
export function getAllUsers(recordMap: RecordMap): V3User[] {
  if (!recordMap.notion_user) return [];
  return Object.values(recordMap.notion_user)
    .map((entry) => entry.value as V3User)
    .filter(Boolean);
}

/** Extract a discussion from a RecordMap by ID. */
export function getDiscussion(recordMap: RecordMap, id: string): V3Discussion | undefined {
  return recordMap.discussion?.[id]?.value as V3Discussion | undefined;
}

/** Extract a comment from a RecordMap by ID. */
export function getComment(recordMap: RecordMap, id: string): V3Comment | undefined {
  return recordMap.comment?.[id]?.value as V3Comment | undefined;
}

/** Extract a user from a RecordMap by ID. */
export function getUser(recordMap: RecordMap, id: string): V3User | undefined {
  return recordMap.notion_user?.[id]?.value as V3User | undefined;
}

/** Transform a v3 comment record to a normalized CommentItem. */
export function transformV3Comment(comment: V3Comment, user?: V3User, anchorText?: string): CommentItem {
  const item: CommentItem = {
    id: comment.id,
    body: v3RichTextToPlain(comment.text),
    author: user
      ? { id: user.id, name: [user.given_name, user.family_name].filter(Boolean).join(" ") || undefined }
      : comment.created_by_id
        ? { id: comment.created_by_id }
        : null,
    createdAt: msToIso(comment.created_time),
  };
  if (anchorText !== undefined) {
    item.anchorText = anchorText;
  }
  return item;
}

/**
 * Extract the anchor text for a discussion from a block's rich text.
 * Looks for characters decorated with ["m", discussionId] and concatenates them.
 * Returns undefined if no anchor decorations are found.
 */
export function extractAnchorText(richText: V3RichText | undefined, discussionId: string): string | undefined {
  if (!richText || richText.length === 0) return undefined;

  let anchor = "";
  for (const segment of richText) {
    const text = segment[0];
    const decorations = segment.length > 1 ? (segment[1] as V3Decoration[]) : undefined;
    if (decorations) {
      const hasDiscussion = decorations.some(
        (d) => d[0] === "m" && d[1] === discussionId,
      );
      if (hasDiscussion) {
        anchor += text;
      }
    }
  }

  return anchor.length > 0 ? anchor : undefined;
}
