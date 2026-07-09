import { describe, expect, test } from "vitest";
import { appTheme } from "./theme";

describe("appTheme", () => {
  test("uses MiSans as the primary font family", () => {
    expect(appTheme.typography.fontFamily).toMatch(/^"MiSans"/);
  });
});
