import { Command } from "commander";
import { configureTruncation } from "./lib/truncation.ts";
import { readConfig } from "./lib/config.ts";
import { registerAuthCommand } from "./cli/auth/index.ts";
import { registerSearchCommand } from "./cli/search/index.ts";
import { registerDatabaseCommand } from "./cli/database/index.ts";
import { registerPageCommand } from "./cli/page/index.ts";
import { registerBlockCommand } from "./cli/block/index.ts";
import { registerCommentCommand } from "./cli/comment/index.ts";
import { registerUserCommand } from "./cli/user/index.ts";
import { registerConfigCommand } from "./cli/config/index.ts";
import { registerUsageCommand } from "./cli/usage/index.ts";

const program = new Command()
  .name("agent-notion")
  .description("Notion CLI for humans and LLMs")
  .version("0.1.0");

program.option(
  "--expand <fields>",
  "Expand truncated fields (comma-separated: description,body,content)",
);
program.option("--full", "Show full content for all truncated fields");

program.hook("preAction", (thisCommand) => {
  const opts = thisCommand.opts();
  const config = readConfig();
  configureTruncation({
    expand: opts.expand,
    full: opts.full,
    maxLength: config.settings?.truncation?.max_length,
  });
});

registerAuthCommand(program);
registerSearchCommand(program);
registerDatabaseCommand(program);
registerPageCommand(program);
registerBlockCommand(program);
registerCommentCommand(program);
registerUserCommand(program);
registerConfigCommand(program);
registerUsageCommand(program);

program.parse(process.argv);
if (!process.argv.slice(2).length) {
  program.outputHelp();
}
