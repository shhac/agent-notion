import type { Command } from "commander";

const USAGE_TEXT = `agent-notion config — View and update persistent CLI settings

SUBCOMMANDS:
  config get [key]            Show current settings (omit key for all)
  config set <key> <value>    Update a setting
  config reset [key]          Reset to defaults (omit key to reset all)
  config list-keys            List all available setting keys with descriptions

SETTING KEYS:
  truncation.maxLength         Max chars before truncating description/body/content fields
                               Default: 200. Must be a non-negative integer (0 = no truncation).
  pagination.defaultPageSize   Default number of results for list commands
                               Default: 50. Must be an integer between 1 and 100 (Notion API max).

EXAMPLES:
  config set truncation.maxLength 500       Show more content before truncating
  config set pagination.defaultPageSize 20  Fetch fewer results per page
  config get truncation.maxLength           Check current truncation setting
  config reset truncation.maxLength         Reset truncation to default (200)
  config reset                              Reset all settings to defaults

STORAGE: Settings persisted in ~/.config/agent-notion/config.json alongside auth credentials.

OUTPUT: JSON to stdout. Unknown keys return error with valid key list.
`;

export function registerUsage(config: Command): void {
  config
    .command("usage")
    .description("Print detailed config documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
