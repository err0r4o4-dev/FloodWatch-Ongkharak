import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MapDataStatus } from "../components/MapDataStatus";
import { MapSidePanel } from "../components/MapSidePanel";
import { RiskDirectory } from "../components/RiskDirectory";
import { useMapData } from "./useMapData";
import type { LayerItems, LayerKey, LayerVisibility, MapBounds } from "../types";

const fetchMapLayerMock = vi.hoisted(() => vi.fn());
vi.mock("../api", () => ({ fetchMapLayer: fetchMapLayerMock }));

type Deferred<T> = { promise: Promise<T>; resolve: (value: T) => void; reject: (reason?: unknown) => void };

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

const allVisible: LayerVisibility = { flood: true, water: true, risk: true, roads: true, reports: true };
const boundsOne: MapBounds = { west: 100, south: 14, east: 101, north: 15 };
const boundsTwo: MapBounds = { west: 102, south: 14, east: 103, north: 15 };

function layerResult(id: string, hasMore = false) {
  return { items: [{ id }] as LayerItems[LayerKey], hasMore };
}

function Probe({ bounds, visible = allVisible }: { bounds: MapBounds; visible?: LayerVisibility }) {
  const data = useMapData(bounds, visible, 0);
  return <output data-testid="snapshot">{JSON.stringify({
    counts: Object.fromEntries(Object.entries(data.items).map(([key, items]) => [key, items.length])),
    failed: data.failed,
    ids: Object.fromEntries(Object.entries(data.items).map(([key, items]) => [key, items.map((item) => item.id)])),
    loading: data.loading,
    previous: data.previousLayers,
    truncated: data.truncated
  })}</output>;
}

function snapshot() {
  return JSON.parse(screen.getByTestId("snapshot").textContent ?? "{}") as {
    counts: Record<LayerKey, number>;
    failed: LayerKey[];
    ids: Record<LayerKey, string[]>;
    loading: boolean;
    previous: LayerKey[];
    truncated: LayerKey[];
  };
}

async function startRequests() {
  await act(async () => {
    vi.advanceTimersByTime(250);
    await Promise.resolve();
  });
}

async function settle() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
});

