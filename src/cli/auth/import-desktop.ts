import { Command } from "commander";
import { storeV3Session } from "../../lib/config.ts";
import {
  DesktopTokenError,
  extractDesktopToken,
  validateDesktopToken,
} from "../../lib/desktop-token.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerImportDesktop(parent: Command): void {
  parent
    .command("import-desktop")
    .description(
      "Import session from the Notion Desktop app (macOS only). " +
        "Extracts token_v2 for use with Notion's internal API.",
    )
    .option("--skip-validation", "Skip token validation against Notion API")
    .action(async (opts: { skipValidation?: boolean }) => {
      try {
        // 1. Extract token from desktop app
        const { token_v2, extracted_at } = extractDesktopToken();

        // 2. Validate and get session info
        let sessionInfo: Awaited<ReturnType<typeof validateDesktopToken>>;
        if (opts.skipValidation) {
          sessionInfo = {
            user_id: "",
            user_email: "",
            user_name: "",
            space_id: "",
            space_name: "",
          };
        } else {
          sessionInfo = await validateDesktopToken(token_v2);
        }

        // 3. Store in config/keychain
        const { storage } = storeV3Session({
          token_v2,
          extracted_at,
          ...sessionInfo,
        });

        printJson({
          ok: true,
          session: {
            user: sessionInfo.user_name || undefined,
            email: sessionInfo.user_email || undefined,
            space: sessionInfo.space_name || undefined,
            space_id: sessionInfo.space_id || undefined,
            storage,
            extracted_at,
          },
        });
      } catch (err) {
        if (err instanceof DesktopTokenError) {
          printError(`${err.message} [${err.code}]`);
        } else {
          printError(
            err instanceof Error
              ? err.message
              : "Failed to import desktop session",
          );
        }
      }
    });
}
