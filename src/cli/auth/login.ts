import { Command } from "commander";
import { execFileSync } from "node:child_process";
import { platform } from "node:os";
import {
  deriveAlias,
  getOAuthConfig,
  getWorkspaces,
  readConfig,
  resolveOAuthClientSecret,
  storeWorkspace,
} from "../../lib/config.ts";
import { startOAuthServer } from "../../lib/oauth-server.ts";
import { printError, printJson } from "../../lib/output.ts";
import { createNotionClientWithToken } from "../../notion/client.ts";

export function registerLogin(parent: Command): void {
  parent
    .command("login")
    .description("Authenticate with a Notion workspace")
    .option("--alias <name>", "Custom workspace alias")
    .option("--port <port>", "Localhost port for OAuth callback", "9876")
    .option("--token <token>", "Internal integration token (skips OAuth)")
    .action(
      async (opts: { alias?: string; port: string; token?: string }) => {
        try {
          if (opts.token) {
            await loginWithToken(opts.token, opts.alias);
          } else {
            await loginWithOAuth(opts.alias, parseInt(opts.port, 10));
          }
        } catch (err) {
          printError(
            err instanceof Error ? err.message : "Login failed",
          );
        }
      },
    );
}

async function loginWithToken(
  token: string,
  alias?: string,
): Promise<void> {
  const trimmed = token.trim();

  if (
    !trimmed.startsWith("ntn_") &&
    !trimmed.startsWith("secret_")
  ) {
    // Warn but proceed — Notion may change token formats
    console.error(
      JSON.stringify({
        warning:
          "Token does not start with 'ntn_' or 'secret_'. Proceeding anyway.",
      }),
    );
  }

  // Validate token against Notion API
  const client = createNotionClientWithToken(trimmed);
  let me: Awaited<ReturnType<typeof client.users.me>>;
  try {
    me = await client.users.me({});
  } catch {
    throw new Error(
      "Invalid token. Check that the token is correct and the integration has access to a workspace.",
    );
  }

  // For internal integrations, get workspace info by searching for bot info
  // The /users/me endpoint returns the bot user for integrations
  const botId = me.id;
  const workspaceName = alias ?? "default";

  const resolvedAlias =
    alias ?? deriveAlias(workspaceName, Object.keys(getWorkspaces()));

  const config = readConfig();
  const isDefault = !config.default_workspace;

  storeWorkspace(resolvedAlias, {
    workspace_id: botId,
    workspace_name: workspaceName,
    bot_id: botId,
    auth_type: "internal_integration",
  } as Parameters<typeof storeWorkspace>[1] & { access_token: string });

  // Re-store with actual token since storeWorkspace needs it
  // Actually, let's pass the token properly
  storeWorkspaceWithToken(resolvedAlias, {
    workspace_id: botId,
    workspace_name: resolvedAlias,
    bot_id: botId,
    auth_type: "internal_integration" as const,
    access_token: trimmed,
  });

  printJson({
    ok: true,
    workspace: {
      alias: resolvedAlias,
      name: resolvedAlias,
      id: botId,
      auth_type: "internal_integration",
      default: isDefault || config.default_workspace === resolvedAlias,
    },
  });
}

function storeWorkspaceWithToken(
  alias: string,
  ws: {
    workspace_id: string;
    workspace_name: string;
    workspace_icon?: string;
    bot_id: string;
    auth_type: "oauth" | "internal_integration";
    access_token: string;
    refresh_token?: string;
    owner?: {
      type: "user";
      user: { id: string; name?: string; email?: string };
    };
  },
): { storage: "keychain" | "config" } {
  return storeWorkspace(alias, ws);
}

