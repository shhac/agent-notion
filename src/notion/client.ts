/**
 * Backend factory — resolves auth credentials and returns the appropriate NotionBackend.
 *
 * Auth resolution order:
 *   1. v3 session (desktop token) → V3Backend
 *   2. Environment variable → OfficialBackend
 *   3. Default workspace → OfficialBackend (with auto-refresh for OAuth)
 *
 * The `withBackend` helper wraps operations with 401 retry for OAuth workspaces.
 */
import { Client } from "@notionhq/client";
import { resolveAccessToken, refreshOrRecover } from "../lib/credentials.ts";
import { getV3Session, resolveV3Token } from "../lib/config.ts";
import type { NotionBackend } from "./interface.ts";
import { OfficialBackend } from "./official/client.ts";
import { V3Backend } from "./v3/backend.ts";
import { V3HttpClient, V3HttpError } from "./v3/client.ts";

export class NotionClientError extends Error {
  constructor(
    message: string,
    public readonly code?: string,
  ) {
    super(message);
    this.name = "NotionClientError";
  }
}

/**
 * Create a NotionBackend for the current default workspace.
 * Prefers v3 session if available, falls back to official SDK.
 */
export function createBackend(): {
  backend: NotionBackend;
  workspace?: string;
  auth_type?: string;
} {
  // 1. Try v3 session (desktop token)
  const v3Session = getV3Session();
  if (v3Session) {
    const tokenV2 = resolveV3Token();
    if (tokenV2) {
      const http = new V3HttpClient({
        tokenV2,
        userId: v3Session.user_id,
        spaceId: v3Session.space_id,
      });
      return {
        backend: new V3Backend(http),
        workspace: v3Session.space_name,
        auth_type: "desktop",
      };
    }
  }

  // 2. Try official API credentials
  const cred = resolveAccessToken();
  if (!cred) {
    throw new NotionClientError(
      "Not authenticated. Run 'agent-notion auth login' to connect.",
      "not_authenticated",
    );
  }

  const client = new Client({ auth: cred.key });
  return {
    backend: new OfficialBackend(client),
    workspace: cred.workspace,
    auth_type: cred.auth_type,
  };
}

/**
 * Execute a backend operation with automatic token refresh on 401.
 * For OAuth workspaces with official backend, attempts refresh then retry.
 * For v3 backend, 401 means the desktop token expired (no auto-refresh possible).
 */
export async function withBackend<T>(
  fn: (backend: NotionBackend) => Promise<T>,
): Promise<T> {
  const { backend, auth_type, workspace } = createBackend();

  try {
    return await fn(backend);
  } catch (err: unknown) {
    if (!isUnauthorizedError(err)) throw err;

    // v3 backend — desktop tokens can't be refreshed programmatically
    if (backend.kind === "v3") {
      throw new NotionClientError(
        "Desktop token expired. Run 'agent-notion auth import-desktop' to re-import.",
        "unauthorized",
      );
    }

    // Internal integrations can't refresh
    if (auth_type === "internal_integration" || !workspace) {
      throw new NotionClientError(
        auth_type === "internal_integration"
          ? "Token is invalid or revoked. Run 'agent-notion auth login --token <token>' to re-authenticate."
          : "Not authenticated. Run 'agent-notion auth login' to connect.",
        "unauthorized",
      );
    }

    // Attempt refresh for OAuth workspaces
    const newToken = await refreshOrRecover(workspace);
    if (!newToken) {
      throw new NotionClientError(
        "Token expired and refresh failed. Run 'agent-notion auth login' to re-authenticate.",
        "refresh_failed",
      );
    }

    const refreshedClient = new Client({ auth: newToken });
    const refreshedBackend = new OfficialBackend(refreshedClient);
    return await fn(refreshedBackend);
  }
}

/**
 * Get a V3HttpClient directly for v3-only features (export, etc.).
 * Throws if no v3 session is configured.
 */
export function createV3Client(): V3HttpClient {
  const v3Session = getV3Session();
  if (!v3Session) {
    throw new NotionClientError(
      "Export requires a v3 desktop session. Run 'agent-notion auth import-desktop' first.",
      "v3_required",
    );
  }
  const tokenV2 = resolveV3Token();
  if (!tokenV2) {
    throw new NotionClientError(
      "Desktop token not found. Run 'agent-notion auth import-desktop' to set up.",
      "v3_required",
    );
  }
  return new V3HttpClient({
    tokenV2,
    userId: v3Session.user_id,
    spaceId: v3Session.space_id,
  });
}

// --- Legacy compatibility ---

/**
 * Create a Notion client for the current default workspace.
 * @deprecated Use createBackend() instead for dual-backend support.
 */
export function createNotionClient(): {
  client: Client;
  workspace?: string;
  auth_type?: string;
} {
  const cred = resolveAccessToken();
  if (!cred) {
    throw new NotionClientError(
      "Not authenticated. Run 'agent-notion auth login' to connect.",
      "not_authenticated",
    );
  }

  const client = new Client({ auth: cred.key });
  return {
    client,
    workspace: cred.workspace,
    auth_type: cred.auth_type,
  };
}

/**
 * Create a Notion client with a specific token (e.g. for validation during login).
 */
export function createNotionClientWithToken(token: string): Client {
  return new Client({ auth: token });
}

/**
 * Execute a Notion API call with automatic token refresh on 401.
 * @deprecated Use withBackend() instead for dual-backend support.
 */
export async function withAutoRefresh<T>(
  fn: (client: Client) => Promise<T>,
): Promise<T> {
  const cred = resolveAccessToken();
  if (!cred) {
    throw new NotionClientError(
      "Not authenticated. Run 'agent-notion auth login' to connect.",
      "not_authenticated",
    );
  }

  const client = new Client({ auth: cred.key });

  try {
    return await fn(client);
  } catch (err: unknown) {
    if (!isUnauthorizedError(err)) throw err;

    if (cred.auth_type === "internal_integration" || !cred.workspace) {
      throw new NotionClientError(
        cred.auth_type === "internal_integration"
          ? "Token is invalid or revoked. Run 'agent-notion auth login --token <token>' to re-authenticate."
          : "Not authenticated. Run 'agent-notion auth login' to connect.",
        "unauthorized",
      );
    }

    const newToken = await refreshOrRecover(cred.workspace);
    if (!newToken) {
      throw new NotionClientError(
        "Token expired and refresh failed. Run 'agent-notion auth login' to re-authenticate.",
        "refresh_failed",
      );
    }

    const refreshedClient = new Client({ auth: newToken });
    return await fn(refreshedClient);
  }
}

function isUnauthorizedError(err: unknown): boolean {
  if (typeof err !== "object" || err === null) return false;
  if ("status" in err && (err as { status: number }).status === 401)
    return true;
  if ("code" in err && (err as { code: string }).code === "unauthorized")
    return true;
  // v3 API may return 403 for expired desktop tokens
  if (err instanceof V3HttpError && err.status === 403) return true;
  return false;
}
