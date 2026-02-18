/**
 * Extract token_v2 from the Notion Desktop app's Chromium cookie store (macOS only).
 *
 * Notion Desktop is Electron-based and uses Chromium's Safe Storage encryption:
 *   1. AES-128-CBC key derived from a passphrase stored in macOS Keychain
 *   2. Cookies stored in an SQLite database
 *   3. Encrypted values prefixed with "v10" (3 bytes)
 *   4. Meta version >= 24 adds a 32-byte SHA256 domain hash before the plaintext
 */

import { execFileSync } from "node:child_process";
import { createDecipheriv, pbkdf2Sync } from "node:crypto";
import { existsSync } from "node:fs";
import { homedir, platform } from "node:os";
import { join } from "node:path";
import { Database } from "bun:sqlite";

const COOKIES_DB_PATH = join(
  homedir(),
  "Library",
  "Application Support",
  "Notion",
  "Partitions",
  "notion",
  "Cookies",
);

const SAFE_STORAGE_SERVICE = "Notion Safe Storage";
const PBKDF2_SALT = "saltysalt";
const PBKDF2_ITERATIONS = 1003;
const PBKDF2_KEY_LEN = 16;
const AES_IV = Buffer.alloc(16, 0x20); // 16 space characters

export type DesktopToken = {
  token_v2: string;
  extracted_at: string;
};

export type DesktopSessionInfo = {
  user_id: string;
  user_email: string;
  user_name: string;
  space_id: string;
  space_name: string;
  space_view_id?: string;
};

export class DesktopTokenError extends Error {
  constructor(
    message: string,
    public readonly code:
      | "not_macos"
      | "no_notion_app"
      | "no_keychain_entry"
      | "no_cookie"
      | "decryption_failed"
      | "validation_failed",
  ) {
    super(message);
    this.name = "DesktopTokenError";
  }
}

/**
 * Extract and decrypt the Notion Safe Storage passphrase from macOS Keychain.
 */
function getSafeStoragePassphrase(): string {
  try {
    const result = execFileSync(
      "security",
      ["find-generic-password", "-s", SAFE_STORAGE_SERVICE, "-w"],
      { encoding: "utf8", stdio: ["pipe", "pipe", "ignore"] },
    );
    return result.trim();
  } catch {
    throw new DesktopTokenError(
      "Notion Safe Storage key not found in Keychain. Is Notion Desktop installed?",
      "no_keychain_entry",
    );
  }
}

/**
 * Derive the AES-128 encryption key from the Safe Storage passphrase.
 */
function deriveEncryptionKey(passphrase: string): Buffer {
  return pbkdf2Sync(
    passphrase,
    PBKDF2_SALT,
    PBKDF2_ITERATIONS,
    PBKDF2_KEY_LEN,
    "sha1",
  );
}

/**
 * Read the encrypted token_v2 cookie from the Notion Cookies SQLite database.
 */
function readEncryptedCookie(): { encrypted: Buffer; metaVersion: number } {
  if (!existsSync(COOKIES_DB_PATH)) {
    throw new DesktopTokenError(
      `Notion Cookies database not found at ${COOKIES_DB_PATH}`,
      "no_notion_app",
    );
  }

  const db = new Database(COOKIES_DB_PATH, { readonly: true });
  try {
    // Check meta version (affects decryption: version >= 24 has 32-byte domain hash prefix)
    const meta = db
      .query<{ value: string }, []>(
        "SELECT value FROM meta WHERE key = 'version'",
      )
      .get();
    const metaVersion = meta ? parseInt(meta.value, 10) : 0;

    const row = db
      .query<{ encrypted_value: Buffer }, []>(
        "SELECT encrypted_value FROM cookies WHERE name = 'token_v2'",
      )
      .get();

    if (!row?.encrypted_value) {
      throw new DesktopTokenError(
        "token_v2 cookie not found. Is Notion Desktop logged in?",
        "no_cookie",
      );
    }

    return { encrypted: Buffer.from(row.encrypted_value), metaVersion };
  } finally {
    db.close();
  }
}

/**
 * Decrypt a Chromium encrypted cookie value.
 *
 * Format: "v10" prefix (3 bytes) + AES-128-CBC encrypted data with PKCS7 padding.
 * For meta version >= 24, the decrypted plaintext starts with a 32-byte SHA256 domain hash.
 */
