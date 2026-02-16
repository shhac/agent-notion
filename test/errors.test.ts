import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { CliError, handleAction } from "../src/lib/errors.ts";

describe("CliError", () => {
  test("creates error with message", () => {
    const err = new CliError("something went wrong");
    expect(err.message).toBe("something went wrong");
    expect(err.name).toBe("CliError");
    expect(err).toBeInstanceOf(Error);
  });
});

describe("handleAction", () => {
  let stderrOutput: string;
  const originalError = console.error;

  beforeEach(() => {
    stderrOutput = "";
    console.error = (...args: unknown[]) => {
      stderrOutput += args.map(String).join(" ");
    };
    process.exitCode = 0;
  });

  afterEach(() => {
    console.error = originalError;
    process.exitCode = 0;
  });

  test("handles CliError with JSON error output", async () => {
    await handleAction(async () => {
      throw new CliError("Page not found. Share the page with your integration.");
    });
    expect(JSON.parse(stderrOutput)).toEqual({
      error: "Page not found. Share the page with your integration.",
    });
    expect(process.exitCode).toBe(1);
  });

  test("handles Notion API unauthorized error", async () => {
    await handleAction(async () => {
      throw { status: 401, code: "unauthorized", message: "API token is invalid." };
    });
    expect(JSON.parse(stderrOutput)).toEqual({
      error: "Not authenticated. Set NOTION_API_KEY env var or run 'agent-notion config set notion.apiKey <key>'.",
    });
  });

  test("handles Notion API not found error", async () => {
    await handleAction(async () => {
      throw { status: 404, code: "object_not_found", message: "Could not find object." };
    });
    expect(JSON.parse(stderrOutput)).toEqual({
      error: "Not found. The integration may not have access. Share the resource with your integration in Notion.",
    });
  });

  test("handles Notion API rate limit error", async () => {
    await handleAction(async () => {
      throw { status: 429, code: "rate_limited", message: "Rate limited." };
    });
    expect(JSON.parse(stderrOutput)).toEqual({
      error: "Rate limited by Notion API. Wait a moment and retry.",
    });
  });

  test("handles Notion API validation error", async () => {
    await handleAction(async () => {
      throw { status: 400, code: "validation_error", message: "Invalid filter property." };
    });
    expect(JSON.parse(stderrOutput)).toEqual({
      error: "Notion API validation error: Invalid filter property.",
    });
  });

  test("handles generic errors", async () => {
    await handleAction(async () => {
      throw new Error("unexpected failure");
    });
    expect(JSON.parse(stderrOutput)).toEqual({
      error: "unexpected failure",
    });
  });

  test("handles non-Error throws", async () => {
    await handleAction(async () => {
      throw "string error";
    });
    expect(JSON.parse(stderrOutput)).toEqual({
      error: "string error",
    });
  });

  test("does not catch on success", async () => {
    await handleAction(async () => {
      // no error
    });
    expect(stderrOutput).toBe("");
    expect(process.exitCode).toBe(0);
  });
});
