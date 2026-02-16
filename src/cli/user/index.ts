import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerMe } from "./me.ts";
import { registerUsage } from "./usage.ts";

export function registerUserCommand(program: Command): void {
  const user = program.command("user").description("User operations");
  registerList(user);
  registerMe(user);
  registerUsage(user);
}
