import type { Command } from "commander";

const USAGE_TEXT = `agent-notion user — Workspace user information

SUBCOMMANDS:
  user list [--limit] [--cursor]    List all users in the workspace
  user me                           Get the bot user (integration) identity

LIST OUTPUT:
  { "items": [{ id, name, type, email?, avatarUrl? }], "pagination"?: ... }

  type: "person" (human user) or "bot" (integration)
  email: Only available for person users (not bots)

ME OUTPUT:
  { id, name, type, workspaceName }
`;

export function registerUsage(user: Command): void {
  user
    .command("usage")
    .description("Print detailed user documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
