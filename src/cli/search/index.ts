import type { Command } from "commander";
import { registerRun } from "./run.ts";
import { registerUsage } from "./usage.ts";

export function registerSearchCommand(program: Command): void {
  const search = program
    .command("search")
    .description("Search Notion by title (pages and databases)");
  registerRun(search);
  registerUsage(search);
}
