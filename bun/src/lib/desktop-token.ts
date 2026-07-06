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
import { unwrapRecordValue } from "../notion/v3/record-map.ts";

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

  const data = (await response.json()) as GetSpacesResponse;
  return parseGetSpacesSession(data);
}

/** getSpaces responds with userId → table name → record ID → record. */
type GetSpacesTables = Record<string, Record<string, { value?: unknown }>>;
export type GetSpacesResponse = Record<string, GetSpacesTables>;

/**
 * Extract session info from a getSpaces response.
 * Only the first (and usually only) user entry is considered.
 * Note: getSpaces bypasses the v3 client, so records may still be
 * role-wrapped — every access goes through unwrapRecordValue.
 */
export function parseGetSpacesSession(data: GetSpacesResponse): DesktopSessionInfo {
  const [firstEntry] = Object.entries(data);
  if (!firstEntry) {
    throw new DesktopTokenError(
      "Could not extract user info from token. The token may be expired.",
      "validation_failed",
    );
  }

  const [userId, tables] = firstEntry;
  const user = findUserInfo(tables);
  const space = pickPreferredSpace(tables);

  return {
    user_id: userId,
    user_email: user.email,
    user_name: user.name,
    space_id: space.id,
    space_name: space.name,
    space_view_id: findSpaceViewId(tables, space.id),
  };
}

function tableEntities(
  tables: GetSpacesTables,
  table: string,
): Record<string, unknown>[] {
  return Object.values(tables[table] ?? {})
    .map((record) => unwrapRecordValue(record.value))
    .filter((v): v is Record<string, unknown> => v !== undefined);
}

function findUserInfo(tables: GetSpacesTables): { email: string; name: string } {
  const user = tableEntities(tables, "notion_user").find((v) => "email" in v);
  if (!user) return { email: "", name: "" };
  // The notion_user record uses "name" (not given_name/family_name)
  return {
    email: (user.email as string) ?? "",
    name: (user.name as string) ?? "",
  };
}

function pickPreferredSpace(tables: GetSpacesTables): { id: string; name: string } {
  const spaces = tableEntities(tables, "space").filter((v) => "name" in v);
  // Take the first space, but prefer team/enterprise plans
  const preferred =
    spaces.filter((v) => v.plan_type === "team" || v.plan_type === "enterprise").at(-1) ??
    spaces[0];
  if (!preferred) return { id: "", name: "" };
  return {
    id: (preferred.id as string) ?? "",
    name: preferred.name as string,
  };
}

function findSpaceViewId(tables: GetSpacesTables, spaceId: string): string | undefined {
  const view = tableEntities(tables, "space_view").find((v) => v.space_id === spaceId);
  return view ? (view.id as string) : undefined;
}
