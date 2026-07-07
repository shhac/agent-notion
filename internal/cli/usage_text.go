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
  search     query <query> — search page/database titles
  page       get | create | update | trash* | restore | archive* | unarchive |
             backlinks | history
  block      list | append | update | replace | move | delete*
  database   list | get | schema | query
  comment    list | page | inline
  user       list | me
  export     page | workspace | poll
  activity   log
  ai         chat-send | chat-list | chat-get | chat-mark-read | model-list
  config     get <key> | set <key> <value> | unset <key> | list
  usage      this overview; '<group> usage' has per-domain detail
  * = destructive: requires --yes, otherwise returns what WOULD happen

BACKENDS
  Two API backends: the official REST API (integration tokens, OAuth) and
  the v3 desktop-session API (auth import-desktop). --backend auto (default)
  prefers the v3 session when one is stored; force with --backend official|v3.
  export/activity/backlinks/history/ai and real Archive need the v3 session.

OUTPUT
  One JSON record per line on stdout (NDJSON). --format json|yaml for pretty.
  Errors on stderr as {error, fixable_by, hint} with exit 1;
  fixable_by is agent|human|retry. Tokens are never printed.
  Fields named description/body/content are truncated (default 200 chars,
  'config set truncation.max_length' overrides) with a {field}Length
  companion showing the full size; --expand <fields> or --full lifts it.`

// domainUsage maps a command-group name to its detail card. Group files
// populate it via addDomainUsage from their registerX functions.
var domainUsage = map[string]string{}
