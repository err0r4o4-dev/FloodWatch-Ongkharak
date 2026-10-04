import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider, LanguageSwitcher, LANGUAGE_STORAGE_KEY } from "../../i18n";
import { ExternalDataPanel } from "./ExternalDataPanel";

const timestamp = "2026-10-04T04:00:00Z";

function record(overrides: Record<string, unknown> = {}) {
  return {
    key: "record-1",
    kind: "OBSERVATION",
    provenance: "OFFICIAL",
    station: { code: "NYK-1", externalId: "1", name: "สถานีคลองทดสอบ", latitude: 14.1, longitude: 101, province: "นครนายก", district: "องครักษ์", agency: "ThaiWater" },
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

function payload(sources = [source()]) {
  return { retrievedAt: timestamp, sources };
}

function renderPanel() {
  return render(<LanguageProvider><ExternalDataPanel /></LanguageProvider>);
}

function mockResponse(body: unknown, ok = true) {
  return { ok, status: ok ? 200 : 503, json: async () => body };
}

beforeEach(() => localStorage.setItem(LANGUAGE_STORAGE_KEY, "th"));
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); vi.restoreAllMocks(); localStorage.removeItem(LANGUAGE_STORAGE_KEY); });

describe("ExternalDataPanel", () => {
  it("expires retained freshness when refreshes hang and are repeatedly aborted", async () => {
    vi.useFakeTimers();
    const request = vi.fn().mockResolvedValueOnce(mockResponse(payload())).mockImplementation(() => new Promise(() => {}));
    vi.stubGlobal("fetch", request);
    const view = renderPanel();
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(screen.getByText("12.5 mm")).toBeInTheDocument();
    await act(async () => { await vi.advanceTimersByTimeAsync(75_000); });
    const panel = screen.getByRole("region", { name: "ข้อมูลจากแหล่งภายนอก" });
    expect(panel).toHaveTextContent("ไม่สามารถรับสถานะล่าสุดได้");
    expect(panel).not.toHaveTextContent("12.5 mm");
    expect(panel).not.toHaveTextContent("ข้อมูลบันทึกล่าสุด");
    await act(async () => { await vi.advanceTimersByTimeAsync(120_000); });
    expect(request).toHaveBeenCalledTimes(4);
    expect(panel).not.toHaveTextContent("12.5 mm");
    view.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
  it("marks retained evidence unknown when the next snapshot request fails", async () => {
    const request = vi.fn().mockResolvedValueOnce(mockResponse(payload())).mockRejectedValue(new Error("offline"));
    vi.stubGlobal("fetch", request);
    renderPanel();
    const panel = await screen.findByRole("region", { name: "ข้อมูลจากแหล่งภายนอก" });
    expect(await screen.findByText("12.5 mm")).toBeInTheDocument();
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "hidden" });
    fireEvent(document, new Event("visibilitychange"));
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
    fireEvent(document, new Event("visibilitychange"));
    await waitFor(() => expect(panel).toHaveTextContent("ไม่สามารถรับสถานะล่าสุดได้"));
    expect(panel).not.toHaveTextContent("12.5 mm");
    expect(panel).not.toHaveTextContent("ข้อมูลบันทึกล่าสุด");
  });
  it("explains expired evidence without attributing system expiry to the provider", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(mockResponse(payload([source({ records: [record({ value: null, freshness: "UNKNOWN", reason: "OBSERVATION_EXPIRED" })] })]))));
    renderPanel();
    expect(await screen.findByText(/ข้อมูลเกินช่วงเวลาที่ใช้งานได้/)).toBeInTheDocument();
    expect(screen.queryByText(/ผู้ให้บริการไม่ได้ระบุเหตุผล/)).not.toBeInTheDocument();
  });
  it("shows loading, source fetch health, and separate record freshness", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(mockResponse(payload([source({ freshness: "STALE", status: "DEGRADED", records: [record({ freshness: "FRESH" })] })]))));
    renderPanel();
    expect(screen.getByRole("status", { name: "กำลังโหลดข้อมูลจากแหล่งภายนอก" })).toBeInTheDocument();
    const panel = await screen.findByRole("region", { name: "ข้อมูลจากแหล่งภายนอก" });
    expect(panel).toHaveTextContent("ข้อมูลต้นทางเก่า");
    expect(panel).toHaveTextContent("ข้อมูลบันทึกล่าสุด");
  });

  it("renders unknown HII values without inventing a forecast day or rainfall unit", async () => {
    const hii = source({ code: "HII_WRF", name: "HII WRF-ROMS", kind: "FORECAST", provenance: "MODEL", records: [record({ key: "hii-1", kind: "FORECAST", provenance: "MODEL", station: null, observedAt: null, validFrom: null, validTo: null, model: "HII_WRF_ROMS", metric: "precipitation", value: null, unit: null, accumulationSeconds: null, latitude: 14.1, longitude: 101, freshness: "UNKNOWN", reason: "HII rainfall unit, timezone, and accumulation window are unverified" })] });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(mockResponse(payload([hii]))));
    renderPanel();
    const panel = await screen.findByRole("region", { name: "ข้อมูลจากแหล่งภายนอก" });
    fireEvent.click(screen.getByText("แสดงรายละเอียดแหล่งข้อมูล"));
    expect(panel).toHaveTextContent("ไม่ทราบสถานะ");
    expect(panel).toHaveTextContent("หน่วย ปริมาณฝน และช่วงเวลาของ HII ยังไม่ได้ตรวจสอบ");
    expect(panel).not.toHaveTextContent("มม.");
    expect(panel).not.toHaveTextContent("2026-10-04");
  });

  it("labels models apart from station observations and preserves rain periods and MSL datum", async () => {
    const rain = source({ records: [record({ key: "rain-1h", metric: "rain_1h", accumulationSeconds: 3600 }), record({ key: "rain-24h", metric: "rain_24h", accumulationSeconds: 86400 })] });
    const water = source({ code: "THAIWATER_WATER", name: "ThaiWater water stations", records: [record({ key: "water", metric: "waterlevel_msl", value: 1.2, unit: "m", datum: "MSL", accumulationSeconds: 0 })] });
    const gates = source({ code: "THAIWATER_GATE", name: "ThaiWater water gates", records: [record({ key: "gate-in", metric: "gate_in", value: 1, unit: "m", accumulationSeconds: null }), record({ key: "gate-out", metric: "gate_out", value: 0.5, unit: "m", accumulationSeconds: null })] });
    const model = source({ code: "OPENMETEO_ECMWF", name: "ECMWF via Open-Meteo", kind: "FORECAST", provenance: "MODEL", url: "https://open-meteo.com", records: [record({ key: "forecast", kind: "FORECAST", provenance: "MODEL", station: null, observedAt: null, validFrom: timestamp, validTo: "2026-10-04T05:00:00Z", model: "ecmwf", metric: "precipitation", value: 2, unit: "mm", accumulationSeconds: 3600, latitude: 14.1, longitude: 101 })] });
    const glofas = source({ code: "GLOFAS", name: "GloFAS via Open-Meteo", kind: "FORECAST", provenance: "MODEL", url: "https://open-meteo.com", records: [record({ key: "discharge", kind: "FORECAST", provenance: "MODEL", station: null, observedAt: null, validFrom: timestamp, validTo: "2026-10-05T04:00:00Z", model: "glofas", metric: "river_discharge", value: 10, unit: "m³/s", accumulationSeconds: null, latitude: 14.1, longitude: 101 })] });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(mockResponse(payload([rain, water, gates, model, glofas]))));
    renderPanel();
    const panel = await screen.findByRole("region", { name: "ข้อมูลจากแหล่งภายนอก" });
    fireEvent.click(screen.getByText("แสดงรายละเอียดแหล่งข้อมูล"));
    expect(panel).toHaveTextContent("ฝนสะสม 1 ชั่วโมง");
    expect(panel).toHaveTextContent("ฝนสะสม 24 ชั่วโมง");
    expect(panel).toHaveTextContent("ระดับน้ำอ้างอิง MSL ไม่ใช่ความลึกน้ำท่วม");
    expect(panel).toHaveTextContent("ประตูน้ำเข้า");
    expect(panel).toHaveTextContent("ประตูน้ำออก");
    expect(panel).toHaveTextContent("อัตราการไหลของแม่น้ำ ไม่ใช่ฝนสะสม");
    expect(panel).toHaveTextContent("แบบจำลอง ไม่ใช่สถานีตรวจวัด");
    expect(panel).toHaveTextContent("สถานีตรวจวัด");
  });

  it("does not call zero current alerts an all-clear", async () => {
    const alerts = source({ code: "TMD_CAP", name: "TMD official warnings", kind: "OFFICIAL_ALERT", url: "https://www.tmd.go.th", records: [], recordCount: 0 });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(mockResponse(payload([alerts]))));
    renderPanel();
    const panel = await screen.findByRole("region", { name: "ข้อมูลจากแหล่งภายนอก" });
    expect(panel).toHaveTextContent("ไม่มีประกาศปัจจุบัน");
    expect(panel).not.toHaveTextContent("ปลอดภัย");
    expect(panel).not.toHaveTextContent("ไม่มีความเสี่ยง");
  });

  it("shows not configured and no-data states, then retries a transport failure", async () => {
    const request = vi.fn()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(mockResponse(payload([source({ enabled: false, status: "NOT_CONFIGURED", records: [], recordCount: 0 })])));
    vi.stubGlobal("fetch", request);
    renderPanel();
    expect(await screen.findByRole("status", { name: "ไม่สามารถโหลดข้อมูลจากแหล่งภายนอกได้" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ลองโหลดข้อมูลจากแหล่งภายนอกอีกครั้ง" }));
    const panel = await screen.findByRole("region", { name: "ข้อมูลจากแหล่งภายนอก" });
    expect(panel).toHaveTextContent("ยังไม่ได้ตั้งค่าแหล่งข้อมูล");
    expect(panel).toHaveTextContent("ไม่มีข้อมูลบันทึก");
    expect(request).toHaveBeenCalledTimes(2);
  });

  it("refreshes only while visible and aborts the active request on cleanup", async () => {
    vi.useFakeTimers();
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "hidden" });
    let signal: AbortSignal | undefined;
    const request = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      signal = init?.signal ?? undefined;
      return new Promise<Response>(() => undefined);
    });
    vi.stubGlobal("fetch", request);
    const view = renderPanel();
    expect(request).not.toHaveBeenCalled();
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
    fireEvent(document, new Event("visibilitychange"));
    expect(request).toHaveBeenCalledTimes(1);
    await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
    expect(request).toHaveBeenCalledTimes(2);
    view.unmount();
    expect(signal?.aborted).toBe(true);
  });

  it("updates the panel language without exposing backend codes", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(mockResponse(payload([source({ status: "DEGRADED", freshness: "STALE", errorCode: "FETCH_FAILED" })]))));
    render(<LanguageProvider><ExternalDataPanel /><LanguageSwitcher /></LanguageProvider>);
    const panel = await screen.findByRole("region", { name: "ข้อมูลจากแหล่งภายนอก" });
    expect(panel).toHaveTextContent("ข้อมูลต้นทางเก่า");
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    await waitFor(() => expect(screen.getByRole("region", { name: "External data for Ongkharak" })).toHaveTextContent("Source data is stale"));
    expect(screen.getByRole("region", { name: "External data for Ongkharak" })).not.toHaveTextContent("FETCH_FAILED");
  });
});
