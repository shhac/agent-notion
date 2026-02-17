import type { Command } from "commander";
import { createV3Client } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";
import { exportAndDownload, defaultExportFilename, type ExportFormat } from "./poll.ts";

export function registerPage(parent: Command): void {
  parent
    .command("page")
    .description("Export a page (and optionally subpages) to markdown or HTML")
    .argument("<page-id>", "Page UUID or dashless ID")
    .option("--format <format>", "Export format: markdown or html", "markdown")
    .option("--recursive", "Include subpages recursively", false)
    .option("--output <path>", "Output file path")
    .option(
      "--timeout <seconds>",
      "Maximum time to wait for export (seconds)",
      "120",
    )
    .action(
      async (
        rawPageId: string,
        opts: {
          format: string;
          recursive: boolean;
          output?: string;
          timeout: string;
        },
      ) => {
        await handleAction(async () => {
          const pageId = normalizeId(rawPageId);
          const format = validateFormat(opts.format);
          const client = createV3Client();

          const task = {
            eventName: "exportBlock",
            request: {
              block: { id: pageId, spaceId: client.spaceId_ },
              recursive: opts.recursive,
              exportOptions: {
                exportType: format,
                timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
                locale: "en",
                flattenExportFiletree: false,
              },
              shouldExportComments: false,
            },
          };

          const output = opts.output ?? defaultExportFilename();
          const result = await exportAndDownload(client, task, output, {
            timeout: parseInt(opts.timeout, 10) * 1000,
          });

          printJson({
            exported: result.path,
            format,
            pagesExported: result.pagesExported,
            recursive: opts.recursive,
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
