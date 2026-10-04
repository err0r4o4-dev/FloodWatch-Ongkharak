import { afterEach, describe, expect, it, vi } from "vitest";
import { apiConfig } from "../../shared/config/api";
import { fetchExternalData, isSafeProviderUrl, validateExternalDataResponse } from "./api";

const timestamp = "2026-10-04T04:00:00Z";

function record(overrides: Record<string, unknown> = {}) {
  return {
    key: "rain-1h",
    kind: "OBSERVATION",
    provenance: "OFFICIAL",
    station: { code: "NYK-1", externalId: "1", name: "Synthetic station", latitude: 14.1, longitude: 101, province: "Nakhon Nayok", district: "Ongkharak", agency: "ThaiWater" },
    observedAt: timestamp,
    issuedAt: null,
    validFrom: null,
    validTo: null,
    model: null,
    member: null,
    metric: "rain_1h",
    value: 12.5,
    unit: "mm",
    datum: null,
    accumulationSeconds: 3600,
    latitude: null,
    longitude: null,
    headline: null,
    description: null,
    alertIdentifier: null,
    alertMessageType: null,
    alertReferences: [],
    areaCodes: [],
    geometry: null,
    reason: null,
    freshness: "FRESH",
    ...overrides
  };
}

function source(overrides: Record<string, unknown> = {}) {
  return {
    code: "THAIWATER_RAIN",
    name: "ThaiWater rain stations",
    kind: "OBSERVATION",
    provenance: "OFFICIAL",
    url: "https://www.thaiwater.net",
    enabled: true,
    status: "AVAILABLE",
    freshness: "FRESH",
    lastAttemptAt: timestamp,
    lastSuccessAt: timestamp,
    errorCode: null,
    recordCount: 1,
    records: [record()],
    ...overrides
  };
}

