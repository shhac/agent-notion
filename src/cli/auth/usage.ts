import type { Command } from "commander";

const USAGE_TEXT = `agent-notion auth — Manage Notion authentication and workspaces

SUBCOMMANDS:
  auth setup-oauth --client-id <id> --client-secret <secret>   Configure OAuth app credentials
  auth login [--alias <name>] [--port <port>]                  OAuth login flow (opens browser)
  auth login --token <ntn_xxx> [--alias <name>]                Internal integration login
  auth logout [--all] [--workspace <alias>]                    Remove credentials
  auth status                                                  Show authentication state
  auth workspace list                                          List configured workspaces
  auth workspace switch <alias>                                Switch active workspace
  auth workspace set-default <alias>                           Alias for switch
  auth workspace remove <alias>                                Remove a workspace

AUTH SOURCES (checked in order):
  1. NOTION_API_KEY or NOTION_TOKEN environment variable
  2. macOS Keychain (default workspace)
  3. Config file ~/.config/agent-notion/config.json (default workspace)

SETUP-OAUTH:
  Register your own Notion integration at https://www.notion.so/my-integrations
  as a public integration with OAuth. Provide the client_id and client_secret.
  Client secret is stored in macOS Keychain when available, config file otherwise.
  Returns: { ok, oauth_configured, client_id, secret_storage }

LOGIN (OAuth):
  Requires setup-oauth first. Starts localhost server on port 9876 (auto-selects 9876-9885).
  Opens browser to Notion authorization. After consent, exchanges code for tokens.
  Tokens stored in Keychain. First workspace auto-becomes default.
  Returns: { ok, workspace: { alias, name, id, bot_id, default }, hint }

LOGIN (Internal Integration):
  Pass --token with an API key from a Notion internal integration.
  Validates token against API before storing. No refresh token needed.
  Returns: { ok, workspace: { alias, name, id, auth_type, default } }

LOGOUT:
  Default: removes current default workspace credentials.
  --workspace <alias>: removes specific workspace.
  --all: removes all workspaces, OAuth config, and keychain entries.
  Returns: { ok, removed, remaining_workspaces, default_workspace }

STATUS:
  Validates current token against Notion API (not just checking presence).
  Shows credential source, workspace info, and other configured workspaces.
  Returns: { authenticated, source, user, workspace, other_workspaces, oauth_configured }

WORKSPACE:
  list: Returns { items: [{ alias, name, auth_type, default }] }
  switch/set-default: Returns { ok, default_workspace }
  remove: Returns { ok, removed, default_workspace }

OUTPUT:
  All commands return JSON to stdout on success (exit 0).
  Errors return { error: "<message>" } to stderr (exit 1).
  Token refresh is automatic on 401 for OAuth workspaces.`;

export function registerUsage(parent: Command): void {
  parent
    .command("usage")
    .description("Show detailed auth documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT);
    });
}
