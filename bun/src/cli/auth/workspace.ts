import { Command } from "vipvot";
import { defineCommand, ExactArgs, type Command as VipvotCommand } from "../../lib/cli.ts";
import {
  getDefaultWorkspace,
  getWorkspaces,
  removeWorkspace,
  setDefaultWorkspace,
} from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerWorkspace(parent: VipvotCommand): void {
  const workspace = Command({ use: "workspace", short: "Manage workspace profiles" });

  workspace.addCommand(
    defineCommand({
      use: "list",
      short: "List configured workspaces",
      action: () => {
        const workspaces = getWorkspaces();
        const defaultAlias = getDefaultWorkspace();

        const items = Object.entries(workspaces).map(([alias, ws]) => ({
          alias,
          name: ws.workspace_name,
          auth_type: ws.auth_type,
          default: alias === defaultAlias,
        }));

        printJson({ items });
      },
    }),
  );

  workspace.addCommand(
    defineCommand({
      use: "switch <alias>",
      short: "Switch active workspace",
      args: ExactArgs(1),
      action: ([alias]) => {
        try {
          setDefaultWorkspace(alias!);
          printJson({ ok: true, default_workspace: alias });
        } catch (err) {
          printError(err instanceof Error ? err.message : "Switch failed");
        }
      },
    }),
  );

  workspace.addCommand(
    defineCommand({
      use: "set-default <alias>",
      short: "Set default workspace (alias for switch)",
      args: ExactArgs(1),
      action: ([alias]) => {
        try {
          setDefaultWorkspace(alias!);
          printJson({ ok: true, default_workspace: alias });
        } catch (err) {
          printError(err instanceof Error ? err.message : "Set default failed");
        }
      },
    }),
  );

  workspace.addCommand(
    defineCommand({
      use: "remove <alias>",
      short: "Remove a workspace",
      args: ExactArgs(1),
      action: ([alias]) => {
        try {
          const wasDefault = alias === getDefaultWorkspace();

          removeWorkspace(alias!);

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
          printError(err instanceof Error ? err.message : "Remove failed");
        }
      },
    }),
  );

  parent.addCommand(workspace);
}
