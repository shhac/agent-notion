import { createServer, type Server } from "node:http";
import { URL } from "node:url";

export type OAuthCallbackResult = {
  code: string;
  port: number;
};

const SUCCESS_HTML = `<!DOCTYPE html>
<html><head><title>agent-notion</title></head>
<body style="font-family:system-ui;text-align:center;padding:60px">
<h2>Authorized</h2>
<p>You can close this tab and return to the terminal.</p>
</body></html>`;

const escapeHtml = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

const ERROR_HTML = (msg: string) => `<!DOCTYPE html>
<html><head><title>agent-notion</title></head>
<body style="font-family:system-ui;text-align:center;padding:60px">
<h2>Error</h2>
<p>${escapeHtml(msg)}</p>
</body></html>`;

/**
 * Start a localhost HTTP server that waits for the OAuth callback.
 * Tries ports from `startPort` to `startPort + 9`.
 * Returns a promise that resolves with the auth code.
 */
export function startOAuthServer(
  expectedState: string,
  startPort: number = 9876,
  timeoutMs: number = 120_000,
): Promise<OAuthCallbackResult> {
  return new Promise((resolve, reject) => {
    let server: Server;
    let timer: ReturnType<typeof setTimeout>;
    let settled = false;

    const cleanup = () => {
      if (timer) clearTimeout(timer);
      try {
        server?.close();
      } catch {
        /* ignore */
      }
    };

    server = createServer((req, res) => {
      if (settled) return;

      const url = new URL(req.url ?? "/", `http://localhost`);
      if (url.pathname !== "/callback") {
        res.writeHead(404);
        res.end("Not found");
        return;
      }

      const code = url.searchParams.get("code");
      const state = url.searchParams.get("state");
      const error = url.searchParams.get("error");

      if (error) {
        settled = true;
        res.writeHead(400, { "Content-Type": "text/html" });
        res.end(ERROR_HTML(`Notion returned an error: ${error}`));
        cleanup();
        reject(
          new Error(
            `Notion OAuth error: ${error}`,
          ),
        );
        return;
      }

      if (state !== expectedState) {
        settled = true;
        res.writeHead(400, { "Content-Type": "text/html" });
        res.end(
          ERROR_HTML(
            "OAuth state mismatch &mdash; possible CSRF attack. Please try again.",
          ),
        );
        cleanup();
        reject(
          new Error(
            "OAuth state mismatch \u2014 possible CSRF attack. Please try again.",
          ),
        );
        return;
      }

      if (!code) {
        settled = true;
        res.writeHead(400, { "Content-Type": "text/html" });
        res.end(ERROR_HTML("No authorization code received."));
        cleanup();
        reject(new Error("No authorization code received from Notion."));
        return;
      }

      settled = true;
      res.writeHead(200, { "Content-Type": "text/html" });
      res.end(SUCCESS_HTML);

      const port = (server.address() as { port: number }).port;
      cleanup();
      resolve({ code, port });
    });

    // Try ports in range
    const tryListen = (port: number, maxPort: number) => {
      server.once("error", (err: NodeJS.ErrnoException) => {
        if (err.code === "EADDRINUSE" && port < maxPort) {
          tryListen(port + 1, maxPort);
        } else {
          reject(
            new Error(
              `Could not find an available port for OAuth callback (tried ${startPort}-${maxPort}). Use --port to specify a port.`,
            ),
          );
        }
      });
      server.listen(port, "127.0.0.1");
    };

    server.once("listening", () => {
      timer = setTimeout(() => {
        if (!settled) {
          settled = true;
          cleanup();
          reject(
            new Error(
              "OAuth flow timed out after 120 seconds. Please try again.",
            ),
          );
        }
      }, timeoutMs);
    });

    tryListen(startPort, startPort + 9);
  });
}
