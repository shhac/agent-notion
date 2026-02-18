import type { Command } from "commander";

const USAGE_TEXT = `agent-notion export — Export pages or workspace (v3 desktop session required)

SUBCOMMANDS:
  export page <page-id> [options]            Export a page to markdown or HTML
  export workspace [options]                 Export the entire workspace

PAGE OPTIONS:
  --format <format>      Export format: markdown or html (default: markdown)
  --recursive            Include subpages recursively (default: false)
  --output <path>        Output file path (default: notion-export-<timestamp>.zip)
  --timeout <seconds>    Maximum wait time for export (default: 120)

WORKSPACE OPTIONS:
  --format <format>      Export format: markdown or html (default: markdown)
  --output <path>        Output file path (default: notion-export-<timestamp>.zip)
  --timeout <seconds>    Maximum wait time for export (default: 600)

OUTPUT:
  Page:      { exported, format, pagesExported, recursive }
  Workspace: { exported, format, pagesExported }

  exported: Absolute path to the downloaded zip file
  Progress is written to stderr during polling.

NOTES:
  Requires a v3 desktop session (auth import-desktop).
  Exports are asynchronous — the CLI polls until completion or timeout.
  Large workspace exports may take several minutes.

EXAMPLES:
  export page abc123                                 Export single page as markdown
  export page abc123 --format html --recursive       Export page tree as HTML
  export workspace --output my-backup.zip            Export entire workspace
`;

export function registerUsage(exp: Command): void {
  exp
    .command("usage")
    .description("Print detailed export documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
