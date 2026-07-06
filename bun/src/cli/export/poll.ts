/**
 * Export polling and download helpers for enqueueTask + getTasks flow.
 */
import { resolve } from "node:path";
import type { V3HttpClient, V3ExportTask } from "../../notion/v3/client.ts";
import { V3HttpError } from "../../notion/v3/client.ts";

const DEFAULT_POLL_INTERVAL = 2_000;
const DEFAULT_TIMEOUT = 120_000;
const MAX_CONSECUTIVE_ERRORS = 3;
const MAX_BACKOFF = 30_000;
const DOWNLOAD_TIMEOUT = 10 * 60_000; // 10 minutes

export type ExportFormat = "markdown" | "html";

export function validateFormat(format: string): ExportFormat {
  if (format === "markdown" || format === "html") return format;
  throw new Error(
    `Invalid format "${format}". Use "markdown" or "html".`,
  );
}

export type PollOptions = {
  pollInterval?: number;
  timeout?: number;
};

export function defaultExportFilename(): string {
  const ts = new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19);
  return `notion-export-${ts}.zip`;
}

/**
 * Enqueue an export task and poll until completion, then download the zip.
 * Writes progress to stderr so stdout stays clean for JSON output.
 */
export async function exportAndDownload(
  client: V3HttpClient,
  task: { eventName: string; request: Record<string, unknown> },
  outputPath: string,
  opts?: PollOptions,
): Promise<{ path: string; pagesExported: number }> {
  // 1. Enqueue the export task
  const { taskId } = await client.enqueueTask(task);
  process.stderr.write(`Export task queued: ${taskId}\n`);

  // 2. Poll until done
  const result = await pollTask(client, taskId, opts);

  if (result.state === "failure") {
    throw new Error(
      `Export failed${result.error ? `: ${result.error}` : ". Check the page ID and try again."}`,
    );
  }

  const exportURL = result.status?.exportURL;
  if (!exportURL) {
    throw new Error("Export succeeded but no download URL was provided.");
  }

  // 3. Download the zip (streamed to disk via Bun.write)
  const resolvedPath = resolve(outputPath);
  process.stderr.write(`Downloading export...\n`);

  const downloadController = new AbortController();
  const downloadTimer = setTimeout(() => downloadController.abort(), DOWNLOAD_TIMEOUT);

  try {
    const response = await fetch(exportURL, { signal: downloadController.signal });
    if (!response.ok) {
      throw new Error(`Download failed: ${response.status} ${response.statusText}`);
    }
    await Bun.write(resolvedPath, response);
  } finally {
    clearTimeout(downloadTimer);
  }

  return {
    path: resolvedPath,
    pagesExported: result.status?.pagesExported ?? 0,
  };
}

async function pollTask(
  client: V3HttpClient,
  taskId: string,
  opts?: PollOptions,
): Promise<V3ExportTask> {
  const interval = opts?.pollInterval ?? DEFAULT_POLL_INTERVAL;
  const timeout = opts?.timeout ?? DEFAULT_TIMEOUT;
  const deadline = Date.now() + timeout;

  let lastPages = 0;
  let consecutiveErrors = 0;

  while (Date.now() < deadline) {
    let task: V3ExportTask | undefined;
    try {
      const { results } = await client.getTasks([taskId]);
      task = results?.[0];
      consecutiveErrors = 0;
    } catch (err) {
      if (isTransientError(err) && consecutiveErrors < MAX_CONSECUTIVE_ERRORS) {
        consecutiveErrors++;
        const backoff = Math.min(interval * 2 ** consecutiveErrors, MAX_BACKOFF);
        process.stderr.write(
          `\nPoll error (attempt ${consecutiveErrors}/${MAX_CONSECUTIVE_ERRORS}), retrying in ${backoff / 1000}s...\n`,
        );
        await sleep(backoff);
        continue;
      }
      throw err;
    }

    if (!task) {
      throw new Error(`Task ${taskId} not found in getTasks response.`);
    }

    if (task.state === "success" || task.state === "failure") {
      process.stderr.write("\n");
      return task;
    }

    const pages = task.status?.pagesExported ?? 0;
    if (pages > lastPages) {
      process.stderr.write(`\rExporting... ${pages} pages exported`);
      lastPages = pages;
    }

    await sleep(interval);
  }

  process.stderr.write(`\nTask ID for manual follow-up: ${taskId}\n`);
  throw new Error(
    `Export timed out after ${timeout / 1000}s (task: ${taskId}). The export may still be running on Notion's servers.`,
  );
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}

function isTransientError(err: unknown): boolean {
  if (err instanceof V3HttpError && err.status >= 500) return true;
  if (err instanceof Error && (err.name === "AbortError" || err.name === "TypeError")) return true;
  return false;
}
