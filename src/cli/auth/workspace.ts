import { Command } from "commander";
import {
  getDefaultWorkspace,
  getWorkspaces,
  removeWorkspace,
  setDefaultWorkspace,
} from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerWorkspace(parent: Command): void {
  const workspace = parent
    .command("workspace")
    .description("Manage workspace profiles");

  // list
  workspace
    .command("list")
    .description("List configured workspaces")
    .action(() => {
      const workspaces = getWorkspaces();
      const defaultAlias = getDefaultWorkspace();

      const items = Object.entries(workspaces).map(
        ([alias, ws]) => ({
          alias,
          name: ws.workspace_name,
          auth_type: ws.auth_type,
          default: alias === defaultAlias,
        }),
      );

      printJson({ items });
    });

  // switch
  workspace
    .command("switch <alias>")
    .description("Switch active workspace")
    .action((alias: string) => {
      try {
        setDefaultWorkspace(alias);
        printJson({ ok: true, default_workspace: alias });
      } catch (err) {
        printError(
          err instanceof Error ? err.message : "Switch failed",
        );
      }
    });

  // set-default (alias for switch)
  workspace
    .command("set-default <alias>")
    .description("Set default workspace (alias for switch)")
    .action((alias: string) => {
      try {
        setDefaultWorkspace(alias);
        printJson({ ok: true, default_workspace: alias });
      } catch (err) {
        printError(
          err instanceof Error
            ? err.message
            : "Set default failed",
        );
      }
    });

  // remove
  workspace
    .command("remove <alias>")
    .description("Remove a workspace")
    .action((alias: string) => {
      try {
        const wasDefault = alias === getDefaultWorkspace();

        removeWorkspace(alias);

        const newDefault = getDefaultWorkspace();
        const result: Record<string, unknown> = {
          ok: true,
          removed: alias,
          default_workspace: newDefault ?? null,
        };
        if (wasDefault) {
          result.warning = "Removed the default workspace";
        }

        printJson(result);
      } catch (err) {
        printError(
          err instanceof Error ? err.message : "Remove failed",
        );
      }
    });
}
