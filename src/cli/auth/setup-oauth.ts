import { Command } from "commander";
import { storeOAuthConfig } from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerSetupOAuth(parent: Command): void {
  parent
    .command("setup-oauth")
    .description("Configure OAuth app credentials")
    .requiredOption("--client-id <id>", "OAuth app client ID")
    .requiredOption("--client-secret <secret>", "OAuth app client secret")
    .action((opts: { clientId: string; clientSecret: string }) => {
      try {
        const clientId = opts.clientId.trim();
        if (!clientId) {
          printError("Client ID cannot be empty.");
          return;
        }

        const clientSecret = opts.clientSecret.trim();
        if (!clientSecret) {
          printError("Client secret cannot be empty.");
          return;
        }

        const { storage } = storeOAuthConfig(clientId, clientSecret);

        const result: Record<string, unknown> = {
          ok: true,
          oauth_configured: true,
          client_id: clientId,
          secret_storage: storage,
        };

        if (storage === "config") {
          result.warning =
            "Client secret stored in plaintext config (keychain unavailable on this platform)";
        }

        printJson(result);
      } catch (err) {
        printError(
          err instanceof Error ? err.message : "Failed to configure OAuth",
        );
      }
    });
}