function response(overrides: Record<string, unknown> = {}) {
  return { retrievedAt: timestamp, sources: [source()], ...overrides };
}

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("external data API contract", () => {
  it("accepts omitted optional fields from the Go HII wire format without dropping other sources", () => {
    const hiiRecord = { key: "hii-wire", kind: "FORECAST", provenance: "MODEL", model: "HII_WRF_ROMS", metric: "precipitation", value: null, latitude: 14.1, longitude: 101, freshness: "UNKNOWN", reason: "HII rainfall unit, timezone, and accumulation window are unverified" };
    const hii = source({ code: "HII_WRF", name: "HII WRF-ROMS", kind: "FORECAST", provenance: "MODEL", url: "https://www.thaiwater.net/forecast/wrfroms", records: [hiiRecord] });
    const result = validateExternalDataResponse(response({ sources: [source(), hii] }));
    expect(result?.sources).toHaveLength(2);
    expect(result?.sources[1].records[0].unit).toBeNull();
  });
  it("validates source health and nullable records without fabricating values", () => {
    const result = validateExternalDataResponse(response({ sources: [source({ code: "HII_WRF", name: "HII WRF-ROMS", kind: "FORECAST", provenance: "MODEL", url: "https://www.thaiwater.net/forecast/wrfroms", records: [record({ value: null, reason: "HII rainfall unit, timezone, and accumulation window are unverified", validFrom: null, validTo: null, kind: "FORECAST", provenance: "MODEL", model: "HII_WRF_ROMS", metric: "precipitation", unit: null, accumulationSeconds: null, station: null, observedAt: null, latitude: 14.1, longitude: 101, freshness: "UNKNOWN" })] })] }));
    expect(result?.sources[0].records[0].value).toBeNull();
    expect(result?.sources[0].records[0].validFrom).toBeNull();
  });

  it("accepts Thai water and gate metrics plus GloFAS discharge without rainfall accumulation", () => {
    const water = source({ code: "THAIWATER_WATER", records: [record({ key: "water-msl", metric: "waterlevel_msl", value: 1.2, unit: "m", datum: "MSL", accumulationSeconds: null }), record({ key: "water-m", metric: "waterlevel_m", value: 1.1, unit: "m", datum: "UNKNOWN", accumulationSeconds: null })], recordCount: 2 });
    const gate = source({ code: "THAIWATER_GATE", name: "ThaiWater water gates", records: [record({ key: "gate-in", metric: "gate_in", value: 1, unit: "m", accumulationSeconds: null }), record({ key: "gate-out", metric: "gate_out", value: 0.5, unit: "m", accumulationSeconds: null })], recordCount: 2 });
    const glofasRecord: Record<string, unknown> = record({ key: "glofas-1", kind: "FORECAST", provenance: "MODEL", station: null, observedAt: null, validFrom: timestamp, validTo: "2026-10-05T04:00:00Z", model: "glofas", metric: "river_discharge", value: 10, unit: "m³/s", latitude: 14.1, longitude: 101 });
    delete glofasRecord.accumulationSeconds;
    const glofas = source({ code: "GLOFAS", name: "GloFAS via Open-Meteo", kind: "FORECAST", provenance: "MODEL", url: "https://open-meteo.com", records: [glofasRecord], recordCount: 1 });

    const result = validateExternalDataResponse(response({ sources: [water, gate, glofas] }));
    expect(result?.sources[0].records.map((item) => item.metric)).toEqual(["waterlevel_msl", "waterlevel_m"]);
    expect(result?.sources[1].records.map((item) => item.metric)).toEqual(["gate_in", "gate_out"]);
    expect(result?.sources[2].records[0].accumulationSeconds).toBeNull();
  });

  it("rejects malformed forecast records that are not the explicit HII unknown contract", () => {
    const base = { code: "HII_WRF", name: "HII WRF-ROMS", kind: "FORECAST", provenance: "MODEL", url: "https://www.thaiwater.net/forecast/wrfroms" };
    for (const overrides of [
      { freshness: "FRESH" },
      { model: "OTHER_MODEL" },
      { reason: "provider value is missing" },
      { validFrom: timestamp, validTo: "2026-10-05T04:00:00Z" }
    ]) {
      expect(validateExternalDataResponse(response({ sources: [source({ ...base, records: [record({ value: null, kind: "FORECAST", provenance: "MODEL", station: null, observedAt: null, validFrom: null, validTo: null, model: "HII_WRF_ROMS", metric: "precipitation", unit: null, accumulationSeconds: null, latitude: 14.1, longitude: 101, freshness: "UNKNOWN", reason: "HII rainfall unit, timezone, and accumulation window are unverified", ...overrides })] })] }))).toBeNull();
    }
  });

  it("rejects unsafe origins, non-finite numbers, invalid units, and non-UTC timestamps", () => {
    expect(isSafeProviderUrl("https://www.thaiwater.net")).toBe(true);
    expect(isSafeProviderUrl("https://open-meteo.com")).toBe(true);
    expect(isSafeProviderUrl("https://www.tmd.go.th")).toBe(true);
    expect(isSafeProviderUrl("https://evil.example.test")).toBe(false);
    expect(validateExternalDataResponse(response({ retrievedAt: "2026-10-04T04:00:00+07:00" }))).toBeNull();
    expect(validateExternalDataResponse(response({ sources: [source({ url: "https://evil.example.test", records: [record({ value: Number.NaN })] })] }))).toBeNull();
    expect(validateExternalDataResponse(response({ sources: [source({ records: [record({ unit: "m", accumulationSeconds: 3600 })] })] }))).toBeNull();
  });

  it("uses the API origin and preserves aborts from the browser request", async () => {
    expect(validateExternalDataResponse(response())).not.toBeNull();
    const request = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.signal).toBeInstanceOf(AbortSignal);
      return Promise.resolve({ ok: true, status: 200, json: async () => response() });
    });
    vi.stubGlobal("fetch", request);
    await fetchExternalData(new AbortController().signal);
    expect(request).toHaveBeenCalledWith(`${apiConfig.baseUrl}/external-data/ongkharak`, expect.objectContaining({ signal: expect.any(AbortSignal) }));

    const controller = new AbortController();
    const aborted = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => new Promise((_resolve, reject) => {
      init?.signal?.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")), { once: true });
    }));
    vi.stubGlobal("fetch", aborted);
    const pending = fetchExternalData(controller.signal);
    controller.abort();
    await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  });
});