async function loginWithOAuth(
  alias?: string,
  port: number = 9876,
): Promise<void> {
  // 1. Check OAuth is configured
  const oauth = getOAuthConfig();
  if (!oauth) {
    throw new Error(
      "OAuth not configured. Run 'agent-notion auth setup-oauth' first, or use 'agent-notion auth login --token <token>' for internal integrations.",
    );
  }

  // 2. Resolve client_secret
  const clientSecret = resolveOAuthClientSecret();
  if (!clientSecret) {
    throw new Error(
      "OAuth client secret not found. Run 'agent-notion auth setup-oauth' to reconfigure.",
    );
  }

  // 3. Generate state parameter
  const state = crypto.randomUUID();

  // 4. Start localhost server and wait for callback
  const serverPromise = startOAuthServer(state, port);

  // 5. Open browser
  const redirectUri = `http://localhost:${port}/callback`;
  const authUrl = new URL("https://api.notion.com/v1/oauth/authorize");
  authUrl.searchParams.set("client_id", oauth.client_id);
  authUrl.searchParams.set("redirect_uri", redirectUri);
  authUrl.searchParams.set("response_type", "code");
  authUrl.searchParams.set("owner", "user");
  authUrl.searchParams.set("state", state);

  openBrowser(authUrl.toString());

  // 6. Wait for callback
  const { code, port: actualPort } = await serverPromise;
  const actualRedirectUri = `http://localhost:${actualPort}/callback`;

  // 7. Exchange code for tokens
  const basicAuth = Buffer.from(
    `${oauth.client_id}:${clientSecret}`,
  ).toString("base64");

  const tokenResponse = await fetch(
    "https://api.notion.com/v1/oauth/token",
    {
      method: "POST",
      headers: {
        Authorization: `Basic ${basicAuth}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        grant_type: "authorization_code",
        code,
        redirect_uri: actualRedirectUri,
      }),
    },
  );

  if (!tokenResponse.ok) {
    const errorBody = (await tokenResponse.json().catch(() => ({}))) as {
      error?: string;
      message?: string;
    };
    throw new Error(
      `Failed to exchange authorization code for token: ${errorBody.message ?? errorBody.error ?? tokenResponse.statusText}`,
    );
  }

  const tokenData = (await tokenResponse.json()) as {
    access_token: string;
    token_type: string;
    bot_id: string;
    workspace_id: string;
    workspace_name: string;
    workspace_icon?: string;
    owner?: {
      type: "user";
      user: { id: string; name?: string; person?: { email?: string } };
    };
    refresh_token?: string;
  };

  // 8. Determine alias
  const existingAliases = Object.keys(getWorkspaces());
  const resolvedAlias =
    alias ?? deriveAlias(tokenData.workspace_name, existingAliases);

  // 9. Store credentials
  const config = readConfig();
  const isDefault = !config.default_workspace;

  storeWorkspace(resolvedAlias, {
    workspace_id: tokenData.workspace_id,
    workspace_name: tokenData.workspace_name,
    workspace_icon: tokenData.workspace_icon,
    bot_id: tokenData.bot_id,
    auth_type: "oauth",
    access_token: tokenData.access_token,
    refresh_token: tokenData.refresh_token,
    owner: tokenData.owner
      ? {
          type: "user",
          user: {
            id: tokenData.owner.user.id,
            name: tokenData.owner.user.name,
            email: tokenData.owner.user.person?.email,
          },
        }
      : undefined,
  });

  printJson({
    ok: true,
    workspace: {
      alias: resolvedAlias,
      name: tokenData.workspace_name,
      id: tokenData.workspace_id,
      bot_id: tokenData.bot_id,
      default:
        isDefault || config.default_workspace === resolvedAlias,
    },
    hint: "Add more workspaces with 'agent-notion auth login --alias <name>'",
  });
}

function openBrowser(url: string): void {
  const plat = platform();
  try {
    if (plat === "darwin") {
      execFileSync("open", [url], { stdio: "ignore" });
    } else if (plat === "linux") {
      execFileSync("xdg-open", [url], { stdio: "ignore" });
    } else if (plat === "win32") {
      execFileSync("cmd", ["/c", "start", url], { stdio: "ignore" });
    }
  } catch {
    // If we can't open the browser, print the URL for manual opening
    console.error(
      JSON.stringify({
        warning: `Could not open browser. Please visit: ${url}`,
      }),
    );
  }
}
