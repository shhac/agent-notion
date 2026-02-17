/**
 * Normalize a Notion ID — accept both dashless (from URLs) and dashed (standard UUID) formats.
 *
 * Examples:
 *   "30a61d9c1112802f95fef30d3a601ec5" → "30a61d9c-1112-802f-95fe-f30d3a601ec5"
 *   "30a61d9c-1112-802f-95fe-f30d3a601ec5" → "30a61d9c-1112-802f-95fe-f30d3a601ec5"
 */
export function normalizeId(id: string): string {
  const cleaned = id.replace(/-/g, "").toLowerCase();
  if (cleaned.length !== 32 || !/^[0-9a-f]+$/.test(cleaned)) {
    return id; // Not a valid UUID hex string — return as-is and let the API reject it
  }
  return `${cleaned.slice(0, 8)}-${cleaned.slice(8, 12)}-${cleaned.slice(12, 16)}-${cleaned.slice(16, 20)}-${cleaned.slice(20)}`;
}
