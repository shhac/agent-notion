import { afterEach, describe, test, expect } from "bun:test";
import { platform } from "node:os";
import {
  DesktopTokenError,
  extractDesktopToken,
  validateDesktopToken,
  parseGetSpacesSession,
} from "../src/lib/desktop-token.ts";
import { mockFetch, restoreFetch } from "./helpers/fetch-mock.ts";

const IS_MACOS = platform() === "darwin";
const LIVE_TESTS = process.env["LIVE_TESTS"] === "1";

function mockGetSpaces(responseBody: unknown, status = 200) {
  mockFetch(responseBody, status);
}

afterEach(restoreFetch);

describe("DesktopTokenError", () => {
  test("has correct name and code", () => {
    const err = new DesktopTokenError("test message", "not_macos");
    expect(err.name).toBe("DesktopTokenError");
    expect(err.code).toBe("not_macos");
    expect(err.message).toBe("test message");
    expect(err).toBeInstanceOf(Error);
  });

  test("supports all error codes", () => {
    const codes = [
      "not_macos",
      "no_notion_app",
      "no_keychain_entry",
      "no_cookie",
      "decryption_failed",
      "validation_failed",
    ] as const;

    for (const code of codes) {
      const err = new DesktopTokenError(`error: ${code}`, code);
      expect(err.code).toBe(code);
    }
  });
});

describe("extractDesktopToken", () => {
  if (!IS_MACOS) {
    test("throws not_macos on non-macOS platforms", () => {
      try {
        extractDesktopToken();
        expect(true).toBe(false); // Should not reach here
      } catch (err) {
        expect(err).toBeInstanceOf(DesktopTokenError);
        expect((err as DesktopTokenError).code).toBe("not_macos");
      }
    });
  } else if (!LIVE_TESTS) {
    test("skipped: live desktop token extraction requires LIVE_TESTS=1", () => {
      expect(true).toBe(true);
    });
  } else {
    test("extracts a valid token_v2 from Notion Desktop", () => {
      const result = extractDesktopToken();

      expect(result.token_v2).toBeDefined();
      expect(typeof result.token_v2).toBe("string");
      expect(result.token_v2.length).toBeGreaterThan(50);

      expect(result.extracted_at).toBeDefined();
      // Verify ISO date format
      expect(new Date(result.extracted_at).toISOString()).toBe(
        result.extracted_at,
      );
    });

    test("returns a token starting with v03 JWE prefix", () => {
      const { token_v2 } = extractDesktopToken();
      expect(token_v2.startsWith("v03:")).toBe(true);
    });
  }
});

