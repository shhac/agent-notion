import { Command } from "commander";
import { registerLogin } from "./login.ts";
import { registerLogout } from "./logout.ts";
import { registerSetupOAuth } from "./setup-oauth.ts";
import { registerStatus } from "./status.ts";
import { registerWorkspace } from "./workspace.ts";

export function registerAuthCommand(program: Command): void {
  const auth = program
    .command("auth")
    .description("Manage authentication and workspaces");

  registerSetupOAuth(auth);
  registerLogin(auth);
  registerLogout(auth);
  registerStatus(auth);
  registerWorkspace(auth);
}
