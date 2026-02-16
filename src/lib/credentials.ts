import {
  KEYCHAIN_PLACEHOLDER,
  KEYCHAIN_SERVICE,
  keychainGet,
} from "./keychain.ts";
import {
  type AuthType,
  clearWorkspaceTokens,
  getOAuthConfig,
  readConfig,
  resolveOAuthClientSecret,
  updateWorkspaceTokens,
} from "./config.ts";

export type ResolvedCredential = {
  key: string;
  source: "environment" | "keychain" | "config";
  workspace?: string;
  auth_type?: AuthType;
};

export function resolveAccessToken(): ResolvedCredential | undefined {
  // 1. Environment variable (highest priority)
  const envKey = (
    process.env["NOTION_API_KEY"] ?? process.env["NOTION_TOKEN"]
  )?.trim();
  if (envKey) return { key: envKey, source: "environment" };

  // 2. Default workspace
  const config = readConfig();
  const alias = config.default_workspace;
  if (!alias || !config.workspaces?.[alias]) return undefined;

  const ws = config.workspaces[alias];

  // 2a. Keychain
  if (ws.access_token === KEYCHAIN_PLACEHOLDER) {
    const key = keychainGet(`access_token:${alias}`, KEYCHAIN_SERVICE);
    if (key) {
      return {
        key,
        source: "keychain",
        workspace: alias,
        auth_type: ws.auth_type,
      };
    }
    return undefined;
  }

  // 2b. Config plaintext
  if (ws.access_token) {
    return {
      key: ws.access_token,
      source: "config",
      workspace: alias,
      auth_type: ws.auth_type,
    };
  }

  return undefined;
}

export function getAccessToken(): string | undefined {
  return resolveAccessToken()?.key;
}

export function getAccessTokenSource():
  | "environment"
  | "keychain"
  | "config"
  | undefined {
  return resolveAccessToken()?.source;
}

// --- Token refresh ---

export async function refreshAccessToken(
  alias: string,
): Promise<{ access_token: string; refresh_token: string } | undefined> {
  const config = readConfig();
  const ws = config.workspaces?.[alias];
  if (!ws || ws.auth_type !== "oauth") return undefined;

  // Resolve refresh_token
  let refreshToken: string | undefined;
  if (ws.refresh_token === KEYCHAIN_PLACEHOLDER) {
    refreshToken =
      keychainGet(`refresh_token:${alias}`, KEYCHAIN_SERVICE) ?? undefined;
  } else {
    refreshToken = ws.refresh_token;
  }
  if (!refreshToken) return undefined;

  // Resolve OAuth client credentials
  const oauth = getOAuthConfig();
  if (!oauth) return undefined;
  const clientSecret = resolveOAuthClientSecret();
  if (!clientSecret) return undefined;

  const basicAuth = Buffer.from(
    `${oauth.client_id}:${clientSecret}`,
  ).toString("base64");

  try {
    const response = await fetch("https://api.notion.com/v1/oauth/token", {
      method: "POST",
      headers: {
        Authorization: `Basic ${basicAuth}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        grant_type: "refresh_token",
        refresh_token: refreshToken,
      }),
    });

    if (!response.ok) return undefined;

    const data = (await response.json()) as {
      access_token: string;
      refresh_token: string;
    };

    // Atomic swap: update both tokens
    updateWorkspaceTokens(alias, data.access_token, data.refresh_token);

    return data;
  } catch {
    return undefined;
  }
}

/**
 * Attempt to refresh, falling back to re-reading keychain in case
 * another process already refreshed (race condition handling).
 */
export async function refreshOrRecover(
  alias: string,
): Promise<string | undefined> {
  const result = await refreshAccessToken(alias);
  if (result) return result.access_token;

  // Another process may have refreshed — re-read from keychain
  const freshKey = keychainGet(`access_token:${alias}`, KEYCHAIN_SERVICE);
  if (freshKey) return freshKey;

  // Truly failed — clear tokens
  clearWorkspaceTokens(alias);
  return undefined;
}
