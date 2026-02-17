import { describe, test, expect } from "bun:test";
import {
  createBlockOps,
  archiveBlockOps,
  updatePropertyOps,
  officialBlockToV3Args,
} from "../src/notion/v3/operations.ts";

describe("createBlockOps", () => {
  test("creates set + listAfter + editMeta operations", () => {
    const ops = createBlockOps({
      id: "new-block-id",
      type: "page",
      parentId: "parent-id",
      parentTable: "block",
      spaceId: "space-id",
      userId: "user-id",
      properties: { title: [["Test"]] },
    });

    expect(ops).toHaveLength(3);

    // set operation
    expect(ops[0]!.command).toBe("set");
    expect(ops[0]!.pointer).toEqual({ table: "block", id: "new-block-id", spaceId: "space-id" });
    expect(ops[0]!.path).toEqual([]);
    const args = ops[0]!.args as Record<string, unknown>;
    expect(args.type).toBe("page");
    expect(args.id).toBe("new-block-id");
    expect(args.parent_id).toBe("parent-id");
    expect(args.parent_table).toBe("block");
    expect(args.alive).toBe(true);
    expect(args.space_id).toBe("space-id");
    expect(args.created_by_id).toBe("user-id");
    expect(args.properties).toEqual({ title: [["Test"]] });

    // listAfter operation
    expect(ops[1]!.command).toBe("listAfter");
    expect(ops[1]!.pointer).toEqual({ table: "block", id: "parent-id", spaceId: "space-id" });
    expect(ops[1]!.path).toEqual(["content"]);
    expect(ops[1]!.args).toEqual({ id: "new-block-id" });

    // editMeta operation
    expect(ops[2]!.command).toBe("update");
    expect(ops[2]!.pointer.id).toBe("parent-id");
    const metaArgs = ops[2]!.args as Record<string, unknown>;
    expect(metaArgs.last_edited_by_id).toBe("user-id");
    expect(metaArgs.last_edited_by_table).toBe("notion_user");
    expect(typeof metaArgs.last_edited_time).toBe("number");
  });

  test("omits properties and format when not provided", () => {
    const ops = createBlockOps({
      id: "id",
      type: "divider",
      parentId: "p",
      parentTable: "block",
      spaceId: "s",
      userId: "u",
    });

    const args = ops[0]!.args as Record<string, unknown>;
    expect(args.properties).toBeUndefined();
    expect(args.format).toBeUndefined();
  });

  test("includes format when provided", () => {
    const ops = createBlockOps({
      id: "id",
      type: "page",
      parentId: "p",
      parentTable: "block",
      spaceId: "s",
      userId: "u",
      format: { page_icon: "🎉" },
    });

    const args = ops[0]!.args as Record<string, unknown>;
    expect(args.format).toEqual({ page_icon: "🎉" });
  });

  test("collection parent uses block table for listAfter pointer", () => {
    const ops = createBlockOps({
      id: "id",
      type: "page",
      parentId: "collection-id",
      parentTable: "collection",
      spaceId: "s",
      userId: "u",
    });

    // The listAfter and editMeta should target "block" table, not "collection"
    expect(ops[1]!.pointer.table).toBe("block");
    expect(ops[2]!.pointer.table).toBe("block");
  });
});

describe("archiveBlockOps", () => {
  test("creates update + listRemove + editMeta operations", () => {
    const ops = archiveBlockOps({
      id: "block-id",
      parentId: "parent-id",
      parentTable: "block",
      spaceId: "space-id",
      userId: "user-id",
    });

    expect(ops).toHaveLength(3);

    // update with alive: false
    expect(ops[0]!.command).toBe("update");
    expect(ops[0]!.pointer.id).toBe("block-id");
    const args = ops[0]!.args as Record<string, unknown>;
    expect(args.alive).toBe(false);

    // listRemove from parent
    expect(ops[1]!.command).toBe("listRemove");
    expect(ops[1]!.pointer.id).toBe("parent-id");
    expect(ops[1]!.path).toEqual(["content"]);
    expect(ops[1]!.args).toEqual({ id: "block-id" });

    // editMeta on parent
    expect(ops[2]!.command).toBe("update");
    expect(ops[2]!.pointer.id).toBe("parent-id");
  });
});

describe("updatePropertyOps", () => {
  test("creates set ops for each property + editMeta", () => {
    const ops = updatePropertyOps({
      id: "page-id",
      spaceId: "space-id",
      userId: "user-id",
      properties: { title: [["New Title"]], abc1: [["Done"]] },
    });

    expect(ops).toHaveLength(3); // 2 properties + 1 editMeta

    expect(ops[0]!.command).toBe("set");
    expect(ops[0]!.path).toEqual(["properties", "title"]);
    expect(ops[0]!.args).toEqual([["New Title"]]);

    expect(ops[1]!.command).toBe("set");
    expect(ops[1]!.path).toEqual(["properties", "abc1"]);
    expect(ops[1]!.args).toEqual([["Done"]]);

    expect(ops[2]!.command).toBe("update");
  });

  test("creates set ops for format", () => {
    const ops = updatePropertyOps({
      id: "page-id",
      spaceId: "s",
      userId: "u",
      format: { page_icon: "🎯" },
    });

    expect(ops).toHaveLength(2); // 1 format + 1 editMeta
    expect(ops[0]!.command).toBe("set");
    expect(ops[0]!.path).toEqual(["format", "page_icon"]);
    expect(ops[0]!.args).toBe("🎯");
  });

  test("only produces editMeta when no properties or format", () => {
    const ops = updatePropertyOps({
      id: "page-id",
      spaceId: "s",
      userId: "u",
    });

    expect(ops).toHaveLength(1);
    expect(ops[0]!.command).toBe("update");
  });
});

