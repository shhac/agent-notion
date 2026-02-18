import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerUsage } from "./usage.ts";

export function registerBacklinksCommand(program: Command): void {
  const backlinks = program
    .command("backlinks")
    .description("List pages that link to a given page (v3 desktop session required)");
  registerList(backlinks);
  registerUsage(backlinks);
}
