import { Command } from "commander";

const program = new Command()
  .name("agent-notion")
  .description("Notion CLI for humans and LLMs")
  .version("0.1.0");

program.parse();
