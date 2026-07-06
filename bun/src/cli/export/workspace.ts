import { defineCommand, type Command } from "../../lib/cli.ts";
import { createV3Client } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { exportAndDownload, defaultExportFilename, validateFormat } from "./poll.ts";

export function registerWorkspace(parent: Command): void {
  parent.addCommand(
    defineCommand({
      use: "workspace",
      short: "Export the entire workspace to markdown or HTML",
      options: {
        format: {
          type: "string",
          default: "markdown",
          description: "Export format: markdown or html",
        },
        output: { type: "string", description: "Output file path" },
        timeout: {
          type: "string",
          default: "600",
          description: "Maximum time to wait for export (seconds)",
        },
      },
      action: async (_args, opts) => {
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

          const timeoutSec = parseInt(opts.timeout, 10);
          if (isNaN(timeoutSec) || timeoutSec <= 0) {
            throw new CliError("--timeout must be a positive number (seconds)");
          }

          const output = opts.output ?? defaultExportFilename();
          const result = await exportAndDownload(client, task, output, {
            timeout: timeoutSec * 1000,
            pollInterval: 5_000,
          });

          printJson({
            exported: result.path,
            format,
            pagesExported: result.pagesExported,
          });
        });
      },
    }),
  );
}
