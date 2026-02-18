import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerUsage } from "./usage.ts";

export function registerHistoryCommand(program: Command): void {
  const history = program
    .command("history")
    .description("List version history of a page (v3 desktop session required)");
  registerList(history);
  registerUsage(history);
}
