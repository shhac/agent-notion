import type { Command } from "commander";
import { createV3Client } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { exportAndDownload, defaultExportFilename, type ExportFormat } from "./poll.ts";

export function registerWorkspace(parent: Command): void {
  parent
    .command("workspace")
    .description("Export the entire workspace to markdown or HTML")
    .option("--format <format>", "Export format: markdown or html", "markdown")
    .option("--output <path>", "Output file path")
    .option(
      "--timeout <seconds>",
      "Maximum time to wait for export (seconds)",
      "600",
    )
    .action(
      async (opts: { format: string; output?: string; timeout: string }) => {
        await handleAction(async () => {
          const format = validateFormat(opts.format);
          const client = createV3Client();

          const task = {
            eventName: "exportSpace",
            request: {
              spaceId: client.spaceId_,
              exportOptions: {
                exportType: format,
                timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
                locale: "en",
              },
              shouldExportComments: false,
            },
          };

          const output = opts.output ?? defaultExportFilename();
          const result = await exportAndDownload(client, task, output, {
            timeout: parseInt(opts.timeout, 10) * 1000,
            pollInterval: 5_000, // workspace exports are slower, poll less frequently
          });

          printJson({
            exported: result.path,
            format,
            pagesExported: result.pagesExported,
          });
        });
      },
    );
}

function validateFormat(format: string): ExportFormat {
  if (format === "markdown" || format === "html") return format;
  throw new Error(
    `Invalid format "${format}". Use "markdown" or "html".`,
  );
}
