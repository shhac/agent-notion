import { Command, type Command as VipvotCommand } from "vipvot";
import { registerPage } from "./page.ts";
import { registerWorkspace } from "./workspace.ts";
import { registerUsage } from "./usage.ts";

export function registerExportCommand(program: VipvotCommand): void {
  const exp = Command({
    use: "export",
    short: "Export pages or workspace (v3 desktop session required)",
  });
  registerPage(exp);
  registerWorkspace(exp);
  registerUsage(exp);
  program.addCommand(exp);
}
