import { describe, test, expect } from "bun:test";
import { platform } from "node:os";
import {
  KEYCHAIN_SERVICE,
  KEYCHAIN_PLACEHOLDER,
  keychainGet,
  keychainSet,
  keychainDelete,
} from "../src/lib/keychain.ts";

const IS_MACOS = platform() === "darwin";
const TEST_ACCOUNT = `test-agent-notion-${Date.now()}`;

describe("keychain constants", () => {
  test("KEYCHAIN_SERVICE is correct", () => {
    expect(KEYCHAIN_SERVICE).toBe("app.paulie.agent-notion");
  });

  test("KEYCHAIN_PLACEHOLDER is correct", () => {
    expect(KEYCHAIN_PLACEHOLDER).toBe("__KEYCHAIN__");
  });
});

describe("keychain operations", () => {
  if (!IS_MACOS) {
    test("keychainGet returns null on non-macOS", () => {
      expect(keychainGet("any", "any")).toBeNull();
    });

    test("keychainSet returns false on non-macOS", () => {
      expect(
        keychainSet({ account: "any", value: "any", service: "any" }),
      ).toBe(false);
    });

    test("keychainDelete returns false on non-macOS", () => {
      expect(keychainDelete("any", "any")).toBe(false);
    });
  } else {
    test("keychainSet + keychainGet roundtrip", () => {
      const testValue = `test-value-${Date.now()}`;
      const success = keychainSet({
        account: TEST_ACCOUNT,
        value: testValue,
        service: KEYCHAIN_SERVICE,
      });
      expect(success).toBe(true);

      const retrieved = keychainGet(TEST_ACCOUNT, KEYCHAIN_SERVICE);
      expect(retrieved).toBe(testValue);

      // Cleanup
      keychainDelete(TEST_ACCOUNT, KEYCHAIN_SERVICE);
    });

    test("keychainGet returns null for nonexistent entry", () => {
      const result = keychainGet(
        "nonexistent-account-12345",
        KEYCHAIN_SERVICE,
      );
      expect(result).toBeNull();
    });

    test("keychainDelete removes entry", () => {
      keychainSet({
        account: TEST_ACCOUNT,
        value: "to-delete",
        service: KEYCHAIN_SERVICE,
      });

      const deleted = keychainDelete(TEST_ACCOUNT, KEYCHAIN_SERVICE);
      expect(deleted).toBe(true);

      const result = keychainGet(TEST_ACCOUNT, KEYCHAIN_SERVICE);
      expect(result).toBeNull();
    });

    test("keychainDelete returns false for nonexistent entry", () => {
      const result = keychainDelete(
        "nonexistent-account-12345",
        KEYCHAIN_SERVICE,
      );
      expect(result).toBe(false);
    });

    test("keychainSet overwrites existing entry", () => {
      keychainSet({
        account: TEST_ACCOUNT,
        value: "first",
        service: KEYCHAIN_SERVICE,
      });
      keychainSet({
        account: TEST_ACCOUNT,
        value: "second",
        service: KEYCHAIN_SERVICE,
      });

      const result = keychainGet(TEST_ACCOUNT, KEYCHAIN_SERVICE);
      expect(result).toBe("second");

      // Cleanup
      keychainDelete(TEST_ACCOUNT, KEYCHAIN_SERVICE);
    });
  }
});
