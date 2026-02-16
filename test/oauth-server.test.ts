import { describe, test, expect } from "bun:test";
import { startOAuthServer } from "../src/lib/oauth-server.ts";

describe("OAuth server", () => {
  test("resolves with code on valid callback", async () => {
    const state = "test-state-123";
    const serverPromise = startOAuthServer(state, 19876, 5000);

    // Give server a moment to start
    await new Promise((r) => setTimeout(r, 100));

    // Simulate callback
    const response = await fetch(
      `http://127.0.0.1:19876/callback?code=auth_code_xyz&state=${state}`,
    );
    expect(response.status).toBe(200);

    const result = await serverPromise;
    expect(result.code).toBe("auth_code_xyz");
    expect(result.port).toBe(19876);
  });

  test("rejects on state mismatch", async () => {
    const state = "correct-state";
    const serverPromise = startOAuthServer(state, 19877, 5000);
    // Attach catch immediately to prevent unhandled rejection
    const caughtPromise = serverPromise.catch((err) => err as Error);

    await new Promise((r) => setTimeout(r, 100));

    // Fire the callback — don't await the response since server closes immediately
    fetch(
      `http://127.0.0.1:19877/callback?code=auth_code&state=wrong-state`,
    ).catch(() => {});

    const err = await caughtPromise;
    expect(err).toBeInstanceOf(Error);
    expect((err as Error).message).toContain("state mismatch");
  });

  test("rejects on Notion error parameter", async () => {
    const state = "test-state";
    const serverPromise = startOAuthServer(state, 19878, 5000);
    const caughtPromise = serverPromise.catch((err) => err as Error);

    await new Promise((r) => setTimeout(r, 100));

    fetch(
      `http://127.0.0.1:19878/callback?error=access_denied`,
    ).catch(() => {});

    const err = await caughtPromise;
    expect(err).toBeInstanceOf(Error);
    expect((err as Error).message).toContain("access_denied");
  });

  test("returns 404 for non-callback paths", async () => {
    const state = "test-state";
    const serverPromise = startOAuthServer(state, 19879, 2000);

    await new Promise((r) => setTimeout(r, 100));

    const response = await fetch("http://127.0.0.1:19879/other");
    expect(response.status).toBe(404);

    // Clean up — send valid callback to resolve the promise
    await fetch(
      `http://127.0.0.1:19879/callback?code=cleanup&state=${state}`,
    );
    await serverPromise;
  });

  test("times out when no callback received", async () => {
    const state = "test-state";
    const serverPromise = startOAuthServer(state, 19880, 500); // 500ms timeout
    const caughtPromise = serverPromise.catch((err) => err as Error);

    const err = await caughtPromise;
    expect(err).toBeInstanceOf(Error);
    expect((err as Error).message).toContain("timed out");
  });
});
