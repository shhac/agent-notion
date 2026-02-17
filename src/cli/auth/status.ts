import { Command } from "commander";
import {
  getDefaultWorkspace,
  getOAuthConfig,
  getV3Session,
  getWorkspaces,
  resolveV3Token,
} from "../../lib/config.ts";
import { resolveAccessToken } from "../../lib/credentials.ts";
import { printError, printJson } from "../../lib/output.ts";
import { createNotionClientWithToken } from "../../notion/client.ts";
import { V3HttpClient } from "../../notion/v3/client.ts";

export function registerStatus(parent: Command): void {
  parent
    .command("status")
    .description("Show authentication state")
    .action(async () => {
      try {
        const oauthConfigured = !!getOAuthConfig();

        // Priority 1: v3 desktop session (matches createBackend() order)
        const v3Session = getV3Session();
        const v3Token = v3Session ? resolveV3Token() : undefined;

        if (v3Session && v3Token) {
          const http = new V3HttpClient({
            tokenV2: v3Token,
            userId: v3Session.user_id,
            spaceId: v3Session.space_id,
          });

          try {
            await http.loadUserContent();
          } catch {
            printJson({
              authenticated: false,
              auth_type: "desktop",
              user: {
                name: v3Session.user_name || undefined,
                email: v3Session.user_email || undefined,
              },
              workspace: {
                name: v3Session.space_name || undefined,
                id: v3Session.space_id || undefined,
              },
              extracted_at: v3Session.extracted_at,
              error:
                "Desktop token is present but invalid or expired.",
              hint: "Run 'agent-notion auth import-desktop' to re-extract your Notion desktop token.",
            });
            return;
          }

          // v3 token is valid — check for additional OAuth/integration creds
          const cred = resolveAccessToken();

          printJson({
            authenticated: true,
            auth_type: "desktop",
            user: {
              id: v3Session.user_id,
              name: v3Session.user_name || undefined,
              email: v3Session.user_email || undefined,
            },
            workspace: {
              name: v3Session.space_name || undefined,
              id: v3Session.space_id || undefined,
            },
            extracted_at: v3Session.extracted_at,
            other_credentials: cred
              ? {
                  source: cred.source,
                  workspace: cred.workspace,
                  auth_type: cred.auth_type,
                }
              : undefined,
            oauth_configured: oauthConfigured,
          });
          return;
        }

        // Priority 2: OAuth / internal integration token
        const cred = resolveAccessToken();

        if (!cred) {
          printJson({
            authenticated: false,
            oauth_configured: oauthConfigured,
            hint: "Run 'agent-notion auth import-desktop' to use your Notion desktop token, 'agent-notion auth setup-oauth' to configure OAuth, or 'agent-notion auth login --token <token>' for internal integrations.",
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
