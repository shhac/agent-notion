/**
 * V3 RecordMap — the shared data vocabulary of the v3 internal API.
 * Entity types, wire-format normalization, and typed lookup helpers.
 * No HTTP here: the transport lives in client.ts.
 */

// --- Wire-format normalization ---

/**
 * Notion's v3 API changed the recordMap entry format from:
 *   { value: V3Block, role?: string }
 * to:
 *   { spaceId: string, value: { value: V3Block, role?: string } }
 *
 * This function unwraps the extra nesting so all consumers work unchanged.
 * Applied automatically in the client's post() method to all API responses.
 */
export function normalizeRecordMapResponse<T>(result: T): T {
  if (!result || typeof result !== "object" || !("recordMap" in result)) {
    return result;
  }

  const r = result as Record<string, unknown>;
  const rm = r.recordMap;
  if (!rm || typeof rm !== "object") return result;

  const normalized: Record<string, unknown> = {};
  for (const [table, records] of Object.entries(rm as Record<string, unknown>)) {
    if (!records || typeof records !== "object") {
      normalized[table] = records;
      continue;
    }

    const tableRecords: Record<string, unknown> = {};
    for (const [id, entry] of Object.entries(records as Record<string, unknown>)) {
      tableRecords[id] = unwrapRecordMapEntry(entry);
    }
    normalized[table] = tableRecords;
  }

  r.recordMap = normalized;
  return result;
}

function isObjectRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

/**
 * A role-wrapped value nests the actual entity one level deeper:
 * { value: entity, role }. In the old format that slot holds a primitive
 * (e.g. a version number), so "object-valued `value`" is the discriminator.
 */
function isRoleWrapped(value: unknown): value is { value: Record<string, unknown> } {
  return isObjectRecord(value) && isObjectRecord(value.value);
}

/** Return the actual v3 entity from either an entity or a role-wrapped value. */
export function unwrapRecordValue(value: unknown): Record<string, unknown> | undefined {
  if (!isObjectRecord(value)) return undefined;
  return isRoleWrapped(value) ? value.value : value;
}

/** Return a v3 RecordMap entry in the normalized { value: entity, role } shape. */
function unwrapRecordMapEntry(entry: unknown): unknown {
  if (!isObjectRecord(entry)) return entry;
  return isRoleWrapped(entry.value) ? entry.value : entry;
}

// --- RecordMap type ---

/**
 * Invariant: entries are always in the normalized `{ value: entity, role? }` shape.
 * The client's `post()` runs `normalizeRecordMapResponse` on every response, so
 * consumers past the client boundary read `entry.value` directly and must not
 * re-unwrap. Only code that fetches v3 data without going through `post()`
 * (e.g. desktop-token's raw getSpaces call) needs `unwrapRecordValue`.
 */
export type RecordMap = {
  block?: Record<string, { value: V3Block; role?: string }>;
  collection?: Record<string, { value: V3Collection; role?: string }>;
  collection_view?: Record<string, { value: V3CollectionView; role?: string }>;
  notion_user?: Record<string, { value: V3User; role?: string }>;
  space?: Record<string, { value: V3Space; role?: string }>;
  [table: string]: Record<string, { value: Record<string, unknown>; role?: string }> | undefined;
};

export type V3Block = {
  id: string;
  type: string;
  version: number;
  created_time: number;
  last_edited_time: number;
  parent_id: string;
  parent_table: string;
  alive: boolean;
  properties?: Record<string, V3RichText>;
  content?: string[];
  format?: Record<string, unknown>;
  space_id: string;
  [key: string]: unknown;
};

/** V3 rich text: array of [text, decorations?] tuples */
export type V3RichText = Array<[string] | [string, V3Decoration[]]>;

/** V3 decoration: [type, ...args] */
export type V3Decoration = [string, ...unknown[]];

export type V3Collection = {
  id: string;
  version: number;
  name: V3RichText;
  description?: V3RichText;
  schema: Record<string, V3PropertySchema>;
  parent_id: string;
  parent_table: string;
  icon?: string;
  cover?: string;
  format?: Record<string, unknown>;
  [key: string]: unknown;
};

export type V3PropertySchema = {
  name: string;
  type: string;
  options?: Array<{ id: string; value: string; color?: string }>;
  groups?: Array<{ id: string; name: string; optionIds?: string[]; color?: string }>;
  number_format?: string;
  collection_id?: string;
  [key: string]: unknown;
};

