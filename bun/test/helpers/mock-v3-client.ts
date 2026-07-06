/**
 * Mock V3HttpClient — the shared "mock Notion" for endpoint-level tests.
 *
 * Each override returns the raw API response for one endpoint; unset
 * endpoints return empty defaults. Every call is tracked in `calls` for
 * request assertions.
 *
 * The real client normalizes responses in post() — fixtures that use the
 * role-wrapped wire format should be passed through normalizeRecordMapResponse
 * so the mock exercises the same seam as production.
 */
import type { V3Operation } from "../../src/notion/v3/operations.ts";

export type MockResponses = {
  search?: () => any;
  loadPageChunk?: (params: any) => any;
  syncRecordValues?: (requests: any) => any;
  syncRecordValuesForPointers?: (pointers: any) => any;
  queryCollection?: (params: any) => any;
  saveTransactions?: (ops: V3Operation[]) => void;
  restoreRecord?: (params: any) => any;
  loadUserContent?: () => any;
  enqueueTask?: (task: any) => any;
  getTasks?: (taskIds: string[]) => any;
};

export function createMockClient(responses: MockResponses = {}) {
  const calls: Record<string, any[]> = {};

  function track(method: string, args: any) {
    if (!calls[method]) calls[method] = [];
    calls[method].push(args);
  }

  const client = {
    get spaceId_() { return "space-1"; },
    get userId_() { return "user-1"; },
    search: async (params: any) => {
      track("search", params);
      return responses.search?.() ?? { results: [], total: 0, recordMap: {} };
    },
    loadPageChunk: async (params: any) => {
      track("loadPageChunk", params);
      return responses.loadPageChunk?.(params) ?? { recordMap: {}, cursor: { stack: [] } };
    },
    syncRecordValues: async (requests: any) => {
      track("syncRecordValues", requests);
      return responses.syncRecordValues?.(requests) ?? { recordMap: {} };
    },
    syncRecordValuesForPointers: async (pointers: any) => {
      track("syncRecordValuesForPointers", pointers);
      return responses.syncRecordValuesForPointers?.(pointers) ?? { recordMap: {} };
    },
    queryCollection: async (params: any) => {
      track("queryCollection", params);
      return responses.queryCollection?.(params) ?? { result: { blockIds: [], total: 0 }, recordMap: {} };
    },
    saveTransactions: async (ops: V3Operation[]) => {
      track("saveTransactions", ops);
      responses.saveTransactions?.(ops);
    },
    restoreRecord: async (params: any) => {
      track("restoreRecord", params);
      return responses.restoreRecord?.(params) ?? { recordMap: {} };
    },
    loadUserContent: async () => {
      track("loadUserContent", {});
      return responses.loadUserContent?.() ?? { recordMap: {} };
    },
    enqueueTask: async (task: any) => {
      track("enqueueTask", task);
      return responses.enqueueTask?.(task) ?? { taskId: "task-1" };
    },
    getTasks: async (taskIds: string[]) => {
      track("getTasks", taskIds);
      return responses.getTasks?.(taskIds) ?? { results: [] };
    },
  };

  return { client: client as any, calls };
}
