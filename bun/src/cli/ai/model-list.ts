import { defineCommand, type Command } from "../../lib/cli.ts";
import { createV3Client } from "../../notion/client.ts";
import { getV3Session } from "../../lib/config.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { getAvailableModels } from "../../notion/v3/ai.ts";

export function registerModelList(model: Command): void {
  model.addCommand(
    defineCommand({
      use: "list",
      short: "List available AI models",
      options: {
        raw: { type: "bool", description: "Return full model objects including codename" },
      },
      action: async (_args, opts) => {
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
      },
    }),
  );
}
