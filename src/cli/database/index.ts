import { Command, type Command as VipvotCommand } from "vipvot";
import { registerList } from "./list.ts";
import { registerGet } from "./get.ts";
import { registerQuery } from "./query.ts";
import { registerSchema } from "./schema.ts";
import { registerUsage } from "./usage.ts";

export function registerDatabaseCommand(program: VipvotCommand): void {
  const database = Command({ use: "database", short: "Database operations" });
  registerList(database);
  registerGet(database);
  registerQuery(database);
  registerSchema(database);
  registerUsage(database);
  program.addCommand(database);
}
