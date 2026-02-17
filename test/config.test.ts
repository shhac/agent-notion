import { describe, test, expect, beforeEach, afterEach, mock } from "bun:test";
import { mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

// Mock keychain with in-memory store to avoid touching real macOS keychain
const mockStore = new Map<string, string>();
mock.module("../src/lib/keychain.ts", () => ({
  KEYCHAIN_SERVICE: "app.paulie.agent-notion",
  KEYCHAIN_PLACEHOLDER: "__KEYCHAIN__",
  keychainGet: (account: string, _service: string) => mockStore.get(account) ?? null,
  keychainSet: (opts: { account: string; value: string; service: string }) => {
    mockStore.set(opts.account, opts.value);
    return true;
  },
  keychainDelete: (account: string, _service: string) => mockStore.delete(account),
  keychainDeleteAll: (_service: string) => {
    mockStore.clear();
  },
}));

// Override XDG_CONFIG_HOME to use a temp directory for tests
const originalXdg = process.env["XDG_CONFIG_HOME"];
let tempDir: string;

beforeEach(() => {
  tempDir = mkdtempSync(join(tmpdir(), "agent-notion-test-"));
  process.env["XDG_CONFIG_HOME"] = tempDir;
  mockStore.clear();
});

afterEach(() => {
  if (originalXdg !== undefined) {
    process.env["XDG_CONFIG_HOME"] = originalXdg;
  } else {
    delete process.env["XDG_CONFIG_HOME"];
  }
  try {
    rmSync(tempDir, { recursive: true });
  } catch {
    /* ignore */
  }
});

// Re-import after env is set — Bun caches modules so we need to use dynamic import
// and access the functions through the module. However, since config.ts reads env
// at call time (not module load time), static imports work fine here.
import {
  readConfig,
  writeConfig,
  storeOAuthConfig,
  getOAuthConfig,
  storeWorkspace,
  getWorkspace,
  getWorkspaces,
  getDefaultWorkspace,
  removeWorkspace,
  setDefaultWorkspace,
  clearAll,
  deriveAlias,
  updateWorkspaceTokens,
  clearWorkspaceTokens,
  storeV3Session,
  getV3Session,
  resolveV3Token,
  clearV3Session,
} from "../src/lib/config.ts";

describe("config read/write", () => {
  test("readConfig returns empty object when no config file exists", () => {
    const config = readConfig();
    expect(config).toEqual({});
  });

  test("writeConfig + readConfig roundtrip", () => {
    const config = { default_workspace: "test" };
    writeConfig(config);
    const read = readConfig();
    expect(read.default_workspace).toBe("test");
  });

  test("writeConfig cleans up empty workspaces object", () => {
    writeConfig({ workspaces: {} });
    const read = readConfig();
    expect(read.workspaces).toBeUndefined();
  });
});

describe("OAuth config", () => {
  test("getOAuthConfig returns undefined when not configured", () => {
    expect(getOAuthConfig()).toBeUndefined();
  });

  test("storeOAuthConfig stores client_id in config", () => {
    storeOAuthConfig("test-client-id", "test-secret");
    const oauth = getOAuthConfig();
    expect(oauth).toBeDefined();
    expect(oauth!.client_id).toBe("test-client-id");
    expect(oauth!.redirect_uri).toBe("http://localhost:9876/callback");
  });
});

describe("workspace CRUD", () => {
  test("getWorkspaces returns empty when no workspaces", () => {
    expect(getWorkspaces()).toEqual({});
  });

  test("storeWorkspace adds workspace and auto-sets default", () => {
    storeWorkspace("test-ws", {
      workspace_id: "ws-1",
      workspace_name: "Test Workspace",
      bot_id: "bot-1",
      auth_type: "internal_integration",
      access_token: "ntn_test123",
    });

    const ws = getWorkspace("test-ws");
    expect(ws).toBeDefined();
    expect(ws!.workspace_id).toBe("ws-1");
    expect(ws!.workspace_name).toBe("Test Workspace");
    expect(ws!.auth_type).toBe("internal_integration");

    // First workspace becomes default
    expect(getDefaultWorkspace()).toBe("test-ws");
  });

  test("second workspace does not override default", () => {
    storeWorkspace("ws-1", {
      workspace_id: "id-1",
      workspace_name: "First",
      bot_id: "bot-1",
      auth_type: "internal_integration",
      access_token: "ntn_1",
    });
    storeWorkspace("ws-2", {
      workspace_id: "id-2",
      workspace_name: "Second",
      bot_id: "bot-2",
      auth_type: "internal_integration",
      access_token: "ntn_2",
    });

    expect(getDefaultWorkspace()).toBe("ws-1");
    expect(Object.keys(getWorkspaces())).toHaveLength(2);
  });

  test("removeWorkspace removes and reassigns default", () => {
    storeWorkspace("ws-1", {
      workspace_id: "id-1",
      workspace_name: "First",
      bot_id: "bot-1",
      auth_type: "internal_integration",
      access_token: "ntn_1",
    });
    storeWorkspace("ws-2", {
      workspace_id: "id-2",
      workspace_name: "Second",
      bot_id: "bot-2",
      auth_type: "internal_integration",
      access_token: "ntn_2",
    });

    removeWorkspace("ws-1");

    expect(getWorkspace("ws-1")).toBeUndefined();
    expect(getDefaultWorkspace()).toBe("ws-2");
  });

  test("removeWorkspace throws for unknown alias", () => {
    expect(() => removeWorkspace("nonexistent")).toThrow(
      /Unknown workspace/,
    );
  });

  test("setDefaultWorkspace switches default", () => {
    storeWorkspace("ws-1", {
      workspace_id: "id-1",
      workspace_name: "First",
      bot_id: "bot-1",
      auth_type: "internal_integration",
      access_token: "ntn_1",
    });
    storeWorkspace("ws-2", {
      workspace_id: "id-2",
      workspace_name: "Second",
      bot_id: "bot-2",
      auth_type: "internal_integration",
      access_token: "ntn_2",
    });

    setDefaultWorkspace("ws-2");
    expect(getDefaultWorkspace()).toBe("ws-2");
  });

  test("setDefaultWorkspace throws for unknown alias", () => {
    expect(() => setDefaultWorkspace("nonexistent")).toThrow(
      /Unknown workspace/,
    );
  });

  test("clearAll removes everything", () => {
    storeWorkspace("ws-1", {
      workspace_id: "id-1",
      workspace_name: "First",
      bot_id: "bot-1",
      auth_type: "internal_integration",
      access_token: "ntn_1",
    });

    clearAll();

    expect(readConfig()).toEqual({});
  });
});

describe("deriveAlias", () => {
  test("converts name to lowercase kebab-case", () => {
    expect(deriveAlias("My Workspace", [])).toBe("my-workspace");
  });

  test("removes special characters", () => {
    expect(deriveAlias("Test's (Workspace)!", [])).toBe("test-s-workspace");
  });

  test("truncates to 32 chars", () => {
    const longName = "a".repeat(50);
    expect(deriveAlias(longName, []).length).toBeLessThanOrEqual(32);
  });

  test("appends suffix on collision", () => {
    expect(deriveAlias("test", ["test"])).toBe("test-2");
    expect(deriveAlias("test", ["test", "test-2"])).toBe("test-3");
  });

  test("uses 'default' for empty name", () => {
    expect(deriveAlias("", [])).toBe("default");
  });
});

describe("token updates", () => {
  test("updateWorkspaceTokens updates tokens", () => {
    storeWorkspace("ws-1", {
      workspace_id: "id-1",
      workspace_name: "First",
      bot_id: "bot-1",
      auth_type: "oauth",
      access_token: "old_access",
      refresh_token: "old_refresh",
    });

    updateWorkspaceTokens("ws-1", "new_access", "new_refresh");

    const ws = getWorkspace("ws-1");
    expect(ws).toBeDefined();
    // With mocked keychain, tokens are stored in keychain (placeholder in config)
    expect(ws!.access_token).toBe("__KEYCHAIN__");
    expect(ws!.refresh_token).toBe("__KEYCHAIN__");
    // Verify the actual values are in the mock store
    expect(mockStore.get("access_token:ws-1")).toBe("new_access");
    expect(mockStore.get("refresh_token:ws-1")).toBe("new_refresh");
  });

  test("clearWorkspaceTokens clears tokens", () => {
    storeWorkspace("ws-1", {
      workspace_id: "id-1",
      workspace_name: "First",
      bot_id: "bot-1",
      auth_type: "oauth",
      access_token: "old_access",
      refresh_token: "old_refresh",
    });

    clearWorkspaceTokens("ws-1");

    const ws = getWorkspace("ws-1");
    expect(ws).toBeDefined();
    expect(ws!.access_token).toBe("");
    expect(ws!.refresh_token).toBeUndefined();
  });
});

describe("V3 session (desktop token)", () => {
  test("getV3Session returns undefined when not configured", () => {
    expect(getV3Session()).toBeUndefined();
  });

  test("storeV3Session + getV3Session roundtrip", () => {
    storeV3Session({
      token_v2: "test-token-v2",
      user_id: "user-123",
      user_email: "test@example.com",
      user_name: "Test User",
      space_id: "space-456",
      space_name: "Test Space",
      extracted_at: "2026-01-01T00:00:00.000Z",
    });

    const session = getV3Session();
    expect(session).toBeDefined();
    expect(session!.user_id).toBe("user-123");
    expect(session!.user_email).toBe("test@example.com");
    expect(session!.user_name).toBe("Test User");
    expect(session!.space_id).toBe("space-456");
    expect(session!.space_name).toBe("Test Space");
    expect(session!.extracted_at).toBe("2026-01-01T00:00:00.000Z");
    // Token stored in mock keychain
    expect(session!.token_v2).toBe("__KEYCHAIN__");
  });

  test("resolveV3Token returns token value from mock keychain", () => {
    storeV3Session({
      token_v2: "test-token-resolve",
      user_id: "user-123",
      user_email: "test@example.com",
      user_name: "Test User",
      space_id: "space-456",
      space_name: "Test Space",
      extracted_at: "2026-01-01T00:00:00.000Z",
    });

    const token = resolveV3Token();
    expect(token).toBe("test-token-resolve");
  });

  test("clearV3Session removes session", () => {
    storeV3Session({
      token_v2: "test-token-clear",
      user_id: "user-123",
      user_email: "test@example.com",
      user_name: "Test User",
      space_id: "space-456",
      space_name: "Test Space",
      extracted_at: "2026-01-01T00:00:00.000Z",
    });

    clearV3Session();

    expect(getV3Session()).toBeUndefined();
    expect(resolveV3Token()).toBeUndefined();
  });

  test("V3 session persists alongside workspaces", () => {
    storeWorkspace("ws-1", {
      workspace_id: "id-1",
      workspace_name: "First",
      bot_id: "bot-1",
      auth_type: "internal_integration",
      access_token: "ntn_1",
    });

    storeV3Session({
      token_v2: "test-token-coexist",
      user_id: "user-123",
      user_email: "test@example.com",
      user_name: "Test User",
      space_id: "space-456",
      space_name: "Test Space",
      extracted_at: "2026-01-01T00:00:00.000Z",
    });

    // Both should be present
    expect(getWorkspace("ws-1")).toBeDefined();
    expect(getV3Session()).toBeDefined();
    expect(resolveV3Token()).toBe("test-token-coexist");
  });
});
