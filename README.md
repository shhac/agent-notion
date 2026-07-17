# agent-notion

Notion CLI for humans and LLMs.

- **NDJSON output** — one JSON record per line on stdout; structured `{error, fixable_by, hint}` on stderr
- **LLM-optimized** — `agent-notion usage` (and `<group> usage`) print concise docs for agent consumption
- **Full CRUD** — search, read, create, update, trash/restore, and archive pages, databases, and blocks
- **AI chat** — send messages to Notion AI, list models and threads, stream responses
- **Markdown conversion** — page content rendered as markdown (tables become GitHub-flavored pipe tables), append content from markdown
- **Two backends** — official REST API (integration tokens, OAuth) and the v3 desktop-session API
- **Zero runtime deps** — a single static Go binary

**Website:** [agent-notion.paulie.app](https://agent-notion.paulie.app/)

## Installation

```bash
brew install shhac/tap/agent-notion
```

### Claude Code / AI agent skill

```bash
npx skills add shhac/agent-skills --skill agent-notion --global
```

Installs the `agent-notion` skill globally so Claude Code (and other AI agents) can discover and use it automatically. It ships from [`shhac/agent-skills`](https://github.com/shhac/agent-skills) — the whole family's skills in one repo, so `npx skills update` checks a single source no matter how many you use. Want several at once? Run `npx skills add shhac/agent-skills --global` and pick from the list.

## Quick start

### 1. Authenticate

**OAuth (recommended for full access):**

```bash
agent-notion auth setup-oauth --client-id <id> --client-secret <secret>
agent-notion auth login
```

**Internal integration token:**

```bash
printf '%s' "$NOTION_TOKEN" | agent-notion auth import   # token read from stdin
```

**Desktop session (for v3 features):**

```bash
agent-notion auth import-desktop                # reads token_v2 from the Notion Desktop app
```

### 2. Search and explore

```bash
agent-notion search query "meeting notes"
agent-notion database list
agent-notion database schema <database-id>
```

### 3. Read pages

```bash
agent-notion page get <page-id>                    # properties only
agent-notion page get <page-id> --content          # with markdown content
agent-notion database query <database-id>          # rows (one NDJSON record each)
```

### 4. Create and update

```bash
agent-notion page create --parent <id> --title "New Page"
agent-notion page update <page-id> --properties '{"Status":"Done"}'
agent-notion block append <page-id> --content "## New Section\n\nContent here."
agent-notion block update <block-id> --content "Updated text"
agent-notion block delete <block-id> --yes
agent-notion block replace <page-id> --content "# Fresh Content\n\nReplaced everything." --yes
agent-notion comment page <page-id> "Looks good!"
```

## Command map

`◆` = requires the v3 desktop session; `--yes` on a signature marks a destructive command that refuses to run without it.

```text
agent-notion [--backend auto|official|v3] [--format json|yaml|jsonl] [--full] [--expand <fields>]
├── search
│   ├── query <query> [--filter page|database] [--limit] [--cursor]
│   └── usage
├── database
│   ├── list [--limit] [--cursor]
│   ├── get <database-id>
│   ├── query <database-id> [--filter <json>] [--sort <json>] [--limit] [--cursor]
│   ├── schema <database-id>
│   └── usage
├── page
│   ├── get <page-id> [--content] [--raw-content]
│   ├── create --parent <id> --title <title> [--properties <json>] [--icon <emoji>]
│   ├── update <page-id> [--title] [--properties <json>] [--icon <emoji>]
│   ├── trash <page-id> --yes
│   ├── restore <page-id>
│   ├── archive <page-id> --yes                                     ◆
│   ├── unarchive <page-id>                                         ◆
│   ├── backlinks <page-id>                                         ◆
│   ├── history <page-id> [--limit]                                 ◆
│   └── usage
├── block
│   ├── list <page-id> [--raw] [--limit] [--cursor]
│   ├── append <page-id> [--content <markdown>] [--blocks <json>]
│   ├── update <block-id> --content <text>
│   ├── delete <block-id> --yes
│   ├── move <block-id> [--parent <block-id>] [--after <block-id>]  ◆
│   ├── replace <page-id> [--content <markdown>] [--blocks <json>] --yes
│   └── usage
├── comment
│   ├── list <page-id> [--limit] [--cursor]
│   ├── page <page-id> <body>
│   ├── inline <block-id> <body> --text <target> [--occurrence <n>] ◆
│   └── usage
├── export                                                          ◆
│   ├── page <page-id> [--format md|html] [--recursive] [--output] [--wait]
│   ├── workspace [--format md|html] [--output] [--wait]
│   ├── poll <task-id> [--output] [--wait]
│   └── usage
├── user
│   ├── list [--limit] [--cursor]
│   ├── me
│   └── usage
├── activity                                                        ◆
│   ├── log [--page <page-id>] [--limit]
│   └── usage
├── ai                                                              ◆
│   ├── model list [--raw]
│   ├── chat list [--limit]
│   ├── chat send <message> [--thread] [--model] [--page] [--no-search] [--read-only] [--stream]
│   ├── chat get <thread-id> [--raw]
│   ├── chat mark-read <thread-id>
│   └── usage
├── config
│   ├── get <key>
│   ├── set <key> <value>
│   ├── unset <key>
│   ├── list
│   └── usage
├── auth
│   ├── setup-oauth --client-id <id> --client-secret <secret>
│   ├── login [--alias <name>] [--port <port>]
│   ├── import [--token <token>] [--alias <name>]
│   ├── logout [--all] [--workspace <alias>] --yes
│   ├── status
│   ├── import-desktop [--skip-validation]                          ◆
│   ├── import-browser <browser> [--profile <p>]                    ◆
│   ├── workspace list | switch <alias> | set-default <alias> | remove <alias> --yes
│   └── usage
└── usage                              # LLM-optimized overview
```

Each command group has a `usage` subcommand for detailed, LLM-friendly documentation (e.g. `agent-notion page usage`). The top-level `agent-notion usage` gives a broad overview.

## Backends

Two API backends serve the same command surface:

- **Official REST API** — integration tokens and OAuth (`auth import`, `auth login`).
- **v3 desktop-session API** — `auth import-desktop` / `auth import-browser`. Powers `export`, `page backlinks`/`history`, `activity log`, real `page archive`/`unarchive`, `block move`, `comment inline`, and `ai`.

`--backend auto` (default) prefers a stored v3 session, else the official credential. Force one with `--backend official` or `--backend v3`.

## Authentication

### OAuth (recommended)

OAuth provides full access to any workspace the user authorizes. Requires a [Notion integration](https://www.notion.so/profile/integrations) with OAuth enabled.

```bash
agent-notion auth setup-oauth --client-id <id> --client-secret <secret>
agent-notion auth login                        # opens the browser
agent-notion auth login --alias work           # multiple workspaces
```

Client secrets are stored in the OS keychain when available, falling back to plaintext config (a warning field says which).

### Internal integration token

For simpler setups or CI environments, use an [internal integration token](https://www.notion.so/profile/integrations). Pipe it on stdin so the secret stays off argv, shell history, and any transcript:

```bash
printf '%s' "$NOTION_TOKEN" | agent-notion auth import   # token read from stdin
```

The token is validated against the API before storing, and the alias is derived from the workspace name (`--alias` overrides). A `--token` flag also works for typing directly in your own terminal, but avoid it in shared or automated contexts where argv is logged. If you are driving an agent, do not hand it a pasted token to place on `--token` — have the agent instruct you to run the import yourself so the secret never enters its context.

### Workspace management

```bash
agent-notion auth status                       # current auth state (never prints tokens)
agent-notion auth workspace list               # all stored workspaces
agent-notion auth workspace switch <alias>     # change default
agent-notion auth workspace remove <alias> --yes
agent-notion auth logout --all --yes           # clear everything
```

Token resolution order: `NOTION_API_KEY`/`NOTION_TOKEN` env var > default workspace token (OS keychain, else config file).

## Advanced commands (v3 API)

Some commands use Notion's internal v3 API instead of the public API. They provide capabilities not available through official integrations but require a desktop session token:

```bash
agent-notion auth import-desktop               # reads token_v2 from the Notion Desktop app
agent-notion auth import-browser chrome        # or from a browser cookie store
```

`import-browser` supports chrome, brave, edge, arc, chromium, firefox, zen, safari (`--profile <p>`).

v3 commands (marked with `◆` in the command map):

| Command | Description |
| ------- | ----------- |
| `export page <id> [--recursive]` | Export page (or page tree) to markdown/HTML zip |
| `export workspace` | Export the entire workspace |
| `export poll <task-id>` | Resume/poll a queued export by task ID |
| `page archive <id> --yes` / `page unarchive <id>` | Real Archive: hide from search, keep the page alive |
| `page backlinks <page-id>` | Find pages that link to a given page |
| `page history <page-id>` | Version history snapshots |
| `activity log [--page <id>]` | Workspace or page activity log |
| `block move <block-id> [--parent <id>] [--after <id>]` | Reorder blocks or move into containers |
| `comment inline <block-id> <body> --text <t>` | Inline comment anchored to specific text |
| `ai model list` | List available AI models |
| `ai chat list` | List recent AI chat threads |
| `ai chat send <message>` | Send a message to Notion AI (`--read-only` for ask/answer only; supports `--stream`) |
| `ai chat get <thread-id>` | Get thread content (messages and metadata) |
| `ai chat mark-read <thread-id>` | Mark a chat thread as read |

Run `<command> usage` for full options and output format (e.g. `agent-notion export usage`).

## Output

- **NDJSON on stdout** — one JSON record per line. List commands print one record per item, then a trailing `{"@pagination": {has_more, next_cursor}}` (or `{"@meta": …}` / `{"@total": n}`) line when there is more.
- **`--format json|yaml`** wraps everything in one pretty `{ "data": [ … ] }` envelope; `--format jsonl` is the default NDJSON.
- **Errors** print to stderr as `{ "error": "...", "fixable_by": "agent|human|retry", "hint": "..." }` with exit code 1. Tokens are never printed.
- Empty/null fields are pruned automatically — a missing key means no value.

## Truncation

Fields named `description`, `body`, or `content` are truncated to 200 characters by default. A companion `{field}Length` key (e.g. `descriptionLength`) always carries the full rune count so you can detect clipping.

```bash
agent-notion --full page get <page-id>                             # expand every truncatable field
agent-notion --expand description database get <id>                # expand specific fields
agent-notion config set truncation.max_length 500                  # raise the default cap
```

`--expand`/`--full` are global flags and may appear before or after the command.

## Configuration

Persistent settings via `agent-notion config` (`get`/`set`/`unset`/`list`):

| Key                     | Default | Description                                            |
| ----------------------- | ------- | ------------------------------------------------------ |
| `page_size`             | 50      | Default results per list command (integer 1–100)       |
| `max_depth`             | —       | Max nesting depth when recursively fetching blocks     |
| `truncation.max_length` | 200     | Max characters before truncating description/body/content |
| `ai.default_model`      | —       | Default AI model codename (see `ai model list --raw`)  |

```bash
agent-notion config set truncation.max_length 500
agent-notion config get page_size
agent-notion config list                   # every key with value + description
agent-notion config unset truncation.max_length
```

Settings persist in `~/.config/agent-notion/config.json`; clearing every key drops the `settings` object entirely.

## Environment variables

| Variable                   | Description                                          |
| -------------------------- | --------------------------------------------------- |
| `NOTION_API_KEY` / `NOTION_TOKEN` | Notion API token (overrides stored credentials)     |
| `XDG_CONFIG_HOME`          | Override the config directory (default `~/.config`)  |
| `AGENT_NOTION_NO_KEYCHAIN` | Set to `1` to skip the OS keychain (config-file only) |

## Development

```bash
make build                   # build the binary
make dev ARGS="--help"       # run from source
make test                    # unit tests (no external calls)
make vet                     # go vet
make lint                    # golangci-lint
```

## License

MIT
