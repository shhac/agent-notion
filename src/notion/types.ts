/**
 * Shared normalized types for the Notion dual-backend system.
 * Both official and v3 backends transform into these types.
 * CLI commands work exclusively with these types.
 */

// --- Pagination ---

export type Paginated<T> = {
  items: T[];
  hasMore: boolean;
  nextCursor?: string;
};

// --- Search ---

export type SearchResult = {
  id: string;
  type: "page" | "database";
  title: string;
  url: string;
  parent?: ParentRef;
  lastEditedAt?: string;
};

// --- Parent Reference ---

export type ParentRef = {
  type: "database" | "page" | "workspace" | "block";
  id?: string;
};

// --- Database ---

export type DatabaseListItem = {
  id: string;
  title: string;
  url: string;
  parent?: ParentRef;
  propertyCount: number;
  lastEditedAt?: string;
};

export type DatabaseDetail = {
  id: string;
  title: string;
  description?: string;
  url: string;
  parent?: ParentRef;
  properties: Record<string, PropertyDefinition>;
  isInline?: boolean;
  createdAt?: string;
  lastEditedAt?: string;
};

export type PropertyDefinition = {
  id: string;
  type: string;
  options?: Array<{ name: string; color?: string }>;
  groups?: Array<{ name: string; options: string[] }>;
  prefix?: string;
  relatedDatabase?: string;
};

export type SchemaProperty = {
  name: string;
  id: string;
  type: string;
  options?: string[];
  groups?: Record<string, string[]>;
  prefix?: string;
  relatedDatabase?: string;
};

export type DatabaseSchema = {
  id: string;
  title: string;
  properties: SchemaProperty[];
};

export type QueryRow = {
  id: string;
  url: string;
  properties: Record<string, unknown>;
  createdAt?: string;
  lastEditedAt?: string;
};

// --- Page ---

export type PageDetail = {
  id: string;
  url: string;
  parent?: ParentRef;
  properties: Record<string, unknown>;
  icon?: { type: string; emoji?: string; url?: string } | null;
  createdAt?: string;
  createdBy?: { id: string; name?: string } | null;
  lastEditedAt?: string;
  lastEditedBy?: { id: string; name?: string } | null;
  archived?: boolean;
};

export type PageCreateResult = {
  id: string;
  url: string;
  title: string;
  parent: Record<string, unknown>;
  createdAt?: string;
};

export type PageUpdateResult = {
  id: string;
  url: string;
  lastEditedAt?: string;
};

export type PageArchiveResult = {
  id: string;
  archived: true;
};

// --- Block ---

export type NormalizedBlock = {
  id: string;
  type: string;
  richText: string;
  hasChildren: boolean;
  // Type-specific fields
  checked?: boolean;
  language?: string;
  url?: string;
  caption?: string;
  emoji?: string;
  title?: string;
  expression?: string;
};

export type BlockListResult = {
  blocks: NormalizedBlock[];
  hasMore: boolean;
};

// --- Comment ---

export type CommentItem = {
  id: string;
  body: string;
  author?: { id: string; name?: string } | null;
  createdAt?: string;
};

export type CommentCreateResult = {
  id: string;
  body: string;
  createdAt?: string;
};

// --- User ---

export type UserItem = {
  id: string;
  name?: string;
  type: "person" | "bot";
  email?: string;
  avatarUrl?: string;
};

export type UserMe = {
  id: string;
  name?: string;
  type: "person" | "bot";
  workspaceName?: string;
};
