import type { Command } from "commander";
import { createV3Client } from "../../notion/client.ts";
import { getV3Session } from "../../lib/config.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { getAvailableModels } from "../../notion/v3/ai.ts";

export function registerModelList(model: Command): void {
  model
    .command("list")
    .description("List available AI models")
    .option("--raw", "Return full model objects including codename")
    .action(async (opts: { raw?: boolean }) => {
      await handleAction(async () => {
        const client = createV3Client();
        const session = getV3Session()!;
        const models = await getAvailableModels(client, session.space_id);

        if (opts.raw) {
          printJson({ models });
          return;
        }

        const formatted = models
          .filter((m) => !m.isDisabled)
          .map((m) => ({
            name: m.modelMessage,
            family: m.modelFamily,
            tier: m.displayGroup,
          }));

        printJson({ models: formatted });
      });
    });
}
