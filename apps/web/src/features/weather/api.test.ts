import { describe, expect, it } from "vitest";
import { isSafeProviderUrl, validateWeatherResponse, type WeatherResponse } from "./api";

const current = {
  observedAt: "2026-10-04T04:00:00Z",
  intervalSeconds: 900,
  temperatureC: 30,
  apparentTemperatureC: 34,
  humidityPercent: 72,
  precipitationMm: 1.5,
  weatherCode: 61,
  windSpeedKmh: 12,
  windDirectionDegrees: 180,
  isDay: true
};

const response: WeatherResponse = {
  location: { name: "Ongkharak", latitude: 14.1, longitude: 101 },
  source: { name: "Open-Meteo", url: "https://open-meteo.com/", kind: "MODEL" },
  freshness: "FRESH",
  retrievedAt: "2026-10-04T04:01:00Z",
  reason: null,
  current
};

describe("weather API boundary", () => {
  it("accepts the complete weather envelope and finite readings", () => {
    expect(validateWeatherResponse(response)).toEqual(response);
  });

  it("rejects missing nullable fields and non-finite numeric readings", () => {
    expect(validateWeatherResponse({ ...response, reason: undefined })).toBeNull();
    expect(validateWeatherResponse({ ...response, current: { ...current, humidityPercent: Number.NaN } })).toBeNull();
  });

  it("accepts unknown future WMO codes without unsafe assumptions", () => {
    expect(validateWeatherResponse({ ...response, current: { ...current, weatherCode: 98 } })).not.toBeNull();
  });

  it("enforces backend ranges, time ordering, and UNKNOWN null-current semantics", () => {
    expect(validateWeatherResponse({ ...response, current: { ...current, intervalSeconds: 59 } })).toBeNull();
    expect(validateWeatherResponse({ ...response, current: { ...current, temperatureC: 71 } })).toBeNull();
    expect(validateWeatherResponse({ ...response, current: { ...current, apparentTemperatureC: -101 } })).toBeNull();
    expect(validateWeatherResponse({ ...response, current: { ...current, precipitationMm: 1001 } })).toBeNull();
    expect(validateWeatherResponse({ ...response, current: { ...current, windSpeedKmh: 501 } })).toBeNull();
    expect(validateWeatherResponse({ ...response, retrievedAt: "2026-10-04T03:58:59Z" })).toBeNull();
    expect(validateWeatherResponse({ ...response, freshness: "UNKNOWN", current: null, retrievedAt: null })).not.toBeNull();
    expect(validateWeatherResponse({ ...response, freshness: "UNKNOWN" })).toBeNull();
    expect(validateWeatherResponse({ ...response, current: { ...current, observedAt: new Date(Date.now() + 61_000).toISOString() }, retrievedAt: new Date(Date.now() + 62_000).toISOString() })).toBeNull();
  });

  it("allows only the fixed Open-Meteo HTTPS provider origin", () => {
    expect(isSafeProviderUrl("https://open-meteo.com/")).toBe(true);
    expect(isSafeProviderUrl("javascript:alert(1)")).toBe(false);
    expect(isSafeProviderUrl("https://example.com/weather")).toBe(false);
    expect(isSafeProviderUrl("https://open-meteo.com:444/")).toBe(false);
  });

  it("accepts the backend one-minute model-time tolerance but rejects inconsistent receipt times", () => {
    const now = Date.now();
    expect(validateWeatherResponse({ ...response, retrievedAt: new Date(now).toISOString(), current: { ...current, observedAt: new Date(now + 30_000).toISOString() } })).not.toBeNull();
    expect(validateWeatherResponse({ ...response, freshness: "UNKNOWN", current: null })).toBeNull();
  });
});
