import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, expect, it, vi } from "vitest";
import { OverviewMapPreview } from "./OverviewMapPreview";

vi.mock("../map/components/SituationMap", async () => {
  const React = await import("react");
  return { SituationMap: ({ items, onBoundsChange, onSelect }: { items: { water: Array<{ id: string; name: string }> }; onBoundsChange: (bounds: { west: number; south: number; east: number; north: number }) => void; onSelect: (selection: unknown) => void }) => {
    const bounds = { west: 100.9, south: 14, east: 101.2, north: 14.3 };
    React.useEffect(() => onBoundsChange(bounds), [onBoundsChange]);
    return <div aria-label="แผนที่สถานการณ์แบบโต้ตอบ"><button type="button" aria-label="เลือกสถานีตัวอย่าง" onClick={() => onSelect({ layer: "water", feature: items.water[0] })} disabled={!items.water.length}>เลือกสถานีตัวอย่าง</button><button type="button" aria-label="เผยแพร่ขอบเขตเดิมอีกครั้ง" onClick={() => onBoundsChange(bounds)}>เผยแพร่ขอบเขตเดิมอีกครั้ง</button></div>;
  } };
});

afterEach(() => vi.unstubAllGlobals());

it("loads only real map layers and can disable a layer", async () => {
  const request = vi.fn((_url: string) => Promise.resolve({ ok: true, json: () => Promise.resolve({ items: [], limit: 100, offset: 0 }) }));
  vi.stubGlobal("fetch", request);
  render(<MemoryRouter><OverviewMapPreview /></MemoryRouter>);

  expect(screen.getByRole("link", { name: "ดูแผนที่เต็มหน้า →" })).toHaveAttribute("href", "/map");
  await waitFor(() => expect(request).toHaveBeenCalledTimes(5));
  expect(request.mock.calls.map(([url]) => url)).toEqual(expect.arrayContaining([
    expect.stringContaining("/risk-areas?"), expect.stringContaining("/flood-events?")
  ]));

  fireEvent.click(screen.getByRole("button", { name: "พื้นที่เสี่ยง" }));
  expect(screen.getByRole("button", { name: "พื้นที่เสี่ยง" })).toHaveAttribute("aria-pressed", "false");
  await waitFor(() => expect(request).toHaveBeenCalledTimes(9));
  expect(request.mock.calls.slice(5).every(([url]) => !url.includes("/risk-areas?"))).toBe(true);
});

it("does not imply safety when every map layer fails", async () => {
  vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new Error("offline"))));
  render(<MemoryRouter><OverviewMapPreview /></MemoryRouter>);
  expect(await screen.findByRole("alert")).toHaveTextContent("สถานะไม่ทราบ");
  expect(screen.getByRole("button", { name: "ลองอีกครั้ง" })).toBeInTheDocument();
});

it("keeps the selected feature when identical bounds are republished", async () => {
  vi.stubGlobal("fetch", vi.fn((url: string) => Promise.resolve({ ok: true, json: () => Promise.resolve({ items: url.includes("/water-stations") && !url.includes("/readings") ? [{ id: "station-1", name: "สถานีตัวอย่าง", active: true, source: { id: "source-1", code: "sample", name: "หน่วยงานตัวอย่าง", sourceType: "OFFICIAL" }, location: { type: "Point", coordinates: [101, 14.1] } }] : [], limit: 100, offset: 0 }) })));
  render(<MemoryRouter><OverviewMapPreview /></MemoryRouter>);
  const selectButton = await screen.findByRole("button", { name: "เลือกสถานีตัวอย่าง" });
  await waitFor(() => expect(selectButton).toBeEnabled());
  fireEvent.click(selectButton);
  expect(screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).toHaveTextContent("สถานีตัวอย่าง");
  fireEvent.click(screen.getByRole("button", { name: "เผยแพร่ขอบเขตเดิมอีกครั้ง" }));
  expect(screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).toHaveTextContent("สถานีตัวอย่าง");
});
