import { describe, test, expect } from "bun:test";
import { platform } from "node:os";
import {
  DesktopTokenError,
  extractDesktopToken,
  validateDesktopToken,
} from "../src/lib/desktop-token.ts";

const IS_MACOS = platform() === "darwin";
const LIVE_TESTS = process.env["LIVE_TESTS"] === "1";

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
