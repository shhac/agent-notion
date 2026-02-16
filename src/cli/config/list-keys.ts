import type { Command } from "commander";
import { printJson } from "../../lib/output.ts";
import { SETTING_DEFS } from "./index.ts";

export function registerListKeys(config: Command): void {
  config
    .command("list-keys")
    .description("List all available setting keys")
    .action(() => {
      const keys = Object.entries(SETTING_DEFS).map(([key, def]) => ({
        key,
        description: def.description,
        default: def.default,
      }));
      printJson({ keys });
    });
}
