import { describe, expect, test } from "bun:test";
import { execSync } from "node:child_process";

function getHelpOutput(command: string): string {
  try {
    return execSync(`bun run src/index.ts ${command} --help`, {
      encoding: "utf8",
      timeout: 10000,
    }).trim();
  } catch (e: unknown) {
    // Commander exits with code 0 for --help but execSync may still capture stdout
    const err = e as { stdout?: string };
    return err.stdout?.trim() ?? "";
  }
}

describe("CLI structure", () => {
  test("top-level --help lists all command groups", () => {
    const output = getHelpOutput("");
    const expected = [
      "auth",
      "search",
      "database",
      "page",
      "block",
      "comment",
      "user",
      "config",
      "usage",
    ];
    for (const cmd of expected) {
      expect(output).toContain(cmd);
    }
  });

  test("database --help lists subcommands", () => {
    const output = getHelpOutput("database");
    const expected = ["list", "get", "query", "schema", "usage"];
    for (const cmd of expected) {
      expect(output).toContain(cmd);
    }
  });

  test("page --help lists subcommands", () => {
    const output = getHelpOutput("page");
    const expected = ["get", "create", "update", "archive", "usage"];
    for (const cmd of expected) {
      expect(output).toContain(cmd);
    }
  });

  test("block --help lists subcommands", () => {
    const output = getHelpOutput("block");
    const expected = ["list", "append", "usage"];
    for (const cmd of expected) {
      expect(output).toContain(cmd);
    }
  });

  test("comment --help lists subcommands", () => {
    const output = getHelpOutput("comment");
    const expected = ["list", "page", "inline", "usage"];
    for (const cmd of expected) {
      expect(output).toContain(cmd);
    }
  });

  test("user --help lists subcommands", () => {
    const output = getHelpOutput("user");
    const expected = ["list", "me", "usage"];
    for (const cmd of expected) {
      expect(output).toContain(cmd);
    }
  });

  test("config --help lists subcommands", () => {
    const output = getHelpOutput("config");
    const expected = ["get", "set", "reset", "list-keys", "usage"];
    for (const cmd of expected) {
      expect(output).toContain(cmd);
    }
  });

  test("global --expand option is listed", () => {
    const output = getHelpOutput("");
    expect(output).toContain("--expand");
  });

  test("global --full option is listed", () => {
    const output = getHelpOutput("");
    expect(output).toContain("--full");
  });
});
