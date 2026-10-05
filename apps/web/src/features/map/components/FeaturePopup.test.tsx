import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FeaturePopup } from "./FeaturePopup";
import type { MapSelection } from "../types";
import type { CommunityReport } from "../../../features/reports/types";

const source = { id: "source", code: "test", name: "แหล่งทดสอบ", sourceType: "OFFICIAL" as const };
const station: MapSelection = { layer: "water", feature: { id: "station", name: "สถานีทดสอบ", source, active: true, location: { type: "Point", coordinates: [101, 14.1] } } };

function report(id: string, observedAt: string, description: string): CommunityReport {
  return {
    id, type: "FLOOD", description, locationName: "พื้นที่ชุมชนทดสอบ", locationNote: "ใกล้ตลาด", location: { type: "Point", coordinates: [101.02, 14.1] },
    estimatedWaterDepthM: 0.2, observedAt, waterTrend: "RISING", status: "VERIFIED", verificationStatus: "VERIFIED", publicationStatus: "PUBLISHED",
    sourceType: "COMMUNITY", createdAt: observedAt, updatedAt: observedAt, verifiedAt: observedAt, media: []
  };
}

const cluster: MapSelection = { layer: "reports", feature: {
  id: "report-group:report-1", name: "", location: { type: "Point", coordinates: [101.02, 14.1] }, sourceType: "COMMUNITY",
  reports: [report("report-1", "2026-10-02T01:00:00Z", "รายงานแรก"), report("report-2", "2026-10-02T02:00:00Z", "รายงานล่าสุด")]
} };

afterEach(() => vi.unstubAllGlobals());

describe("FeaturePopup", () => {
  it("loads a station reading with separate provenance and never infers safety", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [{ source: { ...source, sourceType: "COMMUNITY" }, observedAt: "2026-10-02T00:00:00Z", freshness: "STALE", verificationStatus: "VERIFIED", confidence: null, waterLevelM: 1.2, rainfallMm: null, flowRateM3s: null }] }) }));
    const onDetails = vi.fn();
    render(<FeaturePopup selection={station} onDetails={onDetails} onClose={() => undefined} />);
    expect(await screen.findByText(/ระดับน้ำ 1.2 ม./)).toBeInTheDocument();
    expect(screen.getByText("แหล่งทางการ")).toBeInTheDocument();
    expect(screen.getByText("รายงานชุมชน")).toBeInTheDocument();
    expect(screen.getByText("ข้อมูลเก่า")).toBeInTheDocument();
    expect(screen.getAllByText("ไม่ทราบสถานะ").length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole("button", { name: "ดูรายละเอียด" }));
    expect(onDetails).toHaveBeenCalledWith(station);
  });

  it("shows unavailable readings honestly and dismisses with Escape", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
    const onClose = vi.fn();
    render(<FeaturePopup selection={station} onDetails={() => undefined} onClose={onClose} />);
    expect(await screen.findByRole("alert")).toHaveTextContent("สถานะไม่ทราบ");
    fireEvent.keyDown(screen.getByRole("region", { name: "ข้อมูลสรุปบนแผนที่" }), { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });

  it("shows a grouped report count and note without area safety summaries", () => {
    const onClose = vi.fn();
    const onDetails = vi.fn();
    render(<FeaturePopup selection={cluster} onDetails={onDetails} onClose={onClose} />);
    expect(screen.getByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).toHaveTextContent("กลุ่มรายงานชุมชน");
    expect(screen.getByText("2 รายงานในบริเวณนี้")).toBeInTheDocument();
    expect(screen.getByText(/รวมหมุดตามตำแหน่งในระยะ 150 เมตร/)).toBeInTheDocument();
    expect(screen.queryByText("ช่วงระดับน้ำปัจจุบัน")).not.toBeInTheDocument();
    expect(screen.queryByText("แนวโน้มคำนวณ")).not.toBeInTheDocument();
    fireEvent.keyDown(screen.getByRole("region", { name: "ข้อมูลสรุปบนแผนที่" }), { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "ดูรายละเอียด" }));
    expect(onDetails).toHaveBeenCalledWith(cluster);
  });
});
