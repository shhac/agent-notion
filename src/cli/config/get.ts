import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { readConfig } from "../../lib/config.ts";
import { SETTING_DEFS, VALID_KEYS } from "./index.ts";

export function registerGet(config: Command): void {
  config
    .command("get")
    .argument("[key]", "Setting key (omit to show all)")
    .description("Show current settings")
    .action((key?: string) => {
      const cfg = readConfig();
      const settings = cfg.settings ?? {};

      if (!key) {
        printJson(settings);
        return;
      }

      const def = SETTING_DEFS[key];
      if (!def) {
        printError(`Unknown setting: ${key}. Valid keys: ${VALID_KEYS.join(", ")}`);
        return;
      }

      const value = def.get(settings);
      printJson({ [key]: value ?? null });
    });
}
