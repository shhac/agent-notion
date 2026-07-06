import { Command, type Command as VipvotCommand } from "vipvot";
import { registerLog } from "./log.ts";
import { registerUsage } from "./usage.ts";

export function registerActivityCommand(program: VipvotCommand): void {
  const activity = Command({
    use: "activity",
    short: "Workspace and page activity (v3 desktop session required)",
  });
  registerLog(activity);
  registerUsage(activity);
  program.addCommand(activity);
}
