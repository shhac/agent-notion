import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerGet } from "./get.ts";
import { registerQuery } from "./query.ts";
import { registerSchema } from "./schema.ts";
import { registerUsage } from "./usage.ts";

export function registerDatabaseCommand(program: Command): void {
  const database = program.command("database").description("Database operations");
  registerList(database);
  registerGet(database);
  registerQuery(database);
  registerSchema(database);
  registerUsage(database);
}
