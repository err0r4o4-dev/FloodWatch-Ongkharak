import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider, LANGUAGE_STORAGE_KEY } from "../../i18n";
import { displayFreshness, WEATHER_AGE_REEVALUATION_INTERVAL_MS, WEATHER_REFRESH_INTERVAL_MS, WeatherPanel } from "./WeatherPanel";
import type { WeatherResponse } from "./api";

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

function weather(overrides: Record<string, unknown> = {}) {
  return {
    location: { name: "Ongkharak", latitude: 14.1, longitude: 101 },
    source: { name: "Open-Meteo", url: "https://open-meteo.com/", kind: "MODEL" },
    freshness: "FRESH",
    retrievedAt: "2026-10-04T04:01:00Z",
    reason: null,
    current,
    ...overrides
  };
}

function renderPanel() {
  return render(<LanguageProvider><WeatherPanel /></LanguageProvider>);
}

beforeEach(() => localStorage.setItem(LANGUAGE_STORAGE_KEY, "th"));
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); localStorage.removeItem(LANGUAGE_STORAGE_KEY); });

describe("WeatherPanel", () => {
  it("renders model weather with interval precipitation, timestamps, caveats, and attribution", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => weather() }));
    renderPanel();

    expect(screen.getByRole("status", { name: "กำลังโหลดข้อมูลสภาพอากาศ" })).toBeInTheDocument();
    expect(await screen.findByRole("region", { name: "สภาพอากาศองครักษ์" })).toHaveTextContent("ฝนตกเล็กน้อย");
    expect(screen.getByText("1.5 มม. / 15 นาที")).toBeInTheDocument();
    expect(screen.getByText(/แบบจำลองสภาพอากาศ ไม่ใช่สถานีตรวจวัด/)).toBeInTheDocument();
    expect(screen.getByText(/ไม่ใช่ข้อมูลยืนยันความปลอดภัยน้ำท่วม/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open-Meteo" })).toHaveAttribute("href", "https://open-meteo.com/");
    expect(screen.getByText(/เวลาแบบจำลอง/)).toBeInTheDocument();
    expect(screen.getByText(/เวลารับข้อมูล/)).toBeInTheDocument();
  });

  it("shows a clearly stale reading and never presents it as reassuring", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => weather({ freshness: "STALE", reason: "ข้อมูลจากแคชเก่า", current: { ...current, observedAt: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString() }, retrievedAt: new Date().toISOString() }) }));
    renderPanel();

    const panel = await screen.findByRole("region", { name: "สภาพอากาศองครักษ์" });
    expect(panel).toHaveTextContent("ข้อมูลเก่า");
    expect(panel).toHaveTextContent("ข้อมูลจากแคชเก่า");
    expect(panel).toHaveTextContent("ไม่ใช่ข้อมูลยืนยันความปลอดภัยน้ำท่วม");
    expect(panel).not.toHaveTextContent("ยืนยันว่าปลอดภัย");
  });

  it("translates backend reason codes instead of exposing English codes in Thai", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => weather({ freshness: "STALE", reason: "UPSTREAM_UNAVAILABLE", current: { ...current, observedAt: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString() }, retrievedAt: new Date().toISOString() }) }));
    renderPanel();

    const panel = await screen.findByRole("region", { name: "สภาพอากาศองครักษ์" });
    expect(panel).toHaveTextContent("แหล่งข้อมูลต้นทางไม่พร้อมใช้งาน");
    expect(panel).not.toHaveTextContent("UPSTREAM_UNAVAILABLE");
  });

  it("renders the no-evidence envelope and retries a transport error", async () => {
    const request = vi.fn()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce({ ok: false, status: 503, json: async () => weather({ freshness: "UNKNOWN", current: null, retrievedAt: null, reason: "ยังไม่มีข้อมูลสภาพอากาศ" }) });
    vi.stubGlobal("fetch", request);
    renderPanel();

    expect(await screen.findByRole("status", { name: "ไม่สามารถโหลดข้อมูลสภาพอากาศได้" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ลองโหลดสภาพอากาศอีกครั้ง" }));
    await waitFor(() => expect(screen.getByRole("region", { name: "สภาพอากาศองครักษ์" })).toHaveTextContent("ยังไม่มีข้อมูลสภาพอากาศ"));
    expect(request).toHaveBeenCalledTimes(2);
  });

  it("reevaluates age and cleans up bounded refresh timers and requests", () => {
    const data = weather() as WeatherResponse;
    const observedAt = Date.parse(current.observedAt);
    expect(displayFreshness(data, observedAt + 30 * 60 * 1000, false)).toBe("FRESH");
    expect(displayFreshness(data, observedAt + 61 * 60 * 1000, false)).toBe("STALE");
    expect(displayFreshness(data, observedAt + 7 * 60 * 60 * 1000, false)).toBe("UNKNOWN");
    expect(displayFreshness(data, observedAt + 30 * 60 * 1000, true)).toBe("STALE");

    let signal: AbortSignal | undefined;
    const request = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.signal) signal = init.signal;
      return new Promise<Response>(() => undefined);
    });
    vi.stubGlobal("fetch", request);
    const setInterval = vi.spyOn(window, "setInterval");
    const clearInterval = vi.spyOn(window, "clearInterval");
    const view = renderPanel();
    expect(setInterval).toHaveBeenCalledWith(expect.any(Function), WEATHER_REFRESH_INTERVAL_MS);
    expect(setInterval).toHaveBeenCalledWith(expect.any(Function), WEATHER_AGE_REEVALUATION_INTERVAL_MS);
    view.unmount();
    expect(signal?.aborted).toBe(true);
    expect(clearInterval).toHaveBeenCalledTimes(2);
  });
});
