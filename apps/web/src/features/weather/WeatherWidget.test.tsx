import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { LanguageProvider, LANGUAGE_STORAGE_KEY } from "../../i18n";
import { WeatherProvider } from "./WeatherProvider";
import { WeatherWidget } from "./WeatherWidget";

function weather(freshness = "FRESH", ageHours = 0) {
  return { location: { name: "Ongkharak", latitude: 14.1, longitude: 101 }, source: { name: "Open-Meteo", url: "https://open-meteo.com/", kind: "MODEL" }, freshness, retrievedAt: new Date().toISOString(), reason: null, current: { observedAt: new Date(Date.now() - ageHours * 60 * 60 * 1000).toISOString(), intervalSeconds: 900, temperatureC: 28.4, apparentTemperatureC: 30, humidityPercent: 85, precipitationMm: 0.2, weatherCode: 61, windSpeedKmh: 4, windDirectionDegrees: 180, isDay: true } };
}

function mount() {
  return render(<LanguageProvider><WeatherProvider><WeatherWidget /><WeatherWidget /></WeatherProvider></LanguageProvider>);
}

beforeEach(() => {
  localStorage.setItem(LANGUAGE_STORAGE_KEY, "th");
  Object.defineProperty(HTMLDialogElement.prototype, "showModal", { configurable: true, value: vi.fn(function (this: HTMLDialogElement) { this.setAttribute("open", ""); }) });
  Object.defineProperty(HTMLDialogElement.prototype, "close", { configurable: true, value: vi.fn(function (this: HTMLDialogElement) { this.removeAttribute("open"); this.dispatchEvent(new Event("close")); }) });
});
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); localStorage.clear(); sessionStorage.clear(); });

it("shares data between cards and details and supports minimize/reopen", async () => {
  const request = vi.fn().mockResolvedValue({ ok: true, json: async () => weather() });
  vi.stubGlobal("fetch", request);
  mount();
  expect(await screen.findAllByText("28.4°C")).toHaveLength(2);
  fireEvent.click(screen.getAllByRole("button", { name: "แสดงทั้งหมด" })[0]);
  expect(screen.getByRole("dialog", { name: "รายละเอียดสภาพอากาศองครักษ์" })).toHaveTextContent("Open-Meteo");
  expect(screen.getByRole("dialog")).not.toHaveTextContent("แบบจำลองสภาพอากาศ ไม่ใช่สถานีตรวจวัด");
  expect(request).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: "ปิดรายละเอียดสภาพอากาศ" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  fireEvent.click(screen.getAllByRole("button", { name: "ย่อการ์ดสภาพอากาศ" })[0]);
  fireEvent.click(screen.getByRole("button", { name: "เปิดการ์ดสภาพอากาศ" }));
  expect(screen.getAllByText("28.4°C")).toHaveLength(2);
  expect(request).toHaveBeenCalledTimes(1);
});

it("shows loading without inventing readings", () => {
  vi.stubGlobal("fetch", vi.fn(() => new Promise(() => undefined)));
  mount();
  expect(screen.getAllByRole("status")).toHaveLength(2);
  expect(screen.queryByText("28.4°C")).not.toBeInTheDocument();
});

it("opens a centered detail dialog without a duplicate visible summary", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => weather() }));
  render(<LanguageProvider><WeatherProvider><WeatherWidget /></WeatherProvider></LanguageProvider>);
  await screen.findByText("28.4°C");
  const opener = screen.getByRole("button", { name: "แสดงทั้งหมด" });
  const floating = opener.closest(".weather-floating");
  expect(floating).toBeVisible();
  fireEvent.click(opener);
  expect(floating).not.toBeVisible();
  expect(screen.getByRole("dialog")).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "ปิดรายละเอียดสภาพอากาศ" }));
  expect(floating).toBeVisible();
  expect(opener).toHaveFocus();
});

it("remembers minimize state in this tab and reopens from a cloud-only button", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => weather() }));
  const renderSingle = () => render(<LanguageProvider><WeatherProvider><WeatherWidget /></WeatherProvider></LanguageProvider>);
  const view = renderSingle();
  await screen.findByText("28.4°C");
  fireEvent.click(screen.getByRole("button", { name: "ย่อการ์ดสภาพอากาศ" }));
  expect(sessionStorage.getItem("floodwatch.weather.minimized")).toBe("true");
  expect(screen.getByRole("button", { name: "เปิดการ์ดสภาพอากาศ" }).textContent).toBe("");
  view.unmount();
  renderSingle();
  const reopen = screen.getByRole("button", { name: "เปิดการ์ดสภาพอากาศ" });
  expect(reopen.querySelector("svg")).not.toBeNull();
  expect(reopen.textContent).toBe("");
  expect(screen.queryByRole("button", { name: "แสดงทั้งหมด" })).not.toBeInTheDocument();
  fireEvent.click(reopen);
  expect(screen.getByRole("button", { name: "แสดงทั้งหมด" })).toBeVisible();
  expect(sessionStorage.getItem("floodwatch.weather.minimized")).toBe("false");
});

it("remains usable when session storage is unavailable", () => {
  vi.stubGlobal("fetch", vi.fn(() => new Promise(() => undefined)));
  const original = Storage.prototype.getItem;
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(function (this: Storage, key: string) {
    if (this === sessionStorage) throw new Error("storage disabled");
    return original.call(this, key);
  });
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(function (this: Storage) {
    if (this === sessionStorage) throw new Error("storage disabled");
  });
  render(<LanguageProvider><WeatherProvider><WeatherWidget /></WeatherProvider></LanguageProvider>);
  fireEvent.click(screen.getByRole("button", { name: "ย่อการ์ดสภาพอากาศ" }));
  fireEvent.click(screen.getByRole("button", { name: "เปิดการ์ดสภาพอากาศ" }));
  expect(screen.getByRole("button", { name: "แสดงทั้งหมด" })).toBeVisible();
});

it("retries transport errors once for all consumers and hides unknown readings", async () => {
  const request = vi.fn().mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce({ ok: true, json: async () => weather("UNKNOWN") });
  vi.stubGlobal("fetch", request);
  mount();
  expect(await screen.findAllByText("ไม่สามารถโหลดข้อมูลสภาพอากาศได้")).toHaveLength(2);
  fireEvent.click(screen.getAllByRole("button", { name: "ลองโหลดสภาพอากาศอีกครั้ง" })[0]);
  await waitFor(() => expect(screen.getAllByText("ไม่ทราบสถานะ")).toHaveLength(2));
  expect(screen.queryByText("28.4°C")).not.toBeInTheDocument();
  expect(request).toHaveBeenCalledTimes(2);
});

it("labels old readings as stale and hides expired readings", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => weather("FRESH", 2) }));
  const view = mount();
  expect(await screen.findAllByText("ข้อมูลเก่า ไม่ควรใช้ยืนยันความปลอดภัย")).toHaveLength(2);
  expect(screen.getAllByText("28.4°C")).toHaveLength(2);
  view.unmount();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => weather("FRESH", 7) }));
  mount();
  await waitFor(() => expect(screen.getAllByRole("button", { name: "ลองโหลดสภาพอากาศอีกครั้ง" })).toHaveLength(2));
  expect(screen.queryByText("28.4°C")).not.toBeInTheDocument();
});
