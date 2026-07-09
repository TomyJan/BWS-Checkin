import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, test } from "vitest";

describe("web app manifest", () => {
  test("provides installable PNG icons", () => {
    const manifest = JSON.parse(readFileSync(resolve(__dirname, "../../public/manifest.webmanifest"), "utf8")) as {
      icons: Array<{ src: string; sizes: string; type: string }>;
    };

    expect(manifest.icons).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ src: "/icons/icon-192.png", sizes: "192x192", type: "image/png" }),
        expect.objectContaining({ src: "/icons/icon-512.png", sizes: "512x512", type: "image/png" })
      ])
    );
  });
});