export type V3CollectionView = {
  id: string;
  version: number;
  type: string;
  name?: string;
  parent_id: string;
  parent_table: string;
  alive: boolean;
  format?: Record<string, unknown>;
  query2?: {
    filter?: unknown;
    sort?: unknown;
    aggregations?: unknown;
  };
  [key: string]: unknown;
};

export type V3User = {
  id: string;
  version: number;
  email: string;
  given_name: string;
  family_name: string;
  profile_photo?: string;
  [key: string]: unknown;
};

export type V3Space = {
  id: string;
  version: number;
  name: string;
  icon?: string;
  domain?: string;
  plan_type?: string;
  [key: string]: unknown;
};

export type V3Discussion = {
  id: string;
  version: number;
  parent_id: string;
  parent_table: string;
  resolved: boolean;
  comments: string[];
  [key: string]: unknown;
};

export type V3Comment = {
  id: string;
  version: number;
  alive: boolean;
  parent_id: string;
  parent_table: string;
  text: V3RichText;
  created_by_id: string;
  created_by_table: string;
  created_time: number;
  last_edited_time: number;
  [key: string]: unknown;
};

export type V3Snapshot = {
  id: string;
  version: number;
  last_version: number;
  timestamp: number;
  authors: Array<{ id: string; table: string }>;
};

export type V3Activity = {
  id: string;
  version: number;
  type: string;
  parent_id: string;
  parent_table: string;
  navigable_block_id?: string;
  collection_id?: string;
  space_id: string;
  edits?: Array<{
    type: string;
    block_id?: string;
    timestamp: number;
    authors?: Array<{ id: string; table: string }>;
    [key: string]: unknown;
  }>;
  start_time?: number;
  end_time?: number;
  [key: string]: unknown;
};

// --- Lookup helpers ---

function getRecordEntity<T extends Record<string, unknown>>(
  entry: { value?: unknown } | undefined,
): T | undefined {
  return entry?.value as T | undefined;
}

/** Extract a block from a RecordMap by ID. */
export function getBlock(recordMap: RecordMap, id: string): V3Block | undefined {
  return getRecordEntity<V3Block>(recordMap.block?.[id]);
}

/** Extract a collection from a RecordMap by ID. */
export function getCollection(recordMap: RecordMap, id: string): V3Collection | undefined {
  return getRecordEntity<V3Collection>(recordMap.collection?.[id]);
}

/** Get all blocks from a RecordMap. */
export function getAllBlocks(recordMap: RecordMap): V3Block[] {
  if (!recordMap.block) return [];
  return Object.values(recordMap.block)
    .map((entry) => getRecordEntity<V3Block>(entry))
    .filter((block): block is V3Block => Boolean(block && block.alive !== false));
}

/** Get the first collection from a RecordMap. */
export function getFirstCollection(recordMap: RecordMap): V3Collection | undefined {
  if (!recordMap.collection) return undefined;
  const entries = Object.values(recordMap.collection);
  return getRecordEntity<V3Collection>(entries[0]);
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
  return getRecordEntity<V3User>(entries[0]);
}

/** Get the first space from a RecordMap. */
export function getFirstSpace(recordMap: RecordMap): V3Space | undefined {
  if (!recordMap.space) return undefined;
  const entries = Object.values(recordMap.space);
  return getRecordEntity<V3Space>(entries[0]);
}

/** Get all users from a RecordMap. */
export function getAllUsers(recordMap: RecordMap): V3User[] {
  if (!recordMap.notion_user) return [];
  return Object.values(recordMap.notion_user)
    .map((entry) => getRecordEntity<V3User>(entry))
    .filter((user): user is V3User => Boolean(user));
}

/** Extract a discussion from a RecordMap by ID. */
export function getDiscussion(recordMap: RecordMap, id: string): V3Discussion | undefined {
  return getRecordEntity<V3Discussion>(recordMap.discussion?.[id]);
}

/** Extract a comment from a RecordMap by ID. */
export function getComment(recordMap: RecordMap, id: string): V3Comment | undefined {
  return getRecordEntity<V3Comment>(recordMap.comment?.[id]);
}

/** Extract a user from a RecordMap by ID. */
export function getUser(recordMap: RecordMap, id: string): V3User | undefined {
  return getRecordEntity<V3User>(recordMap.notion_user?.[id]);
}

/** Merge records from a secondary RecordMap into a primary one. */
export function mergeRecordMap(target: RecordMap, source: RecordMap): void {
  for (const table of Object.keys(source)) {
    const sourceTable = source[table];
    if (!sourceTable) continue;
    if (!target[table]) {
      (target as Record<string, unknown>)[table] = {};
    }
    Object.assign(target[table]!, sourceTable);
  }
}
