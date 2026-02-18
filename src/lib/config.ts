import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import {
  KEYCHAIN_PLACEHOLDER,
  KEYCHAIN_SERVICE,
  keychainDelete,
  keychainDeleteAll,
  keychainGet,
  keychainSet,
} from "./keychain.ts";

// --- Types ---

export type AuthType = "oauth" | "internal_integration" | "desktop";

export type Workspace = {
  workspace_id: string;
  workspace_name: string;
  workspace_icon?: string;
  bot_id: string;
  auth_type: AuthType;
  access_token: string;
  refresh_token?: string;
  owner?: {
    type: "user";
    user: { id: string; name?: string; email?: string };
  };
};

export type OAuthConfig = {
  client_id: string;
  client_secret: string;
  redirect_uri: string;
};

export type Settings = {
  page_size?: number;
  max_depth?: number;
  truncation?: {
    max_length?: number;
  };
  ai?: {
    default_model?: string;
  };
};

export type V3Session = {
  token_v2: string;
  user_id: string;
  user_email: string;
  user_name: string;
  space_id: string;
  space_name: string;
  extracted_at: string;
};

export type Config = {
  oauth?: OAuthConfig;
  default_workspace?: string;
  workspaces?: Record<string, Workspace>;
  settings?: Settings;
  v3?: V3Session;
};

// --- Config directory ---

function configDir(): string {
  const xdg = process.env["XDG_CONFIG_HOME"];
  if (xdg) {
    return join(xdg, "agent-notion");
  }
  return join(homedir(), ".config", "agent-notion");
}

function ensureConfigDir(): void {
  const dir = configDir();
  if (!existsSync(dir)) {
    mkdirSync(dir, { recursive: true });
  }
}

function configPath(): string {
  return join(configDir(), "config.json");
}

// --- Config read/write ---

export function readConfig(): Config {
  const path = configPath();
  if (!existsSync(path)) {
    return {};
  }
  try {
    return JSON.parse(readFileSync(path, "utf8")) as Config;
  } catch {
    return {};
  }
}

export function writeConfig(config: Config): void {
  ensureConfigDir();
  // Clean up empty top-level objects
  if (config.workspaces && Object.keys(config.workspaces).length === 0) {
    delete config.workspaces;
  }
  writeFileSync(configPath(), JSON.stringify(config, null, 2) + "\n", {
    encoding: "utf8",
    mode: 0o600,
  });
}

// --- OAuth config ---

export function getOAuthConfig(): OAuthConfig | undefined {
  const config = readConfig();
  return config.oauth;
}

export function storeOAuthConfig(clientId: string, clientSecret: string): {
  storage: "keychain" | "config";
} {
  const config = readConfig();
  const stored = keychainSet({
    account: "oauth_client_secret",
    value: clientSecret,
    service: KEYCHAIN_SERVICE,
  });

  config.oauth = {
    client_id: clientId,
    client_secret: stored ? KEYCHAIN_PLACEHOLDER : clientSecret,
    redirect_uri: "http://localhost:9876/callback",
  };
  writeConfig(config);

  return { storage: stored ? "keychain" : "config" };
}

export function resolveOAuthClientSecret(): string | undefined {
  const oauth = getOAuthConfig();
  if (!oauth) return undefined;

  if (oauth.client_secret === KEYCHAIN_PLACEHOLDER) {
    return keychainGet("oauth_client_secret", KEYCHAIN_SERVICE) ?? undefined;
  }
  return oauth.client_secret;
}

// --- Workspace CRUD ---

export function getWorkspaces(): Record<string, Workspace> {
  const config = readConfig();
  return config.workspaces ?? {};
}

export function getWorkspace(alias: string): Workspace | undefined {
  return getWorkspaces()[alias];
}

export function getDefaultWorkspace(): string | undefined {
  const config = readConfig();
  return config.default_workspace;
}

export function storeWorkspace(
  alias: string,
  workspace: Omit<Workspace, "access_token" | "refresh_token"> & {
    access_token: string;
    refresh_token?: string;
  },
): { storage: "keychain" | "config" } {
  const config = readConfig();
  if (!config.workspaces) {
    config.workspaces = {};
  }

  // Attempt keychain storage for access_token
  const accessStored = keychainSet({
    account: `access_token:${alias}`,
    value: workspace.access_token,
    service: KEYCHAIN_SERVICE,
  });

  // Attempt keychain storage for refresh_token (OAuth only)
  let refreshStored = true;
  if (workspace.refresh_token) {
    refreshStored = keychainSet({
      account: `refresh_token:${alias}`,
      value: workspace.refresh_token,
      service: KEYCHAIN_SERVICE,
    });
  }

  const useKeychain = accessStored && refreshStored;

  if (!useKeychain) {
    // Clean up partial keychain entries
    keychainDelete(`access_token:${alias}`, KEYCHAIN_SERVICE);
    if (workspace.refresh_token) {
      keychainDelete(`refresh_token:${alias}`, KEYCHAIN_SERVICE);
    }
  }

  config.workspaces[alias] = {
    workspace_id: workspace.workspace_id,
    workspace_name: workspace.workspace_name,
    workspace_icon: workspace.workspace_icon,
    bot_id: workspace.bot_id,
    auth_type: workspace.auth_type,
    access_token: useKeychain ? KEYCHAIN_PLACEHOLDER : workspace.access_token,
    refresh_token: workspace.refresh_token
      ? useKeychain
        ? KEYCHAIN_PLACEHOLDER
        : workspace.refresh_token
      : undefined,
    owner: workspace.owner,
  };

  // Auto-set default if first workspace
  if (!config.default_workspace) {
    config.default_workspace = alias;
  }

  writeConfig(config);
  return { storage: useKeychain ? "keychain" : "config" };
}

