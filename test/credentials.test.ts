import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

const originalXdg = process.env["XDG_CONFIG_HOME"];
const originalNotionApiKey = process.env["NOTION_API_KEY"];
const originalNotionToken = process.env["NOTION_TOKEN"];
let tempDir: string;

beforeEach(() => {
  tempDir = mkdtempSync(join(tmpdir(), "agent-notion-cred-test-"));
  process.env["XDG_CONFIG_HOME"] = tempDir;
  delete process.env["NOTION_API_KEY"];
  delete process.env["NOTION_TOKEN"];
});

afterEach(() => {
  if (originalXdg !== undefined) {
    process.env["XDG_CONFIG_HOME"] = originalXdg;
  } else {
    delete process.env["XDG_CONFIG_HOME"];
  }
  if (originalNotionApiKey !== undefined) {
    process.env["NOTION_API_KEY"] = originalNotionApiKey;
  }
  if (originalNotionToken !== undefined) {
    process.env["NOTION_TOKEN"] = originalNotionToken;
  }
  try {
    rmSync(tempDir, { recursive: true });
  } catch {
    /* ignore */
  }
});

import { resolveAccessToken, getAccessToken } from "../src/lib/credentials.ts";
import { storeWorkspace } from "../src/lib/config.ts";

describe("resolveAccessToken", () => {
  test("returns undefined when no credentials exist", () => {
    expect(resolveAccessToken()).toBeUndefined();
  });

  test("env var NOTION_API_KEY takes highest priority", () => {
    process.env["NOTION_API_KEY"] = "env-key-123";

    const result = resolveAccessToken();
    expect(result).toBeDefined();
    expect(result!.key).toBe("env-key-123");
    expect(result!.source).toBe("environment");
  });

  test("env var NOTION_TOKEN works as fallback", () => {
    process.env["NOTION_TOKEN"] = "env-token-456";

    const result = resolveAccessToken();
    expect(result).toBeDefined();
    expect(result!.key).toBe("env-token-456");
    expect(result!.source).toBe("environment");
  });

  test("NOTION_API_KEY takes priority over NOTION_TOKEN", () => {
    process.env["NOTION_API_KEY"] = "api-key";
    process.env["NOTION_TOKEN"] = "token";

    const result = resolveAccessToken();
    expect(result!.key).toBe("api-key");
  });

  test("resolves from default workspace config", () => {
    // Store a workspace — on non-macOS, token will be plaintext in config
    storeWorkspace("test-ws", {
      workspace_id: "ws-1",
      workspace_name: "Test",
      bot_id: "bot-1",
      auth_type: "internal_integration",
      access_token: "ntn_config_token",
    });

    const result = resolveAccessToken();
    expect(result).toBeDefined();
    expect(result!.workspace).toBe("test-ws");
    expect(result!.auth_type).toBe("internal_integration");
    // Source will be either "keychain" or "config" depending on platform
    expect(["keychain", "config"]).toContain(result!.source);
  });

  test("env var overrides workspace config", () => {
    storeWorkspace("test-ws", {
      workspace_id: "ws-1",
      workspace_name: "Test",
      bot_id: "bot-1",
      auth_type: "internal_integration",
      access_token: "ntn_config_token",
    });
    process.env["NOTION_API_KEY"] = "env-override";

    const result = resolveAccessToken();
    expect(result!.key).toBe("env-override");
    expect(result!.source).toBe("environment");
  });
});

describe("getAccessToken", () => {
  test("returns undefined when not authenticated", () => {
    expect(getAccessToken()).toBeUndefined();
  });

  test("returns token string when authenticated", () => {
    process.env["NOTION_API_KEY"] = "test-key";
    expect(getAccessToken()).toBe("test-key");
  });
});
