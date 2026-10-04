import { describe, expect, it } from "vitest";
import { conditionForCode } from "./conditions";

describe("WMO weather condition mapping", () => {
  it("maps every supported Open-Meteo WMO code explicitly", () => {
    const supported = [0, 1, 2, 3, 45, 48, 51, 53, 55, 56, 57, 61, 63, 65, 66, 67, 71, 73, 75, 77, 80, 81, 82, 85, 86, 95, 96, 99];
    for (const code of supported) expect(conditionForCode(code).key).not.toBe("unknown");
  });

  it("uses an honest fallback for unsupported codes", () => {
    expect(conditionForCode(999).key).toBe("unknown");
  });
});
