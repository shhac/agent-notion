import { Command } from "commander";
import {
  clearAll,
  getDefaultWorkspace,
  getWorkspaces,
  removeWorkspace,
} from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerLogout(parent: Command): void {
  parent
    .command("logout")
    .description("Remove credentials for a workspace")
    .option("--all", "Remove all workspaces and credentials")
    .option("--workspace <alias>", "Remove specific workspace")
    .action((opts: { all?: boolean; workspace?: string }) => {
      try {
        if (opts.all) {
          clearAll();
          printJson({ ok: true, cleared: "all" });
          return;
        }

        const target = opts.workspace ?? getDefaultWorkspace();
        if (!target) {
          printError(
            "No workspaces configured. Nothing to log out from.",
          );
          return;
        }

        const wasDefault =
          target === getDefaultWorkspace();

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
        printError(
          err instanceof Error ? err.message : "Logout failed",
        );
      }
    });
}
