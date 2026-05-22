import { Command, type Command as VipvotCommand } from "vipvot";
import { registerGet } from "./get.ts";
import { registerCreate } from "./create.ts";
import { registerUpdate } from "./update.ts";
import { registerTrash } from "./trash.ts";
import { registerRestore } from "./restore.ts";
import { registerArchive } from "./archive.ts";
import { registerUnarchive } from "./unarchive.ts";
import { registerBacklinks } from "./backlinks.ts";
import { registerHistory } from "./history.ts";
import { registerUsage } from "./usage.ts";

export function registerPageCommand(program: VipvotCommand): void {
  const page = Command({ use: "page", short: "Page operations" });
  registerGet(page);
  registerCreate(page);
  registerUpdate(page);
  registerTrash(page);
  registerRestore(page);
  registerArchive(page);
  registerUnarchive(page);
  registerBacklinks(page);
  registerHistory(page);
  registerUsage(page);
  program.addCommand(page);
}
