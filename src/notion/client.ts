import { Client } from "@notionhq/client";
import { resolveAccessToken, refreshOrRecover } from "../lib/credentials.ts";

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
 * Create a Notion client for the current default workspace.
 * Resolves credentials via env → keychain → config cascade.
 * Returns the client and metadata about the credential source.
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
 * For OAuth workspaces, attempts refresh then retry.
 * For internal integrations, fails immediately on 401.
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

    // Internal integrations can't refresh
    if (cred.auth_type === "internal_integration" || !cred.workspace) {
      throw new NotionClientError(
        cred.auth_type === "internal_integration"
          ? "Token is invalid or revoked. Run 'agent-notion auth login --token <token>' to re-authenticate."
          : "Not authenticated. Run 'agent-notion auth login' to connect.",
        "unauthorized",
      );
    }

    // Attempt refresh for OAuth workspaces
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
  // Notion SDK throws APIResponseError with status
  if ("status" in err && (err as { status: number }).status === 401)
    return true;
  if ("code" in err && (err as { code: string }).code === "unauthorized")
    return true;
  return false;
}