describe("officialBlockToV3Args", () => {
  test("converts paragraph block", () => {
    const result = officialBlockToV3Args({
      type: "paragraph",
      paragraph: {
        rich_text: [{ type: "text", text: { content: "Hello world" } }],
      },
    });

    expect(result.type).toBe("text");
    expect(result.properties).toEqual({ title: [["Hello world"]] });
  });

  test("converts heading_1 block", () => {
    const result = officialBlockToV3Args({
      type: "heading_1",
      heading_1: {
        rich_text: [{ type: "text", text: { content: "Title" } }],
      },
    });

    expect(result.type).toBe("header");
    expect(result.properties).toEqual({ title: [["Title"]] });
  });

  test("converts heading_2 block", () => {
    const result = officialBlockToV3Args({
      type: "heading_2",
      heading_2: {
        rich_text: [{ type: "text", text: { content: "Subtitle" } }],
      },
    });

    expect(result.type).toBe("sub_header");
  });

  test("converts heading_3 block", () => {
    const result = officialBlockToV3Args({
      type: "heading_3",
      heading_3: {
        rich_text: [{ type: "text", text: { content: "Sub-subtitle" } }],
      },
    });

    expect(result.type).toBe("sub_sub_header");
  });

  test("converts bulleted_list_item block", () => {
    const result = officialBlockToV3Args({
      type: "bulleted_list_item",
      bulleted_list_item: {
        rich_text: [{ type: "text", text: { content: "Item" } }],
      },
    });

    expect(result.type).toBe("bulleted_list");
    expect(result.properties).toEqual({ title: [["Item"]] });
  });

  test("converts numbered_list_item block", () => {
    const result = officialBlockToV3Args({
      type: "numbered_list_item",
      numbered_list_item: {
        rich_text: [{ type: "text", text: { content: "Step 1" } }],
      },
    });

    expect(result.type).toBe("numbered_list");
  });

  test("converts to_do block with checked state", () => {
    const result = officialBlockToV3Args({
      type: "to_do",
      to_do: {
        rich_text: [{ type: "text", text: { content: "Task" } }],
        checked: true,
      },
    });

    expect(result.type).toBe("to_do");
    expect(result.properties).toEqual({ title: [["Task"]], checked: [["Yes"]] });
  });

  test("converts to_do block without checked state", () => {
    const result = officialBlockToV3Args({
      type: "to_do",
      to_do: {
        rich_text: [{ type: "text", text: { content: "Task" } }],
        checked: false,
      },
    });

    expect(result.type).toBe("to_do");
    expect(result.properties).toEqual({ title: [["Task"]] });
  });

  test("converts code block with language", () => {
    const result = officialBlockToV3Args({
      type: "code",
      code: {
        rich_text: [{ type: "text", text: { content: "const x = 1;" } }],
        language: "typescript",
      },
    });

    expect(result.type).toBe("code");
    expect(result.properties).toEqual({
      title: [["const x = 1;"]],
      language: [["typescript"]],
    });
  });

  test("converts quote block", () => {
    const result = officialBlockToV3Args({
      type: "quote",
      quote: {
        rich_text: [{ type: "text", text: { content: "Famous quote" } }],
      },
    });

    expect(result.type).toBe("quote");
    expect(result.properties).toEqual({ title: [["Famous quote"]] });
  });

  test("converts divider block", () => {
    const result = officialBlockToV3Args({
      type: "divider",
      divider: {},
    });

    expect(result.type).toBe("divider");
    expect(result.properties).toBeUndefined();
  });

  test("converts callout block with emoji", () => {
    const result = officialBlockToV3Args({
      type: "callout",
      callout: {
        rich_text: [{ type: "text", text: { content: "Note" } }],
        icon: { emoji: "💡" },
      },
    });

    expect(result.type).toBe("callout");
    expect(result.properties).toEqual({ title: [["Note"]] });
    expect(result.format).toEqual({ page_icon: "💡" });
  });

  test("converts bookmark block", () => {
    const result = officialBlockToV3Args({
      type: "bookmark",
      bookmark: {
        url: "https://example.com",
      },
    });

    expect(result.type).toBe("bookmark");
    expect(result.properties).toEqual({ link: [["https://example.com"]] });
  });

  test("converts image block with url", () => {
    const result = officialBlockToV3Args({
      type: "image",
      image: {
        url: "https://example.com/img.png",
      },
    });

    expect(result.type).toBe("image");
    expect(result.properties).toEqual({ source: [["https://example.com/img.png"]] });
  });

  test("converts equation block", () => {
    const result = officialBlockToV3Args({
      type: "equation",
      equation: {
        expression: "E = mc^2",
      },
    });

    expect(result.type).toBe("equation");
    expect(result.properties).toEqual({ title: [["E = mc^2"]] });
  });

  test("concatenates multiple rich_text segments", () => {
    const result = officialBlockToV3Args({
      type: "paragraph",
      paragraph: {
        rich_text: [
          { type: "text", text: { content: "Hello " } },
          { type: "text", text: { content: "world" } },
        ],
      },
    });

    expect(result.properties).toEqual({ title: [["Hello world"]] });
  });

  test("handles unknown block type gracefully", () => {
    const result = officialBlockToV3Args({
      type: "unknown_custom_type",
    });

    expect(result.type).toBe("unknown_custom_type");
    expect(result.properties).toBeUndefined();
  });
});
