import { Command, type Command as VipvotCommand } from "vipvot";
import { registerImportDesktop } from "./import-desktop.ts";
import { registerLogin } from "./login.ts";
import { registerLogout } from "./logout.ts";
import { registerSetupOAuth } from "./setup-oauth.ts";
import { registerStatus } from "./status.ts";
import { registerUsage } from "./usage.ts";
import { registerWorkspace } from "./workspace.ts";

export function registerAuthCommand(program: VipvotCommand): void {
  const auth = Command({
    use: "auth",
    short: "Manage authentication and workspaces",
  });

  registerSetupOAuth(auth);
  registerLogin(auth);
  registerLogout(auth);
  registerImportDesktop(auth);
  registerStatus(auth);
  registerWorkspace(auth);
  registerUsage(auth);
  program.addCommand(auth);
}
