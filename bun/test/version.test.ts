import { describe, expect, test } from "bun:test";
import { getPackageVersion } from "../src/lib/version.ts";

describe("getPackageVersion", () => {
  test("returns a valid semver string", () => {
    const version = getPackageVersion();
    expect(version).toMatch(/^\d+\.\d+\.\d+/);
  });

  test("matches version from package.json", async () => {
    const pkg = await Bun.file("package.json").json();
    expect(getPackageVersion()).toBe(pkg.version);
  });
});
