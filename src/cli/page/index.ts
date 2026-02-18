import type { Command } from "commander";
import { registerGet } from "./get.ts";
import { registerCreate } from "./create.ts";
import { registerUpdate } from "./update.ts";
import { registerArchive } from "./archive.ts";
import { registerBacklinks } from "./backlinks.ts";
import { registerHistory } from "./history.ts";
import { registerUsage } from "./usage.ts";

export function registerPageCommand(program: Command): void {
  const page = program.command("page").description("Page operations");
  registerGet(page);
  registerCreate(page);
  registerUpdate(page);
  registerArchive(page);
  registerBacklinks(page);
  registerHistory(page);
  registerUsage(page);
}
