import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerAppend } from "./append.ts";
import { registerUpdate } from "./update.ts";
import { registerDelete } from "./delete.ts";
import { registerMove } from "./move.ts";
import { registerReplace } from "./replace.ts";
import { registerUsage } from "./usage.ts";

export function registerBlockCommand(program: Command): void {
  const block = program.command("block").description("Block (content) operations");
  registerList(block);
  registerAppend(block);
  registerUpdate(block);
  registerDelete(block);
  registerMove(block);
  registerReplace(block);
  registerUsage(block);
}
