import { describe, test, expect } from "bun:test";
import type {
  RecordMap,
  V3Block,
  V3Collection,
  V3User,
  V3Comment,
  V3PropertySchema,
} from "../src/notion/v3/client.ts";
import {
  v3RichTextToPlain,
  v3Parent,
  flattenV3PropertyValue,
  flattenV3Properties,
  normalizeV3Block,
  transformV3SearchResult,
  transformV3DatabaseListItem,
  transformV3DatabaseDetail,
  transformV3DatabaseSchema,
  transformV3QueryRow,
  transformV3PageDetail,
  transformV3User,
  transformV3UserMe,
  transformV3Comment,
  extractAnchorText,
  injectCommentDecoration,
  getBlock,
  getCollection,
  getAllBlocks,
  getFirstCollection,
  getFirstCollectionViewId,
  getFirstUser,
  getAllUsers,
  getDiscussion,
  getComment,
  getUser,
} from "../src/notion/v3/transforms.ts";

// --- Helpers ---

function makeBlock(overrides: Partial<V3Block> & { id: string }): V3Block {
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

function makeCollection(overrides: Partial<V3Collection> & { id: string }): V3Collection {
  return {
    version: 1,
    name: [["Test DB"]],
    schema: {},
    parent_id: "parent-1",
    parent_table: "block",
    ...overrides,
  };
}

function makeUser(overrides: Partial<V3User> & { id: string }): V3User {
  return {
    version: 1,
    email: "test@example.com",
    given_name: "Test",
    family_name: "User",
    ...overrides,
  };
}

function makeComment(overrides: Partial<V3Comment> & { id: string }): V3Comment {
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

// =============================================================================
// Rich text
// =============================================================================

describe("v3RichTextToPlain", () => {
  test("converts single segment", () => {
    expect(v3RichTextToPlain([["Hello"]])).toBe("Hello");
  });

  test("concatenates multiple segments", () => {
    expect(v3RichTextToPlain([["Hello "], ["world"]])).toBe("Hello world");
  });

  test("ignores decorations", () => {
    expect(v3RichTextToPlain([["bold", [["b"]]]])).toBe("bold");
  });

  test("returns empty string for undefined", () => {
    expect(v3RichTextToPlain(undefined)).toBe("");
  });

  test("returns empty string for empty array", () => {
    expect(v3RichTextToPlain([])).toBe("");
  });
});

// =============================================================================
// Parent
// =============================================================================

describe("v3Parent", () => {
  test("collection → database", () => {
    expect(v3Parent("collection", "col-1")).toEqual({ type: "database", id: "col-1" });
  });

  test("block → page", () => {
    expect(v3Parent("block", "block-1")).toEqual({ type: "page", id: "block-1" });
  });

  test("space → workspace", () => {
    expect(v3Parent("space", "space-1")).toEqual({ type: "workspace", id: "space-1" });
  });

  test("unknown → undefined", () => {
    expect(v3Parent("something_else", "id-1")).toBeUndefined();
  });
});

// =============================================================================
// flattenV3PropertyValue
// =============================================================================

describe("flattenV3PropertyValue", () => {
  const schema = (type: string): V3PropertySchema => ({ name: "prop", type });

  test("title returns plain text", () => {
    expect(flattenV3PropertyValue([["My Title"]], schema("title"))).toBe("My Title");
  });

  test("text returns plain text", () => {
    expect(flattenV3PropertyValue([["Some text"]], schema("text"))).toBe("Some text");
  });

  test("number returns numeric value", () => {
    expect(flattenV3PropertyValue([["42"]], schema("number"))).toBe(42);
  });

  test("number returns null for empty", () => {
    expect(flattenV3PropertyValue(undefined, schema("number"))).toBeNull();
  });

  test("select returns string", () => {
    expect(flattenV3PropertyValue([["Option A"]], schema("select"))).toBe("Option A");
  });

  test("select returns null for empty", () => {
    expect(flattenV3PropertyValue([[""]],  schema("select"))).toBeNull();
  });

  test("multi_select splits by comma", () => {
    expect(flattenV3PropertyValue([["A,B,C"]], schema("multi_select"))).toEqual(["A", "B", "C"]);
  });

  test("multi_select returns empty array for empty", () => {
    expect(flattenV3PropertyValue(undefined, schema("multi_select"))).toEqual([]);
  });

  test("status returns string", () => {
    expect(flattenV3PropertyValue([["Done"]], schema("status"))).toBe("Done");
  });

  test("status returns null for empty", () => {
    expect(flattenV3PropertyValue([[""]],  schema("status"))).toBeNull();
  });

  test("checkbox Yes → true", () => {
    expect(flattenV3PropertyValue([["Yes"]], schema("checkbox"))).toBe(true);
  });

  test("checkbox No → false", () => {
    expect(flattenV3PropertyValue([["No"]], schema("checkbox"))).toBe(false);
  });

  test("checkbox undefined → false", () => {
    expect(flattenV3PropertyValue(undefined, schema("checkbox"))).toBe(false);
  });

  test("url returns string", () => {
    expect(flattenV3PropertyValue([["https://example.com"]], schema("url"))).toBe("https://example.com");
  });

  test("url returns null for empty", () => {
    expect(flattenV3PropertyValue(undefined, schema("url"))).toBeNull();
  });

  test("email returns string", () => {
    expect(flattenV3PropertyValue([["a@b.com"]], schema("email"))).toBe("a@b.com");
  });

  test("phone_number returns string", () => {
    expect(flattenV3PropertyValue([["555-1234"]], schema("phone_number"))).toBe("555-1234");
  });

  test("date with decoration extracts start and end", () => {
    const value = [["‣", [["d", { start_date: "2024-01-15", end_date: "2024-01-20" }]]]] as any;
    expect(flattenV3PropertyValue(value, schema("date"))).toEqual({
      start: "2024-01-15",
      end: "2024-01-20",
    });
  });

  test("date with decoration and no end_date", () => {
    const value = [["‣", [["d", { start_date: "2024-01-15" }]]]] as any;
    expect(flattenV3PropertyValue(value, schema("date"))).toEqual({
      start: "2024-01-15",
      end: null,
    });
  });

  test("date with no decoration falls back to text", () => {
    const value = [["2024-01-15"]] as any;
    expect(flattenV3PropertyValue(value, schema("date"))).toEqual({
      start: "2024-01-15",
      end: null,
    });
  });

  test("date returns null for empty", () => {
    expect(flattenV3PropertyValue(undefined, schema("date"))).toBeNull();
  });

  test("person extracts user IDs from decorations", () => {
    const value = [
      ["‣", [["u", "user-1"]]],
      ["‣", [["u", "user-2"]]],
    ] as any;
    expect(flattenV3PropertyValue(value, schema("person"))).toEqual([
      { id: "user-1" },
      { id: "user-2" },
    ]);
  });

  test("person returns empty array for empty", () => {
    expect(flattenV3PropertyValue(undefined, schema("person"))).toEqual([]);
  });

  test("people type works same as person", () => {
    const value = [["‣", [["u", "user-1"]]]] as any;
    expect(flattenV3PropertyValue(value, schema("people"))).toEqual([{ id: "user-1" }]);
  });

  test("relation extracts page IDs from decorations", () => {
    const value = [
      ["‣", [["p", "page-1"]]],
      ["‣", [["p", "page-2"]]],
    ] as any;
    expect(flattenV3PropertyValue(value, schema("relation"))).toEqual([
      { id: "page-1" },
      { id: "page-2" },
    ]);
  });

  test("relation returns empty array for empty", () => {
    expect(flattenV3PropertyValue(undefined, schema("relation"))).toEqual([]);
  });

  test("created_time returns text", () => {
    expect(flattenV3PropertyValue([["1700000000000"]], schema("created_time"))).toBe("1700000000000");
  });

  test("last_edited_time returns text", () => {
    expect(flattenV3PropertyValue([["1700000000000"]], schema("last_edited_time"))).toBe("1700000000000");
  });

  test("created_by returns id object", () => {
    expect(flattenV3PropertyValue([["user-1"]], schema("created_by"))).toEqual({ id: "user-1" });
  });

  test("last_edited_by returns id object", () => {
    expect(flattenV3PropertyValue([["user-1"]], schema("last_edited_by"))).toEqual({ id: "user-1" });
  });

  test("formula returns text", () => {
    expect(flattenV3PropertyValue([["42"]], schema("formula"))).toBe("42");
  });

  test("rollup returns text", () => {
    expect(flattenV3PropertyValue([["100"]], schema("rollup"))).toBe("100");
  });

  test("files returns array with name", () => {
    expect(flattenV3PropertyValue([["file.pdf"]], schema("files"))).toEqual([
      { name: "file.pdf", url: null },
    ]);
  });

  test("files returns empty array for empty", () => {
    expect(flattenV3PropertyValue(undefined, schema("files"))).toEqual([]);
  });

  test("unique_id returns text", () => {
    expect(flattenV3PropertyValue([["PREFIX-42"]], schema("unique_id"))).toBe("PREFIX-42");
  });

  test("unknown type returns text", () => {
    expect(flattenV3PropertyValue([["hello"]], schema("custom_type"))).toBe("hello");
  });

  test("unknown type returns null for empty", () => {
    expect(flattenV3PropertyValue(undefined, schema("custom_type"))).toBeNull();
  });
});

// =============================================================================
// flattenV3Properties
// =============================================================================

describe("flattenV3Properties", () => {
  test("maps schema IDs to human-readable names", () => {
    const properties = {
      title: [["My Page"]] as any,
      abc1: [["Done"]] as any,
    };
    const schema: Record<string, V3PropertySchema> = {
      title: { name: "Name", type: "title" },
      abc1: { name: "Status", type: "status" },
    };

    const result = flattenV3Properties(properties, schema);
    expect(result).toEqual({
      Name: "My Page",
      Status: "Done",
    });
  });

  test("handles undefined properties", () => {
    const schema: Record<string, V3PropertySchema> = {
      abc1: { name: "Status", type: "status" },
    };

    const result = flattenV3Properties(undefined, schema);
    expect(result).toEqual({});
  });

  test("returns default values for missing property values", () => {
    const schema: Record<string, V3PropertySchema> = {
      title: { name: "Name", type: "title" },
      abc1: { name: "Tags", type: "multi_select" },
    };

    // Only title is present in properties
    const result = flattenV3Properties({ title: [["Hello"]] as any }, schema);
    expect(result.Name).toBe("Hello");
    expect(result.Tags).toEqual([]); // multi_select default for undefined
  });
});

// =============================================================================
// normalizeV3Block
// =============================================================================

describe("normalizeV3Block", () => {
  test("normalizes a text block to paragraph", () => {
    const block = makeBlock({
      id: "b1",
      type: "text",
      properties: { title: [["Hello world"]] },
    });
    const result = normalizeV3Block(block);
    expect(result.id).toBe("b1");
    expect(result.type).toBe("paragraph");
    expect(result.richText).toBe("Hello world");
    expect(result.hasChildren).toBe(false);
  });

  test("normalizes header types", () => {
    expect(normalizeV3Block(makeBlock({ id: "b", type: "header", properties: { title: [["H1"]] } })).type).toBe("heading_1");
    expect(normalizeV3Block(makeBlock({ id: "b", type: "sub_header", properties: { title: [["H2"]] } })).type).toBe("heading_2");
    expect(normalizeV3Block(makeBlock({ id: "b", type: "sub_sub_header", properties: { title: [["H3"]] } })).type).toBe("heading_3");
  });

  test("detects hasChildren from content array", () => {
    const block = makeBlock({ id: "b1", type: "page", content: ["child-1", "child-2"] });
    expect(normalizeV3Block(block).hasChildren).toBe(true);
  });

  test("handles to_do block with checked", () => {
    const block = makeBlock({
      id: "b1",
      type: "to_do",
      properties: { title: [["Task"]], checked: [["Yes"]] },
    });
    const result = normalizeV3Block(block);
    expect(result.type).toBe("to_do");
    expect(result.checked).toBe(true);
  });

  test("handles to_do block unchecked", () => {
    const block = makeBlock({
      id: "b1",
      type: "to_do",
      properties: { title: [["Task"]] },
    });
    expect(normalizeV3Block(block).checked).toBe(false);
  });

  test("handles code block with language", () => {
    const block = makeBlock({
      id: "b1",
      type: "code",
      properties: { title: [["const x = 1"]], language: [["typescript"]] },
    });
    const result = normalizeV3Block(block);
    expect(result.type).toBe("code");
    expect(result.language).toBe("typescript");
    expect(result.richText).toBe("const x = 1");
  });

  test("handles image block with format.display_source", () => {
    const block = makeBlock({
      id: "b1",
      type: "image",
      format: { display_source: "https://example.com/img.png" },
      properties: { source: [["https://fallback.com/img.png"]], caption: [["A caption"]] },
    });
    const result = normalizeV3Block(block);
    expect(result.type).toBe("image");
    expect(result.url).toBe("https://example.com/img.png");
    expect(result.caption).toBe("A caption");
  });

  test("handles image block falling back to properties.source", () => {
    const block = makeBlock({
      id: "b1",
      type: "image",
      properties: { source: [["https://fallback.com/img.png"]] },
    });
    expect(normalizeV3Block(block).url).toBe("https://fallback.com/img.png");
  });

  test("handles bookmark block", () => {
    const block = makeBlock({
      id: "b1",
      type: "bookmark",
      properties: { link: [["https://example.com"]], description: [["A site"]] },
    });
    const result = normalizeV3Block(block);
    expect(result.type).toBe("bookmark");
    expect(result.url).toBe("https://example.com");
    expect(result.caption).toBe("A site");
  });

  test("handles equation block", () => {
    const block = makeBlock({
      id: "b1",
      type: "equation",
      properties: { title: [["E = mc^2"]] },
    });
    const result = normalizeV3Block(block);
    expect(result.type).toBe("equation");
    expect(result.expression).toBe("E = mc^2");
  });

  test("handles page block with title", () => {
    const block = makeBlock({
      id: "b1",
      type: "page",
      properties: { title: [["Child Page"]] },
    });
    const result = normalizeV3Block(block);
    expect(result.type).toBe("child_page");
    expect(result.title).toBe("Child Page");
  });

  test("handles collection_view_page block", () => {
    const block = makeBlock({
      id: "b1",
      type: "collection_view_page",
      properties: { title: [["My DB"]] },
    });
    const result = normalizeV3Block(block);
    expect(result.type).toBe("child_database");
    expect(result.title).toBe("My DB");
  });

  test("handles callout block with emoji", () => {
    const block = makeBlock({
      id: "b1",
      type: "callout",
      properties: { title: [["Note"]] },
      format: { page_icon: "💡" },
    });
    const result = normalizeV3Block(block);
    expect(result.type).toBe("callout");
    expect(result.emoji).toBe("💡");
  });

  test("handles embed block", () => {
    const block = makeBlock({
      id: "b1",
      type: "embed",
      properties: { source: [["https://youtube.com/embed/abc"]] },
    });
    expect(normalizeV3Block(block).url).toBe("https://youtube.com/embed/abc");
  });

  test("handles video block", () => {
    const block = makeBlock({
      id: "b1",
      type: "video",
      properties: { source: [["https://example.com/video.mp4"]], caption: [["Video"]] },
    });
    const result = normalizeV3Block(block);
    expect(result.type).toBe("video");
    expect(result.url).toBe("https://example.com/video.mp4");
    expect(result.caption).toBe("Video");
  });

  test("passes through unknown block type", () => {
    const block = makeBlock({ id: "b1", type: "fancy_new_block" });
    expect(normalizeV3Block(block).type).toBe("fancy_new_block");
  });
});

// =============================================================================
// transformV3SearchResult
// =============================================================================

describe("transformV3SearchResult", () => {
  test("transforms page block", () => {
    const block = makeBlock({
      id: "page-1",
      type: "page",
      properties: { title: [["My Page"]] },
      parent_id: "parent-1",
      parent_table: "block",
    });
    const result = transformV3SearchResult(block);
    expect(result.id).toBe("page-1");
    expect(result.type).toBe("page");
    expect(result.title).toBe("My Page");
    expect(result.url).toBe("https://www.notion.so/page1");
    expect(result.parent).toEqual({ type: "page", id: "parent-1" });
    expect(result.lastEditedAt).toBe("2023-11-14T22:30:00.000Z");
  });

  test("transforms collection_view_page as database", () => {
    const block = makeBlock({
      id: "db-1",
      type: "collection_view_page",
      properties: { title: [["My DB"]] },
    });
    expect(transformV3SearchResult(block).type).toBe("database");
  });

  test("transforms collection_view as database", () => {
    const block = makeBlock({
      id: "db-2",
      type: "collection_view",
      properties: { title: [["Inline DB"]] },
    });
    expect(transformV3SearchResult(block).type).toBe("database");
  });
});

// =============================================================================
// transformV3DatabaseListItem
// =============================================================================

describe("transformV3DatabaseListItem", () => {
  test("transforms collection with view page ID", () => {
    const collection = makeCollection({
      id: "col-1",
      name: [["Tasks"]],
      schema: {
        title: { name: "Name", type: "title" },
        abc1: { name: "Status", type: "status" },
      },
      parent_id: "parent-1",
      parent_table: "block",
    });
    const result = transformV3DatabaseListItem(collection, "view-page-1");
    expect(result.id).toBe("view-page-1");
    expect(result.title).toBe("Tasks");
    expect(result.propertyCount).toBe(2);
    expect(result.url).toContain("viewpage1");
  });

  test("falls back to parent_id when no view page ID", () => {
    const collection = makeCollection({
      id: "col-1",
      parent_id: "parent-1",
    });
    const result = transformV3DatabaseListItem(collection);
    expect(result.id).toBe("parent-1");
  });

  test("handles empty schema", () => {
    const collection = makeCollection({ id: "col-1" });
    expect(transformV3DatabaseListItem(collection).propertyCount).toBe(0);
  });
});

// =============================================================================
// transformV3DatabaseDetail
// =============================================================================

describe("transformV3DatabaseDetail", () => {
  test("transforms collection with schema properties", () => {
    const collection = makeCollection({
      id: "col-1",
      name: [["Projects"]],
      description: [["All projects"]],
      schema: {
        title: { name: "Name", type: "title" },
        abc1: {
          name: "Status",
          type: "select",
          options: [
            { id: "opt-1", value: "Active", color: "green" },
            { id: "opt-2", value: "Done", color: "gray" },
          ],
        },
      },
    });
    const result = transformV3DatabaseDetail(collection, "view-page-1");
    expect(result.id).toBe("view-page-1");
    expect(result.title).toBe("Projects");
    expect(result.description).toBe("All projects");
    expect(result.properties.Name).toEqual({ id: "title", type: "title" });
    expect(result.properties.Status!.options).toEqual([
      { name: "Active", color: "green" },
      { name: "Done", color: "gray" },
    ]);
  });

  test("includes groups when present", () => {
    const collection = makeCollection({
      id: "col-1",
      schema: {
        abc1: {
          name: "Status",
          type: "status",
          options: [
            { id: "opt-1", value: "Todo", color: "gray" },
            { id: "opt-2", value: "In Progress", color: "blue" },
            { id: "opt-3", value: "Done", color: "green" },
          ],
          groups: [
            { id: "g1", name: "Not Started", optionIds: ["opt-1"] },
            { id: "g2", name: "Active", optionIds: ["opt-2"] },
          ],
        },
      },
    });
    const result = transformV3DatabaseDetail(collection);
    expect(result.properties.Status!.groups).toEqual([
      { name: "Not Started", options: ["Todo"] },
      { name: "Active", options: ["In Progress"] },
    ]);
  });

  test("includes relatedDatabase for relation properties", () => {
    const collection = makeCollection({
      id: "col-1",
      schema: {
        abc1: { name: "Related", type: "relation", collection_id: "col-2" },
      },
    });
    expect(transformV3DatabaseDetail(collection).properties.Related!.relatedDatabase).toBe("col-2");
  });
});

// =============================================================================
// transformV3DatabaseSchema
// =============================================================================

describe("transformV3DatabaseSchema", () => {
  test("transforms schema to property list", () => {
    const collection = makeCollection({
      id: "col-1",
      name: [["My DB"]],
      schema: {
        title: { name: "Name", type: "title" },
        abc1: {
          name: "Tags",
          type: "multi_select",
          options: [
            { id: "opt-1", value: "Frontend" },
            { id: "opt-2", value: "Backend" },
          ],
        },
      },
    });
    const result = transformV3DatabaseSchema(collection, "view-1");
    expect(result.id).toBe("view-1");
    expect(result.title).toBe("My DB");
    expect(result.properties).toHaveLength(2);

    const nameP = result.properties.find((p) => p.name === "Name");
    expect(nameP?.type).toBe("title");
    expect(nameP?.id).toBe("title");

    const tagsP = result.properties.find((p) => p.name === "Tags");
    expect(tagsP?.options).toEqual(["Frontend", "Backend"]);
  });

  test("includes groups as record", () => {
    const collection = makeCollection({
      id: "col-1",
      schema: {
        abc1: {
          name: "Status",
          type: "status",
          options: [
            { id: "o1", value: "Todo" },
            { id: "o2", value: "Done" },
          ],
          groups: [
            { id: "g1", name: "Open", optionIds: ["o1"] },
            { id: "g2", name: "Closed", optionIds: ["o2"] },
          ],
        },
      },
    });
    const result = transformV3DatabaseSchema(collection);
    const statusP = result.properties[0]!;
    expect(statusP.groups).toEqual({
      Open: ["Todo"],
      Closed: ["Done"],
    });
  });
});

// =============================================================================
// transformV3QueryRow
// =============================================================================

describe("transformV3QueryRow", () => {
  test("transforms block row with schema", () => {
    const block = makeBlock({
      id: "row-1",
      type: "page",
      properties: {
        title: [["Task 1"]],
        abc1: [["Done"]],
      },
    });
    const schema: Record<string, V3PropertySchema> = {
      title: { name: "Name", type: "title" },
      abc1: { name: "Status", type: "status" },
    };
    const result = transformV3QueryRow(block, schema);
    expect(result.id).toBe("row-1");
    expect(result.url).toBe("https://www.notion.so/row1");
    expect(result.properties).toEqual({ Name: "Task 1", Status: "Done" });
    expect(result.createdAt).toBe("2023-11-14T22:13:20.000Z");
    expect(result.lastEditedAt).toBe("2023-11-14T22:30:00.000Z");
  });
});

// =============================================================================
// transformV3PageDetail
// =============================================================================

describe("transformV3PageDetail", () => {
  test("transforms page without schema", () => {
    const block = makeBlock({
      id: "page-1",
      type: "page",
      properties: { title: [["My Page"]] },
      parent_table: "block",
      parent_id: "parent-1",
      alive: true,
    });
    const result = transformV3PageDetail(block);
    expect(result.id).toBe("page-1");
    expect(result.properties).toEqual({ title: "My Page" });
    expect(result.parent).toEqual({ type: "page", id: "parent-1" });
    expect(result.archived).toBe(false);
    expect(result.createdBy).toBeNull();
    expect(result.lastEditedBy).toBeNull();
  });

  test("transforms page with schema", () => {
    const block = makeBlock({
      id: "row-1",
      parent_table: "collection",
      parent_id: "col-1",
      properties: { title: [["Row"]], abc1: [["Active"]] },
    });
    const schema: Record<string, V3PropertySchema> = {
      title: { name: "Name", type: "title" },
      abc1: { name: "Status", type: "status" },
    };
    const result = transformV3PageDetail(block, schema);
    expect(result.properties).toEqual({ Name: "Row", Status: "Active" });
    expect(result.parent).toEqual({ type: "database", id: "col-1" });
  });

  test("includes icon from format", () => {
    const block = makeBlock({
      id: "page-1",
      format: { page_icon: "🎯" },
    });
    const result = transformV3PageDetail(block);
    expect(result.icon).toEqual({ type: "emoji", emoji: "🎯" });
  });

  test("icon is null when no format", () => {
    const block = makeBlock({ id: "page-1" });
    expect(transformV3PageDetail(block).icon).toBeNull();
  });

  test("archived true when alive is false", () => {
    const block = makeBlock({ id: "page-1", alive: false });
    expect(transformV3PageDetail(block).archived).toBe(true);
  });
});

// =============================================================================
// transformV3User / transformV3UserMe
// =============================================================================

describe("transformV3User", () => {
  test("transforms user with full name", () => {
    const user = makeUser({ id: "u1", given_name: "Jane", family_name: "Doe", email: "jane@example.com", profile_photo: "https://photo.com/jane.jpg" });
    const result = transformV3User(user);
    expect(result).toEqual({
      id: "u1",
      name: "Jane Doe",
      type: "person",
      email: "jane@example.com",
      avatarUrl: "https://photo.com/jane.jpg",
    });
  });

  test("handles user with only given_name", () => {
    const user = makeUser({ id: "u1", given_name: "Jane", family_name: "" });
    expect(transformV3User(user).name).toBe("Jane");
  });

  test("handles user with no name", () => {
    const user = makeUser({ id: "u1", given_name: "", family_name: "" });
    expect(transformV3User(user).name).toBeUndefined();
  });
});

describe("transformV3UserMe", () => {
  test("transforms user with space name", () => {
    const user = makeUser({ id: "u1" });
    const result = transformV3UserMe(user, "My Workspace");
    expect(result.id).toBe("u1");
    expect(result.name).toBe("Test User");
    expect(result.type).toBe("person");
    expect(result.workspaceName).toBe("My Workspace");
  });

  test("handles no space name", () => {
    const user = makeUser({ id: "u1" });
    expect(transformV3UserMe(user).workspaceName).toBeUndefined();
  });
});

// =============================================================================
// transformV3Comment
// =============================================================================

describe("transformV3Comment", () => {
  test("transforms comment with user", () => {
    const comment = makeComment({ id: "c1", text: [["Great work!"]], created_by_id: "u1" });
    const user = makeUser({ id: "u1", given_name: "Jane", family_name: "Doe" });
    const result = transformV3Comment(comment, user);
    expect(result.id).toBe("c1");
    expect(result.body).toBe("Great work!");
    expect(result.author).toEqual({ id: "u1", name: "Jane Doe" });
    expect(result.createdAt).toBe("2023-11-14T22:13:20.000Z");
  });

  test("transforms comment without user (falls back to created_by_id)", () => {
    const comment = makeComment({ id: "c1", created_by_id: "u1" });
    const result = transformV3Comment(comment);
    expect(result.author).toEqual({ id: "u1" });
  });

  test("transforms comment with no author info", () => {
    const comment = makeComment({ id: "c1", created_by_id: "" });
    const result = transformV3Comment(comment);
    expect(result.author).toBeNull();
  });

  test("includes anchorText when provided", () => {
    const comment = makeComment({ id: "c1", created_by_id: "u1" });
    const result = transformV3Comment(comment, undefined, "highlighted text");
    expect(result.anchorText).toBe("highlighted text");
  });

  test("omits anchorText when not provided", () => {
    const comment = makeComment({ id: "c1", created_by_id: "u1" });
    const result = transformV3Comment(comment);
    expect(result.anchorText).toBeUndefined();
  });
});

// =============================================================================
// extractAnchorText
// =============================================================================

describe("extractAnchorText", () => {
  test("extracts text decorated with matching discussion ID", () => {
    const richText: any = [
      ["Hello "],
      ["world", [["m", "disc-1"]]],
      ["!"],
    ];
    expect(extractAnchorText(richText, "disc-1")).toBe("world");
  });

  test("concatenates multiple segments with same discussion ID", () => {
    const richText: any = [
      ["Hello "],
      ["beautiful", [["b"], ["m", "disc-1"]]],
      [" ", [["m", "disc-1"]]],
      ["world", [["m", "disc-1"]]],
      ["!"],
    ];
    expect(extractAnchorText(richText, "disc-1")).toBe("beautiful world");
  });

  test("returns undefined for non-matching discussion ID", () => {
    const richText: any = [
      ["Hello "],
      ["world", [["m", "disc-other"]]],
    ];
    expect(extractAnchorText(richText, "disc-1")).toBeUndefined();
  });

  test("returns undefined for undefined rich text", () => {
    expect(extractAnchorText(undefined, "disc-1")).toBeUndefined();
  });

  test("returns undefined for empty rich text", () => {
    expect(extractAnchorText([], "disc-1")).toBeUndefined();
  });

  test("returns undefined when no decorations present", () => {
    const richText: any = [["Hello world"]];
    expect(extractAnchorText(richText, "disc-1")).toBeUndefined();
  });
});

// =============================================================================
// RecordMap accessors
// =============================================================================

describe("RecordMap accessors", () => {
  const recordMap: RecordMap = {
    block: {
      "b1": { value: makeBlock({ id: "b1", alive: true }) as any, role: "reader" },
      "b2": { value: makeBlock({ id: "b2", alive: false }) as any, role: "reader" },
      "b3": { value: makeBlock({ id: "b3", alive: true }) as any, role: "reader" },
    },
    collection: {
      "col-1": { value: makeCollection({ id: "col-1" }) as any, role: "reader" },
    },
    collection_view: {
      "view-1": { value: { id: "view-1" } as any, role: "reader" },
    },
    notion_user: {
      "u1": { value: makeUser({ id: "u1" }) as any, role: "reader" },
      "u2": { value: makeUser({ id: "u2", given_name: "Jane" }) as any, role: "reader" },
    },
    discussion: {
      "d1": { value: { id: "d1", version: 1, parent_id: "b1", parent_table: "block", resolved: false, comments: ["c1"] } as any, role: "reader" },
    },
    comment: {
      "c1": { value: makeComment({ id: "c1" }) as any, role: "reader" },
    },
  };

  test("getBlock returns block by id", () => {
    expect(getBlock(recordMap, "b1")?.id).toBe("b1");
  });

  test("getBlock returns undefined for missing", () => {
    expect(getBlock(recordMap, "missing")).toBeUndefined();
  });

  test("getCollection returns collection by id", () => {
    expect(getCollection(recordMap, "col-1")?.id).toBe("col-1");
  });

  test("getAllBlocks filters out dead blocks", () => {
    const blocks = getAllBlocks(recordMap);
    expect(blocks).toHaveLength(2);
    expect(blocks.map((b) => b.id)).toEqual(expect.arrayContaining(["b1", "b3"]));
    expect(blocks.find((b) => b.id === "b2")).toBeUndefined();
  });

  test("getAllBlocks returns empty for missing block table", () => {
    expect(getAllBlocks({})).toEqual([]);
  });

  test("getFirstCollection returns first collection", () => {
    expect(getFirstCollection(recordMap)?.id).toBe("col-1");
  });

  test("getFirstCollection returns undefined for empty", () => {
    expect(getFirstCollection({})).toBeUndefined();
  });

  test("getFirstCollectionViewId returns first view id", () => {
    expect(getFirstCollectionViewId(recordMap)).toBe("view-1");
  });

  test("getFirstCollectionViewId returns undefined for empty", () => {
    expect(getFirstCollectionViewId({})).toBeUndefined();
  });

  test("getFirstUser returns first user", () => {
    expect(getFirstUser(recordMap)?.id).toBe("u1");
  });

  test("getFirstUser returns undefined for empty", () => {
    expect(getFirstUser({})).toBeUndefined();
  });

  test("getAllUsers returns all users", () => {
    const users = getAllUsers(recordMap);
    expect(users).toHaveLength(2);
  });

  test("getAllUsers returns empty for missing", () => {
    expect(getAllUsers({})).toEqual([]);
  });

  test("getDiscussion returns discussion by id", () => {
    expect(getDiscussion(recordMap, "d1")?.id).toBe("d1");
  });

  test("getComment returns comment by id", () => {
    expect(getComment(recordMap, "c1")?.id).toBe("c1");
  });

  test("getUser returns user by id", () => {
    expect(getUser(recordMap, "u1")?.id).toBe("u1");
  });
});

// =============================================================================
// injectCommentDecoration
// =============================================================================

describe("injectCommentDecoration", () => {
  test("injects decoration on found text", () => {
    const richText: any = [["Hello world"]];
    const result = injectCommentDecoration(richText, "world", "disc-1");
    expect(result).toHaveLength(2);
    expect(result[0]).toEqual(["Hello "]);
    expect(result[1]).toEqual(["world", [["m", "disc-1"]]]);
  });

  test("throws when text is not found", () => {
    const richText: any = [["Hello world"]];
    expect(() => injectCommentDecoration(richText, "missing", "disc-1")).toThrow(/not found/);
  });

  test("throws when target text is empty", () => {
    const richText: any = [["Hello"]];
    expect(() => injectCommentDecoration(richText, "", "disc-1")).toThrow(/cannot be empty/);
  });
});
