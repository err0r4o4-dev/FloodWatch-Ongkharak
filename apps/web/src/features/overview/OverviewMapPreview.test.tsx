import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, expect, it, vi } from "vitest";
import { OverviewMapPreview } from "./OverviewMapPreview";

vi.mock("../map/components/SituationMap", async () => {
  const React = await import("react");
  return { SituationMap: ({ onBoundsChange }: { onBoundsChange: (bounds: { west: number; south: number; east: number; north: number }) => void }) => {
    React.useEffect(() => onBoundsChange({ west: 100.9, south: 14, east: 101.2, north: 14.3 }), [onBoundsChange]);
    return <div aria-label="แผนที่สถานการณ์แบบโต้ตอบ" />;
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
