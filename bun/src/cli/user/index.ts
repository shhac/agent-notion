import { Command, type Command as VipvotCommand } from "vipvot";
import { registerList } from "./list.ts";
import { registerMe } from "./me.ts";
import { registerUsage } from "./usage.ts";

export function registerUserCommand(program: VipvotCommand): void {
  const user = Command({ use: "user", short: "User operations" });
  registerList(user);
  registerMe(user);
  registerUsage(user);
  program.addCommand(user);
}
