import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerUsage } from "./usage.ts";

export function registerActivityCommand(program: Command): void {
  const activity = program
    .command("activity")
    .description("Show recent activity log (v3 desktop session required)");
  registerList(activity);
  registerUsage(activity);
}
