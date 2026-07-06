/**
 * Synthetic v3 entity factories shared across test files.
 * Values are always placeholders — never real-world IDs or content.
 */
import type { V3Block, V3Collection, V3User, V3Comment } from "../../src/notion/v3/record-map.ts";

export function makeBlock(overrides: Partial<V3Block> & { id: string }): V3Block {
  return {
    type: "page",
    version: 1,
    created_time: 1700000000000,
    last_edited_time: 1700001000000,
    parent_id: "parent-1",
    parent_table: "block",
    alive: true,
    space_id: "space-1",
    ...overrides,
  };
}

export function makeCollection(overrides: Partial<V3Collection> & { id: string }): V3Collection {
  return {
    version: 1,
    name: [["Test DB"]],
    schema: {},
    parent_id: "parent-1",
    parent_table: "block",
    ...overrides,
  };
}

export function makeUser(overrides: Partial<V3User> & { id: string }): V3User {
  return {
    version: 1,
    email: "test@example.com",
    given_name: "Test",
    family_name: "User",
    ...overrides,
  };
}

export function makeComment(overrides: Partial<V3Comment> & { id: string }): V3Comment {
  return {
    version: 1,
    alive: true,
    parent_id: "disc-1",
    parent_table: "discussion",
    text: [["Hello"]],
    created_by_id: "user-1",
    created_by_table: "notion_user",
    created_time: 1700000000000,
    last_edited_time: 1700000000000,
    ...overrides,
  };
}

/** Wrap an entity in the normalized { value, role } RecordMap entry shape. */
export function wrapBlock(block: V3Block): { value: V3Block; role: string } {
  return { value: block, role: "reader" };
}
