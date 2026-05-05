import { defineCommand, type Command } from "../../lib/cli.ts";
import { storeOAuthConfig } from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerSetupOAuth(parent: Command): void {
  parent.addCommand(
    defineCommand({
      use: "setup-oauth",
      short: "Configure OAuth app credentials",
      options: {
        clientId: { type: "string", required: true, description: "OAuth app client ID" },
        clientSecret: {
          type: "string",
          required: true,
          description: "OAuth app client secret",
        },
      },
      action: (_args, opts) => {
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
          printError(err instanceof Error ? err.message : "Failed to configure OAuth");
        }
      },
    }),
  );
}
