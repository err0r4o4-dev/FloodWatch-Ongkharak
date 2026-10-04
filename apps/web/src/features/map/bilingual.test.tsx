import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider, LanguageSwitcher, LANGUAGE_STORAGE_KEY } from "../../i18n";
import { MapPage } from "./MapPage";

vi.mock("./components/SituationMap", async () => {
  const React = await import("react");
  return { SituationMap: ({ onBoundsChange }: { onBoundsChange: (bounds: { west: number; south: number; east: number; north: number }) => void }) => {
    React.useEffect(() => { onBoundsChange({ west: 100.9, south: 14, east: 101.2, north: 14.3 }); }, [onBoundsChange]);
    return <div aria-label="แผนที่สถานการณ์แบบโต้ตอบ" />;
  } };
});

const source = { id: "source-1", code: "sample", name: "หน่วยงานทดสอบ", sourceType: "OFFICIAL" as const };
const station = { id: "station-1", name: "สถานีคลองทดสอบ", source, active: true, location: { type: "Point" as const, coordinates: [101, 14.1] as [number, number] } };
const risk = { id: "risk-1", name: "พื้นที่เสี่ยงทดสอบ", description: "คำอธิบายต้นฉบับ", source, riskLevel: "HIGH", effectiveRiskLevel: "HIGH", freshness: "FRESH", validFrom: null, validTo: null, geometry: { type: "MultiPolygon" as const, coordinates: [[[[101, 14], [101.1, 14], [101.1, 14.1], [101, 14]]]] } };

function mockApi() {
  const collections: Record<string, unknown[]> = {
    "/api/v1/water-stations": [station],
    "/api/v1/risk-areas": [risk]
  };
  const request = vi.fn((url: string) => Promise.resolve({ ok: true, json: () => Promise.resolve({ items: collections[new URL(url, "http://localhost").pathname] ?? [], limit: 100, offset: 0 }) }));
  vi.stubGlobal("fetch", request);
  return request;
}

beforeEach(() => localStorage.setItem(LANGUAGE_STORAGE_KEY, "th"));
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); localStorage.removeItem(LANGUAGE_STORAGE_KEY); });

describe("map bilingual state", () => {
  it("switches copy without resetting selected feature details or search query", async () => {
    const request = mockApi();
    render(<LanguageProvider><MemoryRouter><MapPage /><LanguageSwitcher /></MemoryRouter></LanguageProvider>);

    const search = await screen.findByRole("searchbox", { name: "ค้นหาสถานีหรือพื้นที่ในขอบเขตที่โหลด" });
    fireEvent.change(search, { target: { value: "พื้นที่" } });
    fireEvent.click(await screen.findByRole("button", { name: /พื้นที่เสี่ยงทดสอบ/ }));
    expect(screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).toHaveTextContent("คำอธิบายต้นฉบับ");
    fireEvent.change(search, { target: { value: "พื้นที่" } });
    const requestsBeforeLocaleChange = request.mock.calls.length;

    fireEvent.click(screen.getByRole("button", { name: "EN" }));

    await waitFor(() => {
      expect(screen.getByRole("searchbox", { name: "Search loaded stations or areas" })).toHaveValue("พื้นที่");
      expect(screen.getByRole("region", { name: "Selected location information" })).toHaveTextContent("คำอธิบายต้นฉบับ");
      expect(screen.getByRole("region", { name: "Selected location information" })).toHaveTextContent("Original content");
      expect(request.mock.calls.length).toBe(requestsBeforeLocaleChange);
    });
  });

  it("switches copy without resetting selection, layer visibility, search query, or fetching again", async () => {
    const request = mockApi();
    render(<LanguageProvider><MemoryRouter><MapPage /><LanguageSwitcher /></MemoryRouter></LanguageProvider>);

    const search = await screen.findByRole("searchbox", { name: "ค้นหาสถานีหรือพื้นที่ในขอบเขตที่โหลด" });
    await screen.findByRole("checkbox", { name: "สถานีวัดน้ำ" });
    fireEvent.click(screen.getByRole("button", { name: "พื้นที่เสี่ยง" }));
    fireEvent.click(await screen.findByRole("button", { name: /พื้นที่เสี่ยงทดสอบ/ }));
    await waitFor(() => expect(screen.getByRole("button", { name: /พื้นที่เสี่ยงทดสอบ/ })).toHaveAttribute("aria-pressed", "true"));
    fireEvent.click(screen.getByRole("checkbox", { name: "สถานีวัดน้ำ" }));
    fireEvent.change(search, { target: { value: "คลอง" } });
    await waitFor(() => expect(request.mock.calls.length).toBe(9));
    const requestsBeforeLocaleChange = request.mock.calls.length;

    fireEvent.click(screen.getByRole("button", { name: "EN" }));

    await waitFor(() => {
      expect(screen.getByRole("searchbox", { name: "Search loaded stations or areas" })).toHaveValue("คลอง");
      expect(screen.getByRole("checkbox", { name: "Water stations" })).not.toBeChecked();
      expect(screen.getByRole("button", { name: /พื้นที่เสี่ยงทดสอบ/ })).toHaveAttribute("aria-pressed", "true");
      expect(request.mock.calls.length).toBe(requestsBeforeLocaleChange);
    });
  });
});
