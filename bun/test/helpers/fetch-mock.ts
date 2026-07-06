/**
 * Shared global-fetch mock for tests.
 *
 * Callers must register `afterEach(restoreFetch)` in their own file — the
 * hook is not installed here because bun caches this module across test files.
 */

export type FetchCall = { url: string; body: unknown };

export type FetchHandler = (url: string, init?: RequestInit) => Response | Promise<Response>;

const originalFetch = globalThis.fetch;

function parseJsonBody(body: unknown): unknown {
  if (typeof body !== "string") return undefined;
  try {
    return JSON.parse(body);
  } catch {
    return body;
  }
}

/**
 * Replace globalThis.fetch. Pass a response body (+ optional status) for a
 * canned JSON response, or a handler function for anything custom. Returns
 * the list of captured calls, appended to as the code under test fetches.
 */
export function mockFetch(response: unknown, status = 200): FetchCall[] {
  const calls: FetchCall[] = [];
  const handler = async (url: string | URL | Request, init?: RequestInit): Promise<Response> => {
    calls.push({ url: String(url), body: parseJsonBody(init?.body) });
    if (typeof response === "function") {
      return (response as FetchHandler)(String(url), init);
    }
    return new Response(JSON.stringify(response), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  };
  globalThis.fetch = Object.assign(handler, {
    preconnect: originalFetch.preconnect,
  }) as typeof fetch;
  return calls;
}

export function restoreFetch(): void {
  globalThis.fetch = originalFetch;
}
