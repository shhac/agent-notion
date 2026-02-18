import type { Command } from "commander";
import { createV3Client } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerList(history: Command): void {
  history
    .argument("<page-id>", "Page UUID or dashless ID")
    .option("--limit <n>", "Number of snapshots to fetch", "20")
    .action(async (rawPageId: string, opts: { limit: string }) => {
      await handleAction(async () => {
        const pageId = normalizeId(rawPageId);
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
    });
}