describe("useMapData", () => {
  beforeEach(() => vi.useFakeTimers());

  it("publishes each layer as it fulfills and keeps previous results while updating", async () => {
    const requests: { key: LayerKey; bounds: MapBounds; deferred: Deferred<ReturnType<typeof layerResult>> }[] = [];
    fetchMapLayerMock.mockImplementation((key: LayerKey, bounds: MapBounds) => {
      const pending = deferred<ReturnType<typeof layerResult>>();
      requests.push({ key, bounds, deferred: pending });
      return pending.promise;
    });
    const view = render(<Probe bounds={boundsOne} />);

    await startRequests();
    expect(requests).toHaveLength(5);
    requests.find((request) => request.key === "flood" && request.bounds === boundsOne)?.deferred.resolve(layerResult("old-flood", true));
    await settle();
    expect(snapshot().counts.flood).toBe(1);
    expect(snapshot().loading).toBe(true);

    view.rerender(<Probe bounds={boundsTwo} />);
    expect(snapshot().counts.flood).toBe(1);
    expect(snapshot().previous).toContain("flood");
    expect(snapshot().truncated).toContain("flood");
    expect(snapshot().loading).toBe(true);

    await startRequests();
    const currentFlood = requests.find((request) => request.key === "flood" && request.bounds === boundsTwo);
    currentFlood?.deferred.resolve(layerResult("new-flood"));
    await settle();
    expect(snapshot().counts.flood).toBe(1);
    expect(snapshot().previous).not.toContain("flood");
    expect(snapshot().truncated).not.toContain("flood");

    requests.find((request) => request.key === "water" && request.bounds === boundsOne)?.deferred.resolve(layerResult("late-old-water"));
    await settle();
    expect(snapshot().ids.flood).toEqual(["new-flood"]);
    expect(snapshot().ids.water).toEqual([]);
  });

  it("does not retain hidden data or failed-layer evidence for the current viewport", async () => {
    const requests: { key: LayerKey; bounds: MapBounds; deferred: Deferred<ReturnType<typeof layerResult>> }[] = [];
    fetchMapLayerMock.mockImplementation((key: LayerKey, bounds: MapBounds) => {
      const pending = deferred<ReturnType<typeof layerResult>>();
      requests.push({ key, bounds, deferred: pending });
      return pending.promise;
    });
    const view = render(<Probe bounds={boundsOne} />);
    await startRequests();
    requests.find((request) => request.key === "flood" && request.bounds === boundsOne)?.deferred.resolve(layerResult("old-flood"));
    await settle();

    const floodHidden: LayerVisibility = { ...allVisible, flood: false };
    view.rerender(<Probe bounds={boundsTwo} visible={floodHidden} />);
    expect(snapshot().counts.flood).toBe(0);
    await startRequests();
    expect(requests.filter((request) => request.bounds === boundsTwo).map((request) => request.key)).not.toContain("flood");

    view.rerender(<Probe bounds={boundsOne} />);
    await startRequests();
    const currentFloodRequests = requests.filter((request) => request.key === "flood" && request.bounds === boundsOne);
    currentFloodRequests[currentFloodRequests.length - 1]?.deferred.reject(new Error("offline"));
    await settle();
    expect(snapshot().counts.flood).toBe(0);
    expect(snapshot().failed).toContain("flood");
  });

  it("keeps previous layer errors during refresh until that layer settles", async () => {
    const requests: { key: LayerKey; bounds: MapBounds; deferred: Deferred<ReturnType<typeof layerResult>> }[] = [];
    fetchMapLayerMock.mockImplementation((key: LayerKey, bounds: MapBounds) => {
      const pending = deferred<ReturnType<typeof layerResult>>();
      requests.push({ key, bounds, deferred: pending });
      return pending.promise;
    });
    const view = render(<Probe bounds={boundsOne} />);
    await startRequests();
    requests.find((request) => request.key === "water" && request.bounds === boundsOne)?.deferred.reject(new Error("offline"));
    await settle();
    expect(snapshot().failed).toContain("water");

    view.rerender(<Probe bounds={boundsTwo} />);
    expect(snapshot().failed).toContain("water");
    await startRequests();
    const currentWater = requests.filter((request) => request.key === "water" && request.bounds === boundsTwo).at(-1);
    currentWater?.deferred.resolve(layerResult("new-water"));
    await settle();
    expect(snapshot().failed).not.toContain("water");
  });

  it("polls the visible viewport so stale area summaries can expire without a new event", async () => {
    fetchMapLayerMock.mockResolvedValue({ items: [], hasMore: false });
    render(<Probe bounds={boundsOne} />);
    await startRequests();
    await settle();
    const initialRequests = fetchMapLayerMock.mock.calls.length;
    await act(async () => {
      vi.advanceTimersByTime(60_000);
      await Promise.resolve();
    });
    await act(async () => {
      vi.advanceTimersByTime(250);
      await Promise.resolve();
    });
    await settle();
    expect(fetchMapLayerMock.mock.calls.length).toBeGreaterThan(initialRequests);
  });

  it("labels retained items as previous results while updating", () => {
    const items = { flood: [], water: [], risk: [], roads: [], reports: [] } as LayerItems;
    render(<MapDataStatus items={items} visible={allVisible} loading failed={false} failedLayers={[]} previousLayers={["flood"]} truncated={[]} onRetry={vi.fn()} />);
    expect(screen.getByText(/ข้อมูลก่อนหน้า/)).toBeInTheDocument();
    expect(screen.getByText("กำลังอัปเดตข้อมูลตามขอบเขตปัจจุบัน")).toBeInTheDocument();
  });

  it("keeps progressive list results visible while the map is updating", () => {
    const items = { flood: [], water: [{ id: "old-water", name: "Old water" }], risk: [], roads: [], reports: [] } as unknown as LayerItems;
    render(<MapSidePanel mode="SITUATION" items={items} visible={allVisible} loading failed={false} failedLayers={[]} previousLayers={["water"]} truncated={[]} onRetry={vi.fn()} selection={null} onSelect={vi.fn()} onClose={vi.fn()} points={{ origin: { longitude: "", latitude: "" }, destination: { longitude: "", latitude: "" } }} onPointChange={vi.fn()} picking={null} onPickingChange={vi.fn()} />);
    expect(screen.getByRole("button", { name: /Old water/ })).toBeInTheDocument();
    expect(screen.getByText(/ข้อมูลก่อนหน้า/)).toBeInTheDocument();
  });

  it("keeps risk filters usable when risk results are available during another layer update", () => {
    const risk = {
      id: "risk-progressive",
      name: "Progressive risk",
      description: "Risk result",
      geometry: { type: "MultiPolygon", coordinates: [[[[100, 14], [100.1, 14], [100.1, 14.1], [100, 14]]]] },
      source: { id: "source-1", code: "sample", name: "Test source", sourceType: "OFFICIAL" },
      riskLevel: "HIGH",
      effectiveRiskLevel: "HIGH",
      freshness: "FRESH",
      validFrom: null,
      validTo: null
    } as unknown as import("../types").RiskFeature;
    render(<RiskDirectory items={[risk]} loading failed={false} previousLayers={["risk"]} truncated={false} selectedId={null} onSelect={vi.fn()} onRetry={vi.fn()} />);
    expect(screen.getByRole("button", { name: "กรอง: ทั้งหมด" })).not.toBeDisabled();
    expect(screen.getByText(/ข้อมูลก่อนหน้า/)).toBeInTheDocument();
  });
});
