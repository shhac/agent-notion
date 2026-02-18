import type { Command } from "commander";

const USAGE_TEXT = `agent-notion history — Version history (snapshots) of a page

USAGE:
  history <page-id> [--limit <n>]

  Requires a v3 desktop session (auth import-desktop).

ARGUMENTS:
  <page-id>    Page UUID or dashless ID

OPTIONS:
  --limit <n>    Number of snapshots to fetch (default: 20)

OUTPUT:
  { "snapshots": [{ id, version, lastVersion, timestamp, authors }], "total": <n> }

  id: Snapshot identifier
  version: Version number of this snapshot
  lastVersion: Previous version number
  timestamp: ISO-8601 timestamp of the snapshot
  authors: Array of user IDs who contributed to this version

EXAMPLES:
  history abc123                       Recent snapshots for a page
  history abc123 --limit 50            Fetch more snapshots
`;

export function registerUsage(history: Command): void {
  history
    .command("usage")
    .description("Print detailed history documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
