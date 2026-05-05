import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { createV3Client } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerHistory(page: Command): void {
  page.addCommand(
    defineCommand({
      use: "history <page-id>",
      short:
        "List version history (snapshots) of a page (v3 desktop session required)",
      args: ExactArgs(1),
      options: {
        limit: { type: "string", default: "20", description: "Number of snapshots to fetch" },
      },
      action: async ([rawPageId], opts) => {
        await handleAction(async () => {
          const pageId = normalizeId(rawPageId!);
          const client = createV3Client();
          const limit = parseInt(opts.limit, 10);

          const result = await client.getSnapshotsList({
            blockId: pageId,
            size: limit,
          });

          const snapshots = (result.snapshots ?? []).map((snap) => ({
            id: snap.id,
            version: snap.version,
            lastVersion: snap.last_version,
            timestamp: new Date(snap.timestamp).toISOString(),
            authors: snap.authors?.map((a) => a.id) ?? [],
          }));

          printJson({ snapshots, total: snapshots.length });
        });
      },
    }),
  );
}