describe("validateDesktopToken", () => {
  test("throws validation_failed on a non-OK response", async () => {
    mockGetSpaces({}, 401);

    const err = await validateDesktopToken("expired-token").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(DesktopTokenError);
    expect((err as DesktopTokenError).code).toBe("validation_failed");
    expect((err as DesktopTokenError).message).toContain("HTTP 401");
  });

  test("throws validation_failed when the response has no user entry", async () => {
    mockGetSpaces({});

    const err = await validateDesktopToken("expired-token").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(DesktopTokenError);
    expect((err as DesktopTokenError).code).toBe("validation_failed");
    expect((err as DesktopTokenError).message).toContain("Could not extract user info");
  });

  test("extracts session info from role-wrapped getSpaces records", async () => {
    mockGetSpaces({
      "user-map-id": {
        notion_user: {
          "user-1": {
            value: {
              value: {
                id: "user-1",
                email: "alice@example.com",
                name: "Alice Example",
              },
              role: "reader",
            },
          },
        },
        space: {
          "space-1": {
            value: {
              value: {
                id: "space-1",
                name: "Example Workspace",
                plan_type: "team",
              },
              role: "editor",
            },
          },
        },
        space_view: {
          "view-1": {
            value: {
              value: {
                id: "view-1",
                space_id: "space-1",
              },
              role: "reader",
            },
          },
        },
      },
    });

    const session = await validateDesktopToken("fake-token");

    expect(session).toEqual({
      user_id: "user-map-id",
      user_email: "alice@example.com",
      user_name: "Alice Example",
      space_id: "space-1",
      space_name: "Example Workspace",
      space_view_id: "view-1",
    });
  });

  test("still extracts session info from shallow getSpaces records", async () => {
    mockGetSpaces({
      "user-map-id": {
        notion_user: {
          "user-1": {
            value: {
              id: "user-1",
              email: "alice@example.com",
              name: "Alice Example",
            },
          },
        },
        space: {
          "space-1": {
            value: {
              id: "space-1",
              name: "Example Workspace",
            },
          },
        },
      },
    });

    const session = await validateDesktopToken("fake-token");

    expect(session.user_id).toBe("user-map-id");
    expect(session.user_email).toBe("alice@example.com");
    expect(session.user_name).toBe("Alice Example");
    expect(session.space_id).toBe("space-1");
    expect(session.space_name).toBe("Example Workspace");
  });

  if (!IS_MACOS || !LIVE_TESTS) {
    test("skipped: live token validation requires macOS + LIVE_TESTS=1", () => {
      expect(true).toBe(true);
    });
  } else {
    test("validates extracted token and returns session info", async () => {
      const { token_v2 } = extractDesktopToken();
      const session = await validateDesktopToken(token_v2);

      expect(session.user_id).toBeDefined();
      expect(typeof session.user_id).toBe("string");
      expect(session.user_id.length).toBeGreaterThan(0);

      expect(session.user_email).toBeDefined();
      expect(session.user_email).toContain("@");

      expect(session.user_name).toBeDefined();
      expect(session.user_name.length).toBeGreaterThan(0);

      expect(session.space_id).toBeDefined();
      expect(session.space_id.length).toBeGreaterThan(0);

      expect(session.space_name).toBeDefined();
      expect(session.space_name.length).toBeGreaterThan(0);
    });

    test("throws validation_failed for invalid token", async () => {
      try {
        await validateDesktopToken("invalid-token-value");
        expect(true).toBe(false); // Should not reach here
      } catch (err) {
        expect(err).toBeInstanceOf(DesktopTokenError);
        expect((err as DesktopTokenError).code).toBe("validation_failed");
      }
    });
  }
});

describe("parseGetSpacesSession", () => {
  const shallowRecord = (entity: Record<string, unknown>) => ({ value: entity });

  test("prefers a team space over an earlier free space", () => {
    const session = parseGetSpacesSession({
      "user-map-id": {
        notion_user: {
          "user-1": shallowRecord({ id: "user-1", email: "alice@example.com", name: "Alice Example" }),
        },
        space: {
          "space-free": shallowRecord({ id: "space-free", name: "Personal", plan_type: "personal" }),
          "space-team": shallowRecord({ id: "space-team", name: "Team Workspace", plan_type: "team" }),
        },
      },
    });

    expect(session.space_id).toBe("space-team");
    expect(session.space_name).toBe("Team Workspace");
  });

  test("falls back to the first space when no team/enterprise plan exists", () => {
    const session = parseGetSpacesSession({
      "user-map-id": {
        space: {
          "space-a": shallowRecord({ id: "space-a", name: "First" }),
          "space-b": shallowRecord({ id: "space-b", name: "Second" }),
        },
      },
    });

    expect(session.space_id).toBe("space-a");
    expect(session.user_email).toBe("");
  });

  test("leaves space_view_id undefined when no space_view matches", () => {
    const session = parseGetSpacesSession({
      "user-map-id": {
        space: {
          "space-1": shallowRecord({ id: "space-1", name: "Example Workspace" }),
        },
        space_view: {
          "view-other": shallowRecord({ id: "view-other", space_id: "some-other-space" }),
        },
      },
    });

    expect(session.space_id).toBe("space-1");
    expect(session.space_view_id).toBeUndefined();
  });

  test("throws when the response is empty", () => {
    expect(() => parseGetSpacesSession({})).toThrow(DesktopTokenError);
  });
});
