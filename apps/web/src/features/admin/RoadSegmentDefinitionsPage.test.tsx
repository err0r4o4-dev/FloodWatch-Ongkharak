import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RoadSegmentDefinitionsPage } from "./RoadSegmentDefinitionsPage";

const roadApi = vi.hoisted(() => ({
  fetchRoadSegments: vi.fn(),
  listRoadOptionPage: vi.fn(),
  roadSegmentsApi: { create: vi.fn() }
}));

vi.mock("../road-segments/api", () => roadApi);
vi.mock("../../shared/alerts", () => ({ notifyError: vi.fn(), notifySuccess: vi.fn() }));

const road = { id: "road-1", name: "ถนนนำเข้า", code: "OSM-WAY-1" };
const segment = {
  id: "segment-1", roadId: road.id, name: "ช่วงสะพาน", startLabel: "101.000,14.000", endLabel: "101.010,14.010",
  geometry: { type: "LineString" as const, coordinates: [[101, 14], [101.01, 14.01]] as [number, number][] },
  location: { type: "Point" as const, coordinates: [101.005, 14.005] as [number, number] }, version: 2, supersedesId: null, active: true, createdAt: "2026-10-02T00:00:00Z"
};

function renderPage() {
  return render(<MemoryRouter><RoadSegmentDefinitionsPage /></MemoryRouter>);
}

function setupData() {
  roadApi.listRoadOptionPage.mockResolvedValue({ items: [road], limit: 100, offset: 0, hasMore: false });
  roadApi.fetchRoadSegments.mockResolvedValue({ items: [segment], limit: 100, offset: 0 });
  roadApi.roadSegmentsApi.create.mockResolvedValue(undefined);
}

describe("RoadSegmentDefinitionsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setupData();
  });

  it("loads more roads and more segments explicitly", async () => {
    roadApi.listRoadOptionPage
      .mockResolvedValueOnce({ items: [road], limit: 1, offset: 0, hasMore: true })
      .mockResolvedValueOnce({ items: [{ id: "road-2", name: "Road two", code: "OSM-WAY-2" }], limit: 1, offset: 1, hasMore: false });
    roadApi.fetchRoadSegments
      .mockResolvedValueOnce({ items: [segment], limit: 1, offset: 0 })
      .mockResolvedValueOnce({ items: [{ ...segment, id: "segment-2", name: "ช่วงตลาด" }], limit: 1, offset: 1 });

    renderPage();
    expect(await screen.findByRole("option", { name: /OSM-WAY-1/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "ผู้มีส่วนร่วม OpenStreetMap" })).toHaveAttribute("href", "https://www.openstreetmap.org/copyright");
    fireEvent.click(screen.getByRole("button", { name: "โหลดถนนเพิ่ม" }));
    expect(await screen.findByRole("option", { name: /OSM-WAY-2/ })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("ค้นหาถนนที่โหลดแล้ว"), { target: { value: "Road two" } });
    expect(screen.getByRole("option", { name: /OSM-WAY-2/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "โหลดช่วงถนนเพิ่ม" }));
    expect(await screen.findByText("ช่วงตลาด")).toBeInTheDocument();
    expect(roadApi.listRoadOptionPage).toHaveBeenLastCalledWith(100, 1);
    expect(roadApi.fetchRoadSegments).toHaveBeenLastCalledWith(null, 100, 1);
  });

  it("prefills a replacement from an existing segment and submits its version link", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /สร้างรุ่นทดแทนสำหรับ/ }));

    expect(screen.getByRole("heading", { name: "สร้างช่วงถนนรุ่นทดแทน" })).toBeInTheDocument();
    expect(screen.getByLabelText("ชื่อช่วงถนน")).toHaveValue(segment.name);
    expect(screen.getByLabelText("ป้ายต้นช่วง")).toHaveValue(segment.startLabel);
    expect(screen.getByLabelText("ป้ายปลายช่วง")).toHaveValue(segment.endLabel);
    expect(screen.getByLabelText("GeoJSON LineString")).toHaveValue(JSON.stringify(segment.geometry));
    expect(screen.getByText(/คงรายงานเดิมไว้/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "สร้างช่วงทดแทน" }));
    await waitFor(() => expect(roadApi.roadSegmentsApi.create).toHaveBeenCalledWith({
      roadId: segment.roadId,
      name: segment.name,
      startLabel: segment.startLabel,
      endLabel: segment.endLabel,
      geometry: segment.geometry,
      supersedesId: segment.id
    }));
  });

  it("distinguishes missing fields from out-of-range geometry", async () => {
    renderPage();
    await screen.findByRole("option", { name: /OSM-WAY-1/ });
    fireEvent.click(screen.getByRole("button", { name: "สร้างช่วงถนน" }));
    expect(screen.getByRole("alert")).toHaveTextContent("เลือกถนนแม่");
    expect(roadApi.roadSegmentsApi.create).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("ถนนแม่"), { target: { value: road.id } });
    fireEvent.change(screen.getByLabelText("ชื่อช่วงถนน"), { target: { value: "ช่วงผิดพิกัด" } });
    fireEvent.change(screen.getByLabelText("ป้ายต้นช่วง"), { target: { value: "A" } });
    fireEvent.change(screen.getByLabelText("ป้ายปลายช่วง"), { target: { value: "B" } });
    fireEvent.change(screen.getByLabelText("GeoJSON LineString"), { target: { value: '{"type":"LineString","coordinates":[[181,14],[101,14]]}' } });
    fireEvent.click(screen.getByRole("button", { name: "สร้างช่วงถนน" }));
    expect(screen.getByRole("alert")).toHaveTextContent("ลองจิจูด");
    expect(roadApi.roadSegmentsApi.create).not.toHaveBeenCalled();
  });
});
