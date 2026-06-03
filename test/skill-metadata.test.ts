import { describe, expect, test } from "bun:test";

const MAX_DESCRIPTION_LENGTH = 1024;

describe("skill metadata", () => {
  test("keeps frontmatter description within harness limits", async () => {
    const text = await Bun.file("skills/agent-notion/SKILL.md").text();
    const match = text.match(/^---\n([\s\S]*?)\n---\n/);

    expect(match).not.toBeNull();

    const metadata = Bun.YAML.parse(match![1]!) as {
      name?: unknown;
      description?: unknown;
    };

    expect(metadata.name).toBe("agent-notion");
    expect(typeof metadata.description).toBe("string");
    expect((metadata.description as string).length).toBeLessThanOrEqual(MAX_DESCRIPTION_LENGTH);
  });
});
