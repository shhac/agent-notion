import { Command, type Command as VipvotCommand } from "vipvot";
import { registerList } from "./list.ts";
import { registerAppend } from "./append.ts";
import { registerUpdate } from "./update.ts";
import { registerDelete } from "./delete.ts";
import { registerMove } from "./move.ts";
import { registerReplace } from "./replace.ts";
import { registerUsage } from "./usage.ts";

export function registerBlockCommand(program: VipvotCommand): void {
  const block = Command({ use: "block", short: "Block (content) operations" });
  registerList(block);
  registerAppend(block);
  registerUpdate(block);
  registerDelete(block);
  registerMove(block);
  registerReplace(block);
  registerUsage(block);
  program.addCommand(block);
}
