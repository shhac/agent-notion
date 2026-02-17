/**
 * NotionBackend interface — all operations the CLI needs.
 * Both official and v3 backends implement this.
 */
import type {
  Paginated,
  SearchResult,
  DatabaseListItem,
  DatabaseDetail,
  DatabaseSchema,
  QueryRow,
  PageDetail,
  PageCreateResult,
  PageUpdateResult,
  PageArchiveResult,
  NormalizedBlock,
  BlockListResult,
  CommentItem,
  CommentCreateResult,
  UserItem,
  UserMe,
} from "./types.ts";

export interface NotionBackend {
  /** Which backend this is */
  readonly kind: "official" | "v3";

  // --- Search ---
  search(params: {
    query: string;
    filter?: "page" | "database";
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<SearchResult>>;

  // --- Databases ---
  listDatabases(params?: {
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<DatabaseListItem>>;

  getDatabase(id: string): Promise<DatabaseDetail>;

  queryDatabase(params: {
    id: string;
    filter?: unknown;
    sort?: unknown;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<QueryRow>>;

  getDatabaseSchema(id: string): Promise<DatabaseSchema>;

  // --- Pages ---
  getPage(id: string): Promise<PageDetail>;

  createPage(params: {
    parentId: string;
    title: string;
    properties?: Record<string, unknown>;
    icon?: string;
  }): Promise<PageCreateResult>;

  updatePage(params: {
    id: string;
    title?: string;
    properties?: Record<string, unknown>;
    icon?: string;
  }): Promise<PageUpdateResult>;

  archivePage(id: string): Promise<PageArchiveResult>;

  // --- Blocks ---

  /** Paginated block list (for --raw mode) */
  listBlocks(params: {
    id: string;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<NormalizedBlock>>;

  /** Fetch all blocks up to 1000 (for markdown/content mode) */
  getAllBlocks(id: string): Promise<BlockListResult>;

  /** Fetch children of specific blocks */
  getChildBlocks(blockIds: string[]): Promise<Map<string, NormalizedBlock[]>>;

  appendBlocks(params: {
    id: string;
    blocks: unknown[];
  }): Promise<{ blocksAdded: number }>;

  // --- Comments ---
  listComments(params: {
    pageId: string;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<CommentItem>>;

  addComment(params: {
    pageId: string;
    body: string;
  }): Promise<CommentCreateResult>;

  addInlineComment(params: {
    blockId: string;
    body: string;
    text: string;
    occurrence?: number;
  }): Promise<CommentCreateResult>;

  // --- Users ---
  listUsers(params?: {
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<UserItem>>;

  getMe(): Promise<UserMe>;

  // --- Utility ---

  /** Check if a given ID is a database (used by page create to detect parent type) */
  isDatabase(id: string): Promise<boolean>;
}
