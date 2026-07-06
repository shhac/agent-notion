import { defineCommand, type Command } from "../../lib/cli.ts";
import {
  clearAll,
  getDefaultWorkspace,
  getWorkspaces,
  removeWorkspace,
} from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerLogout(parent: Command): void {
  parent.addCommand(
    defineCommand({
      use: "logout",
      short: "Remove credentials for a workspace",
      options: {
        all: { type: "bool", description: "Remove all workspaces and credentials" },
        workspace: { type: "string", description: "Remove specific workspace" },
      },
      action: (_args, opts) => {
        try {
          if (opts.all) {
            clearAll();
            printJson({ ok: true, cleared: "all" });
            return;
          }

          const target = opts.workspace ?? getDefaultWorkspace();
          if (!target) {
            printError("No workspaces configured. Nothing to log out from.");
            return;
          }

          const wasDefault = target === getDefaultWorkspace();

          removeWorkspace(target);

          const remaining = Object.keys(getWorkspaces());
          const newDefault = getDefaultWorkspace();

          const result: Record<string, unknown> = {
            ok: true,
            removed: target,
            remaining_workspaces: remaining,
            default_workspace: newDefault ?? null,
          };
          if (wasDefault) {
            result.warning = "Removed the default workspace";
          }

          printJson(result);
        } catch (err) {
          printError(err instanceof Error ? err.message : "Logout failed");
        }
      },
    }),
  );
}
