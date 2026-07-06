import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

declare const AGENT_NOTION_BUILD_VERSION: string | undefined;

let cachedVersion: string | undefined;

export function getPackageVersion(): string {
  if (cachedVersion !== undefined) {
    return cachedVersion;
  }

  if (typeof AGENT_NOTION_BUILD_VERSION === "string" && AGENT_NOTION_BUILD_VERSION) {
    cachedVersion = AGENT_NOTION_BUILD_VERSION;
    return cachedVersion;
  }

  const envVersion =
    process.env.AGENT_NOTION_VERSION?.trim() || process.env.npm_package_version?.trim();
  if (envVersion) {
    cachedVersion = envVersion;
    return cachedVersion;
  }

  try {
    let dir = dirname(fileURLToPath(import.meta.url));
    for (let i = 0; i < 6; i++) {
      const candidate = join(dir, "package.json");
      if (existsSync(candidate)) {
        const raw = readFileSync(candidate, "utf8");
        const pkg = JSON.parse(raw) as { version?: unknown };
        const v = typeof pkg.version === "string" ? pkg.version.trim() : "";
        cachedVersion = v || "0.0.0";
        return cachedVersion;
      }
      const next = dirname(dir);
      if (next === dir) {
        break;
      }
      dir = next;
    }
  } catch {
    // fall through
  }

  cachedVersion = "0.0.0";
  return cachedVersion;
}
