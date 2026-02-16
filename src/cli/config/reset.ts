import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { readConfig, writeConfig } from "../../lib/config.ts";
import { SETTING_DEFS, VALID_KEYS } from "./index.ts";

export function registerReset(config: Command): void {
  config
    .command("reset")
    .argument("[key]", "Setting key (omit to reset all)")
    .description("Reset settings to defaults")
    .action((key?: string) => {
      const cfg = readConfig();

      if (!key) {
        cfg.settings = {};
        writeConfig(cfg);
        printJson({ reset: "all" });
        return;
      }

      const def = SETTING_DEFS[key];
      if (!def) {
        printError(`Unknown setting: ${key}. Valid keys: ${VALID_KEYS.join(", ")}`);
        return;
      }

      if (cfg.settings) {
        def.reset(cfg.settings);
        writeConfig(cfg);
      }

      printJson({ reset: key });
    });
}
