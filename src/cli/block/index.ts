import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerAppend } from "./append.ts";
import { registerUsage } from "./usage.ts";

export function registerBlockCommand(program: Command): void {
  const block = program.command("block").description("Block (content) operations");
  registerList(block);
  registerAppend(block);
  registerUsage(block);
}
