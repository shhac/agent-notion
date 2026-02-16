import { Command } from "commander";
import {
  getDefaultWorkspace,
  getOAuthConfig,
  getWorkspaces,
} from "../../lib/config.ts";
import { resolveAccessToken } from "../../lib/credentials.ts";
import { printError, printJson } from "../../lib/output.ts";
import { createNotionClientWithToken } from "../../notion/client.ts";

export function registerStatus(parent: Command): void {
  parent
    .command("status")
    .description("Show authentication state")
    .action(async () => {
      try {
        const cred = resolveAccessToken();
        const oauthConfigured = !!getOAuthConfig();

        if (!cred) {
          printJson({
            authenticated: false,
            oauth_configured: oauthConfigured,
            hint: "Run 'agent-notion auth setup-oauth' to configure OAuth, or 'agent-notion auth login --token <token>' for internal integrations.",
          });
          return;
        }

        // Validate token with API
        const client = createNotionClientWithToken(cred.key);
        let me: Awaited<ReturnType<typeof client.users.me>>;
        try {
          me = await client.users.me({});
        } catch {
          printJson({
            authenticated: false,
            source: cred.source,
            workspace: cred.workspace,
            oauth_configured: oauthConfigured,
            error: "Token is present but invalid or expired.",
            hint: "Run 'agent-notion auth login' to re-authenticate.",
          });
          return;
        }

        const defaultAlias = getDefaultWorkspace();
        const allWorkspaces = getWorkspaces();
        const defaultWs = defaultAlias
          ? allWorkspaces[defaultAlias]
          : undefined;

        const otherWorkspaces = Object.entries(allWorkspaces)
          .filter(([alias]) => alias !== defaultAlias)
          .map(([alias, ws]) => ({
            alias,
            name: ws.workspace_name,
            auth_type: ws.auth_type,
          }));

        printJson({
          authenticated: true,
          source: cred.source,
          user: {
            id: me.id,
            name: me.name,
            type: me.type,
          },
          workspace: defaultAlias
            ? {
                alias: defaultAlias,
                name: defaultWs?.workspace_name,
                id: defaultWs?.workspace_id,
                auth_type: defaultWs?.auth_type,
              }
            : undefined,
          other_workspaces:
            otherWorkspaces.length > 0 ? otherWorkspaces : undefined,
          oauth_configured: oauthConfigured,
        });
      } catch (err) {
        printError(
          err instanceof Error ? err.message : "Status check failed",
        );
      }
    });
}
