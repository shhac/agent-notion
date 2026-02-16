import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { readConfig, writeConfig } from "../../lib/config.ts";
import { SETTING_DEFS, VALID_KEYS } from "./index.ts";

export function registerSet(config: Command): void {
  config
    .command("set")
    .argument("<key>", "Setting key")
    .argument("<value>", "Setting value")
    .description("Update a setting")
    .action((key: string, value: string) => {
      const def = SETTING_DEFS[key];
      if (!def) {
        printError(`Unknown setting: ${key}. Valid keys: ${VALID_KEYS.join(", ")}`);
        return;
      }

      let parsed: unknown;
      try {
        parsed = def.parse(value);
      } catch (err) {
        printError((err as Error).message);
        return;
      }

      const cfg = readConfig();
      if (!cfg.settings) {
        cfg.settings = {};
      }
      def.set(cfg.settings, parsed);
      writeConfig(cfg);
      printJson({ [key]: parsed });
    });
}
