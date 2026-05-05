import { Command, type Command as VipvotCommand } from "vipvot";
import { registerList } from "./list.ts";
import { registerPage, registerInline } from "./add.ts";
import { registerUsage } from "./usage.ts";

export function registerCommentCommand(program: VipvotCommand): void {
  const comment = Command({ use: "comment", short: "Comment operations" });
  registerList(comment);
  registerPage(comment);
  registerInline(comment);
  registerUsage(comment);
  program.addCommand(comment);
}
