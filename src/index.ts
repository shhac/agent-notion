import { Command } from "commander";
import { registerAuthCommand } from "./cli/auth/index.ts";

const program = new Command()
  .name("agent-notion")
  .description("Notion CLI for humans and LLMs")
  .version("0.1.0");

registerAuthCommand(program);

program.parse();
