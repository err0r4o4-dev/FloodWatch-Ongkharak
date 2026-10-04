export type WeatherConditionKey = "clear" | "mainlyClear" | "partlyCloudy" | "overcast" | "fog" | "rimeFog" | "drizzleLight" | "drizzleModerate" | "drizzleDense" | "freezingDrizzleLight" | "freezingDrizzleDense" | "rainLight" | "rainModerate" | "rainHeavy" | "freezingRainLight" | "freezingRainHeavy" | "snowLight" | "snowModerate" | "snowHeavy" | "snowGrains" | "rainShowersLight" | "rainShowersModerate" | "rainShowersViolent" | "snowShowersLight" | "snowShowersHeavy" | "thunderstorm" | "thunderstormHailLight" | "thunderstormHailHeavy" | "unknown";

export type WeatherCondition = { key: WeatherConditionKey };

const conditions: Record<number, WeatherCondition> = {
  0: { key: "clear" },
  1: { key: "mainlyClear" },
  2: { key: "partlyCloudy" },
  3: { key: "overcast" },
  45: { key: "fog" },
  48: { key: "rimeFog" },
  51: { key: "drizzleLight" },
  53: { key: "drizzleModerate" },
  55: { key: "drizzleDense" },
  56: { key: "freezingDrizzleLight" },
  57: { key: "freezingDrizzleDense" },
  61: { key: "rainLight" },
  63: { key: "rainModerate" },
  65: { key: "rainHeavy" },
  66: { key: "freezingRainLight" },
  67: { key: "freezingRainHeavy" },
  71: { key: "snowLight" },
  73: { key: "snowModerate" },
  75: { key: "snowHeavy" },
  77: { key: "snowGrains" },
  80: { key: "rainShowersLight" },
  81: { key: "rainShowersModerate" },
  82: { key: "rainShowersViolent" },
  85: { key: "snowShowersLight" },
  86: { key: "snowShowersHeavy" },
  95: { key: "thunderstorm" },
  96: { key: "thunderstormHailLight" },
  99: { key: "thunderstormHailHeavy" }
};

export function conditionForCode(code: number): WeatherCondition {
  return conditions[code] ?? { key: "unknown" };
}
