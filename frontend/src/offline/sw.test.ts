import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, test } from "vitest";

describe("service worker precache", () => {
  test("collects built asset URLs from index HTML during install", () => {
    const source = readFileSync(resolve(__dirname, "../../public/sw.js"), "utf8");

    expect(source).toContain("collectIndexAssetURLs");
    expect(source).toContain("cache.addAll([...APP_SHELL, ...assetURLs])");
  });
});
