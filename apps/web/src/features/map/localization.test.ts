import { describe, expect, it } from "vitest";
import th from "../../i18n/locales/th/map";
import en from "../../i18n/locales/en/map";

describe("map localization catalogs", () => {
  it("keeps Thai and English keys aligned with non-empty strings", () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(th).sort());
    expect(Object.values(th).every((value) => value.trim().length > 0)).toBe(true);
    expect(Object.values(en).every((value) => value.trim().length > 0)).toBe(true);
  });
});
