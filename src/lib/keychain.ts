import { platform } from "node:os";
import { execFileSync } from "node:child_process";

const IS_MACOS = platform() === "darwin";

export const KEYCHAIN_SERVICE = "app.paulie.agent-notion";
export const KEYCHAIN_PLACEHOLDER = "__KEYCHAIN__";

export function keychainGet(account: string, service: string): string | null {
  if (!IS_MACOS) {
    return null;
  }
  try {
    const result = execFileSync(
      "security",
      ["find-generic-password", "-s", service, "-a", account, "-w"],
      { encoding: "utf8", stdio: ["pipe", "pipe", "ignore"] },
    );
    return result.trim() || null;
  } catch {
    return null;
  }
}

export function keychainSet(input: {
  account: string;
  value: string;
  service: string;
}): boolean {
  if (!IS_MACOS) {
    return false;
  }
  const { account, value, service } = input;
  try {
    try {
      execFileSync(
        "security",
        ["delete-generic-password", "-s", service, "-a", account],
        { stdio: ["pipe", "pipe", "ignore"] },
      );
    } catch {
      /* entry may not exist */
    }
    execFileSync(
      "security",
      ["add-generic-password", "-s", service, "-a", account, "-w", value],
      { stdio: ["pipe", "pipe", "ignore"] },
    );
    return true;
  } catch {
    return false;
  }
}

export function keychainDelete(
  account: string,
  service: string,
): boolean {
  if (!IS_MACOS) {
    return false;
  }
  try {
    execFileSync(
      "security",
      ["delete-generic-password", "-s", service, "-a", account],
      { stdio: ["pipe", "pipe", "ignore"] },
    );
    return true;
  } catch {
    return false;
  }
}

export function keychainDeleteAll(service: string): void {
  if (!IS_MACOS) {
    return;
  }
  for (;;) {
    try {
      execFileSync(
        "security",
        ["delete-generic-password", "-s", service],
        { stdio: ["pipe", "pipe", "ignore"] },
      );
    } catch {
      break;
    }
  }
}