function decryptCookieValue(
  encrypted: Buffer,
  key: Buffer,
  metaVersion: number,
): string {
  // Verify v10 prefix
  const prefix = encrypted.subarray(0, 3).toString("ascii");
  if (prefix !== "v10") {
    throw new DesktopTokenError(
      `Unexpected cookie encryption prefix: "${prefix}" (expected "v10")`,
      "decryption_failed",
    );
  }

  const ciphertext = encrypted.subarray(3);

  const decipher = createDecipheriv("aes-128-cbc", key, AES_IV);
  const decrypted = Buffer.concat([
    decipher.update(ciphertext),
    decipher.final(),
  ]);

  // For meta version >= 24, skip 32-byte SHA256 domain hash
  const offset = metaVersion >= 24 ? 32 : 0;
  const raw = decrypted.subarray(offset).toString("utf-8");

  // URL-decode (Chromium may URL-encode cookie values)
  return decodeURIComponent(raw);
}

/**
 * Extract token_v2 from the Notion Desktop app. macOS only.
 */
export function extractDesktopToken(): DesktopToken {
  if (platform() !== "darwin") {
    throw new DesktopTokenError(
      "Desktop token extraction is only supported on macOS.",
      "not_macos",
    );
  }

  const passphrase = getSafeStoragePassphrase();
  const key = deriveEncryptionKey(passphrase);
  const { encrypted, metaVersion } = readEncryptedCookie();
  const token = decryptCookieValue(encrypted, key, metaVersion);

  if (!token || token.length < 50) {
    throw new DesktopTokenError(
      "Decrypted token_v2 appears invalid (too short).",
      "decryption_failed",
    );
  }

  return {
    token_v2: token,
    extracted_at: new Date().toISOString(),
  };
}

/**
 * Validate a token_v2 against Notion's unofficial API and return session info.
 */
export async function validateDesktopToken(
  token: string,
): Promise<DesktopSessionInfo> {
  // Call getSpaces to validate the token and get workspace info
  const response = await fetch("https://www.notion.so/api/v3/getSpaces", {
    method: "POST",
    headers: {
      Cookie: `token_v2=${token}`,
      "Content-Type": "application/json",
    },
    body: "{}",
  });

  if (!response.ok) {
    throw new DesktopTokenError(
      `Token validation failed: HTTP ${response.status}`,
      "validation_failed",
    );
  }

  const data = (await response.json()) as Record<string, Record<string, Record<string, { value?: Record<string, unknown> }>>>;

  // Extract user and space info from the getSpaces response
  let userId = "";
  let userEmail = "";
  let userName = "";
  let spaceId = "";
  let spaceName = "";

  for (const [uid, tables] of Object.entries(data)) {
    userId = uid;

    // Find user info from notion_user table
    const users = tables["notion_user"];
    if (users) {
      for (const record of Object.values(users)) {
        const v = record.value;
        if (v && typeof v === "object" && "email" in v) {
          userEmail = (v.email as string) ?? "";
          // The notion_user record uses "name" (not given_name/family_name)
          userName = (v.name as string) ?? "";
          break;
        }
      }
    }

    // Find primary space (prefer team plan, fall back to first)
    const spaces = tables["space"];
    if (spaces) {
      for (const record of Object.values(spaces)) {
        const v = record.value;
        if (v && typeof v === "object" && "name" in v) {
          const name = v.name as string;
          const plan = v.plan_type as string | undefined;
          // Take the first space, but prefer team/enterprise plans
          if (!spaceId || plan === "team" || plan === "enterprise") {
            spaceId = (v.id as string) ?? "";
            spaceName = name;
          }
        }
      }
    }

    break; // Only process the first (and usually only) user
  }

  if (!userId) {
    throw new DesktopTokenError(
      "Could not extract user info from token. The token may be expired.",
      "validation_failed",
    );
  }

  // Find the space_view for the selected space
  let spaceViewId: string | undefined;
  for (const tables of Object.values(data)) {
    const spaceViews = tables["space_view"] as Record<string, { value?: Record<string, unknown> }> | undefined;
    if (spaceViews) {
      for (const record of Object.values(spaceViews)) {
        if (record.value?.space_id === spaceId) {
          spaceViewId = record.value.id as string;
          break;
        }
      }
    }
    break;
  }

  return { user_id: userId, user_email: userEmail, user_name: userName, space_id: spaceId, space_name: spaceName, space_view_id: spaceViewId };
}
