import { Command, type Command as VipvotCommand } from "vipvot";
import { registerQuery } from "./query.ts";
import { registerUsage } from "./usage.ts";

export function registerSearchCommand(program: VipvotCommand): void {
  const search = Command({
    use: "search",
    short: "Search Notion by title (pages and databases)",
  });
  registerQuery(search);
  registerUsage(search);
  program.addCommand(search);
}
