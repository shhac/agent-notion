/**
 * Convention-based field truncation.
 *
 * Fields named "description", "body", or "content" are truncatable.
 * Truncated fields get a companion `{field}Length` showing full size.
 */

const TRUNCATABLE_FIELDS = new Set(["description", "body", "content"]);
const DEFAULT_MAX_LENGTH = 200;
const ELLIPSIS = "\u2026";

// Module-level state, configured once per CLI invocation
let expandedFields: Set<string> | "all" = new Set();
let configuredMaxLength: number = DEFAULT_MAX_LENGTH;

export function configureTruncation(opts: {
  expand?: string;
  full?: boolean;
  maxLength?: number;
}): void {
  if (opts.full) {
    expandedFields = "all";
  } else if (opts.expand) {
    expandedFields = new Set(opts.expand.split(",").map((s) => s.trim().toLowerCase()));
  } else {
    expandedFields = new Set();
  }
  configuredMaxLength = opts.maxLength ?? DEFAULT_MAX_LENGTH;
}

function shouldExpand(fieldName: string): boolean {
  return expandedFields === "all" || expandedFields.has(fieldName);
}

function truncateString(value: string, maxLength: number): string {
  if (value.length <= maxLength) {
    return value;
  }
  return `${value.slice(0, maxLength)}${ELLIPSIS}`;
}

/**
 * Recursively walk data, truncating truncatable fields and adding
 * companion length fields. Works on objects and arrays.
 */
export function applyTruncation(data: unknown): unknown {
  if (data === null || data === undefined) {
    return data;
  }

  if (Array.isArray(data)) {
    return data.map((item) => applyTruncation(item));
  }

  if (typeof data === "object") {
    const obj = data as Record<string, unknown>;
    const result: Record<string, unknown> = {};

    for (const [key, value] of Object.entries(obj)) {
      if (TRUNCATABLE_FIELDS.has(key) && typeof value === "string") {
        result[`${key}Length`] = value.length;
        result[key] = shouldExpand(key) ? value : truncateString(value, configuredMaxLength);
      } else if (typeof value === "object" && value !== null) {
        result[key] = applyTruncation(value);
      } else {
        result[key] = value;
      }
    }

    return result;
  }

  return data;
}
