import { defineCommand, MaximumNArgs, type Command } from "../../lib/cli.ts";
import { printJson, printError } from "../../lib/output.ts";
import { readConfig, writeConfig } from "../../lib/config.ts";
import { SETTING_DEFS, VALID_KEYS } from "./index.ts";

export function registerReset(config: Command): void {
  config.addCommand(
    defineCommand({
      use: "reset [key]",
      short: "Reset settings to defaults",
      args: MaximumNArgs(1),
      action: ([key]) => {
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
      },
    }),
  );
}
