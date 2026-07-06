import { applyTruncation } from "./truncation.ts";

const DEFAULT_PAGE_SIZE = 50;

export function pruneEmpty<T>(value: T): T {
  const pruned = pruneEmptyInternal(value);
  return (pruned === undefined ? ({} as T) : (pruned as T)) as T;
}

function pruneEmptyInternal(value: unknown): unknown {
  if (value === null || value === undefined) {
    return undefined;
  }

  if (typeof value === "string") {
    return value.trim() === "" ? undefined : value;
  }

  if (typeof value === "number" || typeof value === "boolean") {
    return value;
  }

  if (Array.isArray(value)) {
    const next = value
      .map((v) => pruneEmptyInternal(v))
      .filter((v): v is Exclude<unknown, undefined> => v !== undefined);
    return next.length === 0 ? undefined : next;
  }

  if (typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      const next = pruneEmptyInternal(v);
      if (next !== undefined) {
        out[k] = next;
      }
    }
    return Object.keys(out).length === 0 ? undefined : out;
  }

  return value;
}

export function printJson(data: unknown): void {
  console.log(JSON.stringify(applyTruncation(pruneEmpty(data)), null, 2));
}

/**
 * Print paginated list output with { items, pagination? } wrapper.
 * Always returns { items: [...] } even when the array is empty.
 * Includes pagination when hasMore is true.
 */
export function printPaginated(
  items: unknown[],
  pageInfo: { hasMore: boolean; nextCursor?: string | null },
): void {
  const prunedItems = items.map((item) => applyTruncation(pruneEmpty(item)));
  const payload: Record<string, unknown> = { items: prunedItems };
  if (pageInfo.hasMore) {
    payload.pagination = {
      hasMore: true,
      nextCursor: pageInfo.nextCursor ?? null,
    };
  }
  console.log(JSON.stringify(payload, null, 2));
}

export function printError(message: string): void {
  console.error(JSON.stringify({ error: message }));
  process.exitCode = 1;
}

export function resolvePageSize(opts: { limit?: string }): number {
  if (opts.limit !== undefined) {
    const parsed = parseInt(opts.limit, 10);
    return Math.min(Math.max(parsed, 1), 100); // Notion API max is 100
  }
  return DEFAULT_PAGE_SIZE;
}
