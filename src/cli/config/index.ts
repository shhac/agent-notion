import type { Command } from "commander";
import type { Settings } from "../../lib/config.ts";
import { registerGet } from "./get.ts";
import { registerSet } from "./set.ts";
import { registerReset } from "./reset.ts";
import { registerListKeys } from "./list-keys.ts";
import { registerUsage } from "./usage.ts";

export type SettingDef = {
  /** How to read/write this setting from Settings object */
  get: (s: Settings) => unknown;
  set: (s: Settings, v: unknown) => void;
  reset: (s: Settings) => void;
  parse: (v: string) => unknown;
  description: string;
  default: unknown;
};

export const SETTING_DEFS: Record<string, SettingDef> = {
  "truncation.maxLength": {
    get: (s) => s.truncation?.max_length,
    set: (s, v) => {
      if (!s.truncation) s.truncation = {};
      s.truncation.max_length = v as number;
    },
    reset: (s) => {
      if (s.truncation) delete s.truncation.max_length;
    },
    parse: (v) => {
      const n = Number(v);
      if (!Number.isInteger(n) || n < 0) {
        throw new Error(`Invalid value: ${v}. Must be a non-negative integer.`);
      }
      return n;
    },
    description:
      "Max characters before truncating description/body/content fields (default: 200, 0 = no truncation)",
    default: 200,
  },
  "pagination.defaultPageSize": {
    get: (s) => s.page_size,
    set: (s, v) => {
      s.page_size = v as number;
    },
    reset: (s) => {
      delete s.page_size;
    },
    parse: (v) => {
      const n = Number(v);
      if (!Number.isInteger(n) || n < 1 || n > 100) {
        throw new Error(
          `Invalid value: ${v}. Must be an integer between 1 and 100 (Notion API max).`,
        );
      }
      return n;
    },
    description: "Default number of results for list commands (default: 50, max: 100)",
    default: 50,
  },
  "ai.defaultModel": {
    get: (s) => s.ai?.default_model,
    set: (s, v) => {
      if (!s.ai) s.ai = {};
      s.ai.default_model = v as string;
    },
    reset: (s) => {
      if (s.ai) delete s.ai.default_model;
    },
    parse: (v) => {
      if (!v || !v.trim()) {
        throw new Error("Model name cannot be empty. Use 'ai model list' to see available models.");
      }
      return v.trim();
    },
    description:
      "Default AI model codename (e.g., oatmeal-cookie). Use 'ai model list' to see options.",
    default: undefined,
  },
};

export const VALID_KEYS = Object.keys(SETTING_DEFS);

export function registerConfigCommand(program: Command): void {
  const config = program.command("config").description("View and update CLI settings");
  registerGet(config);
  registerSet(config);
  registerReset(config);
  registerListKeys(config);
  registerUsage(config);
}
