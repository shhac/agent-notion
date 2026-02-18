import type { Command } from "commander";
import { registerQuery } from "./query.ts";
import { registerUsage } from "./usage.ts";

export function registerSearchCommand(program: Command): void {
  const search = program
    .command("search")
    .description("Search Notion by title (pages and databases)");

  registerQuery(search);
  registerUsage(search);
}
