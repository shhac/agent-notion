import { printError } from "./output.ts";
import { V3HttpError } from "../notion/v3/client.ts";

/**
 * Structured CLI error with guidance message.
 * Every error includes what went wrong + how to fix it.
 */
export class CliError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "CliError";
  }
}

/**
 * Run a CLI action, catching CliError and Notion API errors
 * and formatting them as JSON error output.
 */
export async function handleAction(fn: () => Promise<void>): Promise<void> {
  try {
    await fn();
  } catch (err) {
    if (err instanceof CliError) {
      printError(err.message);
      return;
    }

    // v3 backend throws V3HttpError with status + endpoint
    if (err instanceof V3HttpError) {
      printError(formatV3Error(err));
      return;
    }

    // @notionhq/client throws APIResponseError with status + code
    if (isNotionApiError(err)) {
      printError(formatNotionError(err));
      return;
    }

    const message = err instanceof Error ? err.message : String(err);
    printError(message);
  }
}

interface NotionApiError {
  status: number;
  code: string;
  message: string;
}

function isNotionApiError(err: unknown): err is NotionApiError {
  return (
    typeof err === "object" &&
    err !== null &&
    "status" in err &&
    "code" in err &&
    typeof (err as NotionApiError).message === "string"
  );
}

function formatV3Error(err: V3HttpError): string {
  switch (err.status) {
    case 401:
      return "Desktop token expired. Run 'agent-notion auth import-desktop' to re-import.";
    case 403:
      return "Access denied. The token may not have access to this resource, or it may have expired.";
    case 404:
      return "Not found. Check the ID, or ensure the page is accessible with your desktop token.";
    case 429:
      return "Rate limited by Notion. Wait a moment and retry.";
    default:
      return `v3 API error (${err.status} on ${err.endpoint}): ${err.message}`;
  }
}

function formatNotionError(err: NotionApiError): string {
  switch (err.code) {
    case "unauthorized":
      return "Not authenticated. Set NOTION_API_KEY env var or run 'agent-notion config set notion.apiKey <key>'.";
    case "object_not_found":
      return "Not found. The integration may not have access. Share the resource with your integration in Notion.";
    case "validation_error":
      return `Notion API validation error: ${err.message}`;
    case "rate_limited":
      return "Rate limited by Notion API. Wait a moment and retry.";
    default:
      return `Notion API error (${err.code}): ${err.message}`;
  }
}
