package cli

// Static LLM-facing help content for `usage` and `<group> usage`, split out
// from the command wiring in usage.go so doc edits do not churn the wiring.
// Doc-lockstep rule: a change to commands/flags/output updates this file in
// the same commit.

const rootUsageText = `agent-notion: Notion CLI for AI agents. NDJSON out, structured errors, no interactivity.

COMMANDS
  auth       status | setup-oauth | login | import | logout* |
             workspace list/switch/set-default/remove* |
             import-desktop | import-browser <name>
  config     get <key> | set <key> <value> | unset <key> | list
  usage      this overview; 'auth usage'/'config usage' have per-domain detail
  * = destructive: requires --yes, otherwise returns what WOULD happen

(mid-migration: page, block, database, search, comment, export, user,
activity, and ai groups are being ported and land in upcoming releases)

OUTPUT
  One JSON record per line on stdout (NDJSON). --format json|yaml for pretty.
  Errors on stderr as {error, fixable_by, hint} with exit 1;
  fixable_by is agent|human|retry. Tokens are never printed.`

// domainUsage maps a command-group name to its detail card.
var domainUsage = map[string]string{
	"auth":   authUsageText,
	"config": configUsageText,
}

const configUsageText = `agent-notion config — View and update persistent CLI settings

SUBCOMMANDS
  config get <key>            Show one setting: {key, value, set}
  config set <key> <value>    Update a setting: {set, value}
  config unset <key>          Reset a setting to its default: {unset}
  config list                 List every key: {key, value, set, description}

SETTING KEYS
  page_size               Default results per list command. Integer 1-100 (Notion API max). Default 50.
  max_depth               Max nesting depth when recursively fetching blocks. Positive integer; unset = no limit.
  truncation.max_length   Max characters before truncating description/body/content. Positive integer; unset = default 200.
  ai.default_model        Default AI model codename (e.g. oatmeal-cookie). Use 'ai model list' for options.

EXAMPLES
  config set page_size 20                 Fetch fewer results per page
  config set truncation.max_length 500    Show more content before truncating
  config get truncation.max_length        Check the current value
  config unset truncation.max_length      Reset truncation to its default
  config list                             See all keys with descriptions

STORAGE
  Persisted in ~/.config/agent-notion/config.json alongside auth. Once every
  setting is cleared, the 'settings' object is dropped from the file entirely.

OUTPUT
  NDJSON on stdout. Unknown keys return {error, fixable_by:agent, hint} listing
  the valid keys.

NOTE
  This adopts the family-standard get/set/unset/list surface, replacing the TS
  'config reset'/'config list-keys'.`

const authUsageText = `agent-notion auth — Manage Notion authentication and workspaces

SUBCOMMANDS
  auth status                                                  Show the resolved credential (never prints tokens)
  auth setup-oauth --client-id <id> --client-secret <secret>   Store OAuth app credentials
  auth login [--alias <name>] [--port <port>]                  OAuth login flow (opens the browser)
  auth import [--token <token>] [--alias <name>]               Store an internal-integration token (stdin ok)
  auth logout [--all] [--workspace <alias>] --yes              Remove credentials
  auth workspace list                                          List workspaces (one NDJSON record each)
  auth workspace switch <alias>                                Set the default workspace
  auth workspace set-default <alias>                           Alias for switch
  auth workspace remove <alias> --yes                          Remove a workspace
  auth import-desktop [--skip-validation]                      Import token_v2 from the Notion Desktop app
  auth import-browser <browser> [--profile <p>]                Import token_v2 from a browser cookie store

AUTH SOURCES (checked in order)
  1. NOTION_API_KEY or NOTION_TOKEN environment variable
  2. Default workspace token — OS keychain, else config file
     (~/.config/agent-notion/config.json)

SETUP-OAUTH
  Register a public integration at https://www.notion.so/my-integrations and
  store its client credentials. The secret goes to the OS keychain when
  available, plaintext config otherwise (a warning field says which).
  Returns: {ok, oauth_configured, client_id, secret_storage}

LOGIN (OAuth)
  Requires setup-oauth first. Binds a localhost callback (default port 9876,
  falls forward to 9885), opens the browser for consent, exchanges the code,
  and stores the tokens. The first workspace becomes the default.
  Returns: {ok, storage, workspace: {alias, name, id, bot_id, default}, hint}

IMPORT (internal integration)
  Token from --token or stdin; ntn_ or secret_ prefix expected (warned
  otherwise). Validated against the API (users/me) before storing.
  Returns: {ok, storage, workspace: {alias, name, id, auth_type, default}}

LOGOUT / WORKSPACE REMOVE
  Destructive: refused without --yes. logout targets the default workspace,
  --workspace <alias> a specific one; --all wipes every workspace, the OAuth
  config, and all keychain entries.
  Returns: {ok, removed, remaining_workspaces, default_workspace?, warning?}
  Returns (--all): {ok, cleared: "all"}

WORKSPACE
  list: one record per workspace: {alias, name, auth_type, default}
  switch/set-default: {ok, default_workspace}

IMPORT-DESKTOP / IMPORT-BROWSER
  Read the token_v2 session cookie for Notion's unofficial API (used by
  history/activity/backlinks/ai once ported). Validated via getSpaces unless
  --skip-validation. Browsers: chrome, brave, edge, arc, chromium, firefox,
  zen, safari.
  Returns: {ok, storage, extracted_at, user?, email?, space?, space_id?, source?}

OUTPUT
  NDJSON records on stdout; errors on stderr as {error, fixable_by, hint}
  with exit 1. Tokens are never printed. OAuth access tokens refresh
  automatically once API commands land.`
