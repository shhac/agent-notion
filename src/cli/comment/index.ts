import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerPage, registerInline } from "./add.ts";
import { registerUsage } from "./usage.ts";

export function registerCommentCommand(program: Command): void {
  const comment = program.command("comment").description("Comment operations");
  registerList(comment);
  registerPage(comment);
  registerInline(comment);
  registerUsage(comment);
}
