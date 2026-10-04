import { describe, expect, it } from "vitest";

const catalogs = import.meta.glob<{ default: Record<string, string> }>("./locales/*/*.ts", { eager: true });

describe("all language catalogs", () => {
  for (const [path, thai] of Object.entries(catalogs).filter(([path]) => path.includes("/th/"))) {
    it(`has complete English copy and matching placeholders for ${path}`, () => {
      const english = catalogs[path.replace("/th/", "/en/")];
      expect(english).toBeDefined();
      expect(Object.keys(english.default).sort()).toEqual(Object.keys(thai.default).sort());
      for (const [key, value] of Object.entries(thai.default)) {
        expect(value.trim()).not.toBe("");
        expect(english.default[key].trim()).not.toBe("");
        const placeholders = (text: string) => [...text.matchAll(/\{([a-zA-Z0-9_]+)\}/g)].map((match) => match[1]).sort();
        expect(placeholders(english.default[key]), key).toEqual(placeholders(value));
      }
    });
  }
});
