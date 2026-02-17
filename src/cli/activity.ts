import type { Command } from "commander";
import { createV3Client } from "../notion/client.ts";
import { handleAction } from "../lib/errors.ts";
import { normalizeId } from "../lib/ids.ts";
import { printJson } from "../lib/output.ts";
import { getBlock, getUser, v3RichTextToPlain } from "../notion/v3/transforms.ts";

export function registerActivityCommand(program: Command): void {
  program
    .command("activity")
    .description("Show recent activity log for workspace or a page (v3 desktop session required)")
    .option("--page <page-id>", "Scope to a specific page")
    .option("--limit <n>", "Number of activity entries", "20")
    .action(async (opts: { page?: string; limit: string }) => {
      await handleAction(async () => {
        const client = createV3Client();
        const limit = parseInt(opts.limit, 10);
        const navigableBlockId = opts.page ? normalizeId(opts.page) : undefined;

        const result = await client.getActivityLog({
          navigableBlockId,
          limit,
        });

        const entries = (result.activityIds ?? []).map((actId) => {
          const activity = result.activities[actId];
          if (!activity) return { id: actId };

          // Resolve page title from recordMap
          const blockId = activity.navigable_block_id ?? activity.parent_id;
          const block = blockId ? getBlock(result.recordMap, blockId) : undefined;
          const pageTitle = block?.properties?.title
            ? v3RichTextToPlain(block.properties.title)
            : undefined;

          // Resolve author names
          const authors = (activity.edits ?? [])
            .flatMap((e) => e.authors ?? [])
            .map((a) => {
              const user = getUser(result.recordMap, a.id);
              return user
                ? [user.given_name, user.family_name].filter(Boolean).join(" ")
                : a.id;
            });
          const uniqueAuthors = [...new Set(authors)];

          return {
            id: actId,
            type: activity.type,
            pageId: blockId,
            pageTitle,
            authors: uniqueAuthors.length > 0 ? uniqueAuthors : undefined,
            editTypes: activity.edits?.map((e) => e.type),
            startTime: activity.start_time ? new Date(activity.start_time).toISOString() : undefined,
            endTime: activity.end_time ? new Date(activity.end_time).toISOString() : undefined,
          };
        });

        printJson({ activities: entries, total: entries.length });
      });
    });
}
