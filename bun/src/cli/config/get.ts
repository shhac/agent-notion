import { defineCommand, MaximumNArgs, type Command } from "../../lib/cli.ts";
import { printJson, printError } from "../../lib/output.ts";
import { readConfig } from "../../lib/config.ts";
import { SETTING_DEFS, VALID_KEYS } from "./index.ts";

export function registerGet(config: Command): void {
  config.addCommand(
    defineCommand({
      use: "get [key]",
      short: "Show current settings",
      args: MaximumNArgs(1),
      action: ([key]) => {
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
      },
    }),
  );
}
