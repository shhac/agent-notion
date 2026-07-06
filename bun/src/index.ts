import { Command, ref } from "vipvot";
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
import { registerExportCommand } from "./cli/export/index.ts";
import { registerActivityCommand } from "./cli/activity/index.ts";
import { registerAiCommand } from "./cli/ai/index.ts";
import { registerUsageCommand } from "./cli/usage/index.ts";
import { getPackageVersion } from "./lib/version.ts";

const expand = ref("");
const full = ref(false);

const program = Command({
  use: "agent-notion",
  short: "Notion CLI for humans and LLMs",
  version: getPackageVersion(),
  persistentPreRun: () => {
    const config = readConfig();
    configureTruncation({
      expand: expand.value || undefined,
      full: full.value,
      maxLength: config.settings?.truncation?.max_length,
    });
  },
});

program
  .persistentFlags()
  .stringVarP(
    expand,
    "expand",
    "",
    "",
    "Expand truncated fields (comma-separated: description,body,content)",
  );
program
  .persistentFlags()
  .boolVarP(full, "full", "", false, "Show full content for all truncated fields");

registerAuthCommand(program);
registerSearchCommand(program);
registerDatabaseCommand(program);
registerPageCommand(program);
registerBlockCommand(program);
registerCommentCommand(program);
registerUserCommand(program);
registerExportCommand(program);
registerActivityCommand(program);
registerAiCommand(program);
registerConfigCommand(program);
registerUsageCommand(program);

await program.execute();
