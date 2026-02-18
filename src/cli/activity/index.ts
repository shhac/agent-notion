import type { Command } from "commander";
import { registerLog } from "./log.ts";
import { registerUsage } from "./usage.ts";

export function registerActivityCommand(program: Command): void {
  const activity = program
    .command("activity")
    .description("Workspace and page activity (v3 desktop session required)");

  registerLog(activity);
  registerUsage(activity);
}