export function removeWorkspace(alias: string): void {
  const config = readConfig();
  const workspaces = config.workspaces ?? {};

  if (!workspaces[alias]) {
    const valid = Object.keys(workspaces);
    throw new Error(
      `Unknown workspace: '${alias}'. Valid workspaces: ${valid.join(", ") || "(none)"}`,
    );
  }

  // Delete keychain entries
  keychainDelete(`access_token:${alias}`, KEYCHAIN_SERVICE);
  keychainDelete(`refresh_token:${alias}`, KEYCHAIN_SERVICE);

  // Remove from config
  delete workspaces[alias];
  config.workspaces = workspaces;

  // Reassign default if needed
  if (config.default_workspace === alias) {
    const remaining = Object.keys(workspaces);
    config.default_workspace = remaining[0];
  }

  writeConfig(config);
}

export function setDefaultWorkspace(alias: string): void {
  const config = readConfig();
  const workspaces = config.workspaces ?? {};

  if (!workspaces[alias]) {
    const valid = Object.keys(workspaces);
    throw new Error(
      `Unknown workspace: '${alias}'. Valid workspaces: ${valid.join(", ") || "(none)"}`,
    );
  }

  config.default_workspace = alias;
  writeConfig(config);
}

export function clearAll(): void {
  keychainDeleteAll(KEYCHAIN_SERVICE);
  writeConfig({} as Config);
}

// --- Workspace alias derivation ---

export function deriveAlias(
  name: string,
  existingAliases: string[],
): string {
  let alias = name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 32);

  if (!alias) alias = "default";

  if (!existingAliases.includes(alias)) return alias;

  for (let i = 2; i <= 99; i++) {
    const candidate = `${alias}-${i}`;
    if (!existingAliases.includes(candidate)) return candidate;
  }

  return `${alias}-${Date.now()}`;
}

// --- Token update (for refresh) ---

export function updateWorkspaceTokens(
  alias: string,
  accessToken: string,
  refreshToken?: string,
): void {
  const config = readConfig();
  const ws = config.workspaces?.[alias];
  if (!ws) return;

  const accessStored = keychainSet({
    account: `access_token:${alias}`,
    value: accessToken,
    service: KEYCHAIN_SERVICE,
  });

  let refreshStored = true;
  if (refreshToken) {
    refreshStored = keychainSet({
      account: `refresh_token:${alias}`,
      value: refreshToken,
      service: KEYCHAIN_SERVICE,
    });
  }

  const useKeychain = accessStored && refreshStored;

  ws.access_token = useKeychain ? KEYCHAIN_PLACEHOLDER : accessToken;
  if (refreshToken) {
    ws.refresh_token = useKeychain ? KEYCHAIN_PLACEHOLDER : refreshToken;
  }

  writeConfig(config);
}

export function clearWorkspaceTokens(alias: string): void {
  const config = readConfig();
  const ws = config.workspaces?.[alias];
  if (!ws) return;

  keychainDelete(`access_token:${alias}`, KEYCHAIN_SERVICE);
  keychainDelete(`refresh_token:${alias}`, KEYCHAIN_SERVICE);

  ws.access_token = "";
  ws.refresh_token = undefined;
  writeConfig(config);
}

// --- V3 session (desktop token) ---

export function storeV3Session(session: Omit<V3Session, "token_v2"> & { token_v2: string }): {
  storage: "keychain" | "config";
} {
  const config = readConfig();

  const stored = keychainSet({
    account: "v3:token_v2",
    value: session.token_v2,
    service: KEYCHAIN_SERVICE,
  });

  config.v3 = {
    token_v2: stored ? KEYCHAIN_PLACEHOLDER : session.token_v2,
    user_id: session.user_id,
    user_email: session.user_email,
    user_name: session.user_name,
    space_id: session.space_id,
    space_name: session.space_name,
    extracted_at: session.extracted_at,
  };

  writeConfig(config);
  return { storage: stored ? "keychain" : "config" };
}

export function getV3Session(): V3Session | undefined {
  const config = readConfig();
  return config.v3;
}

export function resolveV3Token(): string | undefined {
  const session = getV3Session();
  if (!session) return undefined;

  if (session.token_v2 === KEYCHAIN_PLACEHOLDER) {
    return keychainGet("v3:token_v2", KEYCHAIN_SERVICE) ?? undefined;
  }
  return session.token_v2;
}

export function clearV3Session(): void {
  keychainDelete("v3:token_v2", KEYCHAIN_SERVICE);
  const config = readConfig();
  delete config.v3;
  writeConfig(config);
}
