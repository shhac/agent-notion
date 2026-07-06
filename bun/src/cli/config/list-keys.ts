import { defineCommand, type Command } from "../../lib/cli.ts";
import { printJson } from "../../lib/output.ts";
import { SETTING_DEFS } from "./index.ts";

export function registerListKeys(config: Command): void {
  config.addCommand(
    defineCommand({
      use: "list-keys",
      short: "List all available setting keys",
      action: () => {
        const keys = Object.entries(SETTING_DEFS).map(([key, def]) => ({
          key,
          description: def.description,
          default: def.default,
        }));
        printJson({ keys });
      },
    }),
  );
}
