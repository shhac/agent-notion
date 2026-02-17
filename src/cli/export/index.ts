import type { Command } from "commander";
import { registerPage } from "./page.ts";
import { registerWorkspace } from "./workspace.ts";

export function registerExportCommand(program: Command): void {
  const exp = program
    .command("export")
    .description("Export pages or workspace (v3 desktop session required)");
  registerPage(exp);
  registerWorkspace(exp);
}
