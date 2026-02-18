# agent-notion

Notion CLI for humans and LLMs.

- **Structured JSON output** — all output is JSON to stdout, errors to stderr
- **LLM-optimized** — `agent-notion usage` prints concise docs for agent consumption
- **Full CRUD** — search, read, create, update, and archive pages and databases
- **Markdown conversion** — page content rendered as markdown, append content from markdown
- **Zero runtime deps** — single compiled binary via `bun build --compile`

## Installation

```bash
brew install shhac/tap/agent-notion
```

### Claude Code / AI agent skill

```bash
npx skills add shhac/agent-notion
```

This installs the `agent-notion` skill so Claude Code (and other AI agents) can discover and use `agent-notion` automatically. See [skills.sh](https://skills.sh) for details.

## Quick start

### 1. Authenticate

**OAuth (recommended for full access):**

```bash
agent-notion auth setup-oauth --client-id <id> --client-secret <secret>
agent-notion auth login
```

**Internal integration token:**

```bash
agent-notion auth login --token ntn_...
```

### 2. Search and explore

```bash
agent-notion search "meeting notes"
agent-notion database list
agent-notion database schema <database-id>
```

### 3. Read pages

```bash
agent-notion page get <page-id>                    # properties only
agent-notion page get <page-id> --content          # with markdown content
agent-notion database query <database-id>          # all rows
```

### 4. Create and update

```bash
agent-notion page create --parent <id> --title "New Page"
agent-notion page update <page-id> --properties '{"Status":"Done"}'
agent-notion block append <page-id> --content "## New Section\n\nContent here."
agent-notion comment page <page-id> "Looks good!"
```

## Command map

```text
agent-notion [--full] [--expand <fields>]
├── search <query> [--filter page|database] [--limit] [--cursor]
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
│   ├── archive <page-id>
│   └── usage
├── block
│   ├── list <page-id> [--raw] [--limit] [--cursor]
│   ├── append <page-id> [--content <markdown>] [--blocks <json>]
│   └── usage
├── comment
│   ├── list <page-id> [--limit] [--cursor]
│   ├── page <page-id> <body>
│   ├── inline <block-id> <body> --text <target> [--occurrence <n>]  ◆
│   └── usage
├── user
│   ├── list [--limit] [--cursor]
│   ├── me
│   └── usage
├── config
│   ├── get [key]
│   ├── set <key> <value>
│   ├── reset [key]
│   ├── list-keys
│   └── usage
├── auth
│   ├── setup-oauth --client-id <id> --client-secret <secret>
│   ├── login [--alias <name>] [--token <token>] [--port <port>]
│   ├── logout [--all] [--workspace <alias>]
│   ├── status
│   ├── workspace list | switch <alias> | remove <alias>
│   └── usage
└── usage                              # LLM-optimized docs
```

Each command group has a `usage` subcommand for detailed, LLM-friendly documentation (e.g., `agent-notion page usage`). The top-level `agent-notion usage` gives a broad overview.

## Authentication

### OAuth (recommended)

OAuth provides full access to any workspace the user authorizes. Requires a [Notion integration](https://www.notion.so/profile/integrations) with OAuth enabled.

```bash
agent-notion auth setup-oauth --client-id <id> --client-secret <secret>
agent-notion auth login                        # opens browser
agent-notion auth login --alias work           # multiple workspaces
```

Client secrets are stored in the macOS Keychain when available, falling back to plaintext config.

### Internal integration token

For simpler setups or CI environments, use an [internal integration token](https://www.notion.so/profile/integrations):

```bash
agent-notion auth login --token ntn_...
```

### Workspace management

```bash
agent-notion auth status                       # current auth state
agent-notion auth workspace list               # all stored workspaces
agent-notion auth workspace switch <alias>     # change default
agent-notion auth workspace remove <alias>
agent-notion auth logout --all                 # clear everything
```

Token resolution order: `NOTION_TOKEN` env > active workspace credentials.

## Advanced commands (v3 API)

Some commands use Notion's internal v3 API instead of the public API. These provide capabilities not available through official integrations but require a desktop session token:

```bash
agent-notion auth import-desktop               # macOS only — reads token from Notion Desktop app
```

v3 commands (marked with `◆` in the command map):

| Command | Description |
| ------- | ----------- |
| `export page <id> [--recursive]` | Export page (or page tree) to markdown/HTML zip |
| `export workspace` | Export entire workspace |
| `backlinks <page-id>` | Find pages that link to a given page |
| `history <page-id>` | Version history snapshots |
| `activity [--page <id>]` | Workspace or page activity log |
| `comment inline <block-id> <body> --text <t>` | Inline comment anchored to specific text |

Run `<command> usage` for full options and output format (e.g., `agent-notion export usage`).

## Output

- All output is JSON to stdout
- Errors go to stderr as `{ "error": "..." }` with non-zero exit code
- Empty/null fields are pruned automatically
- Long strings are truncated with companion `*Length` fields

## Truncation

Fields named `description`, `body`, or `content` are truncated to 200 characters by default. A companion `*Length` key (e.g. `descriptionLength`) shows the full size.

```bash
agent-notion --full page get <page-id>                             # expand all fields
agent-notion --expand description database get <id>                # expand specific fields
agent-notion config set truncation.maxLength 500                   # change default
```

## Configuration

Persistent settings via `agent-notion config`:

| Key                          | Default | Description                                           |
| ---------------------------- | ------- | ----------------------------------------------------- |
| `truncation.maxLength`       | 200     | Max characters before truncating (0 = no truncation)  |
| `pagination.defaultPageSize` | 50      | Default results per page (max 100)                    |

```bash
agent-notion config set truncation.maxLength 500
agent-notion config get pagination.defaultPageSize
agent-notion config list-keys              # all keys with defaults
agent-notion config reset                  # reset all to defaults
```

## Environment variables

| Variable       | Description                                      |
| -------------- | ------------------------------------------------ |
| `NOTION_TOKEN` | Notion API token (overrides stored credentials)  |
| `XDG_CONFIG_HOME` | Override config directory (default: `~/.config`) |

## Development

```bash
bun install
bun run dev -- --help        # run in dev mode
bun run tsc --noEmit         # type check
bun test                     # run tests
bun run lint                 # lint
```

## License

MIT
