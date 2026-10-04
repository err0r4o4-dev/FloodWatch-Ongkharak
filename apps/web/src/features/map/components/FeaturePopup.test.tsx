import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FeaturePopup } from "./FeaturePopup";
import type { MapSelection } from "../types";

const source = { id: "source", code: "test", name: "แหล่งทดสอบ", sourceType: "OFFICIAL" as const };
const station: MapSelection = { layer: "water", feature: { id: "station", name: "สถานีทดสอบ", source, active: true, location: { type: "Point", coordinates: [101, 14.1] } } };
const area: MapSelection = { layer: "reports", feature: {
  id: "area-1", name: "พื้นที่ชุมชนทดสอบ", location: { type: "Point", coordinates: [101.02, 14.1] }, radiusM: 150,
  sourceType: "COMMUNITY", depthBand: "MODERATE", previousDepthBand: "LOW", trend: "FALLING", reportedTrend: "RISING",
  freshness: "FRESH", summaryReason: "SUMMARIZED", latestObservedAt: "2026-10-02T01:00:00Z", computedAt: "2026-10-02T02:00:00Z",
  windowStart: "2026-10-01T02:00:00Z", windowEnd: "2026-10-02T02:00:00Z", reportCount: 3, sampleCount: 2, totalReportCount: 4, reportIds: ["report-1"]
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

  it("groups one stable area pin without marking the calculated summary as original content", () => {
    const onClose = vi.fn();
    const onDetails = vi.fn();
    const currentArea = { ...area.feature, latestObservedAt: new Date().toISOString(), computedAt: new Date().toISOString() };
    render(<FeaturePopup selection={{ layer: "reports", feature: currentArea }} onDetails={onDetails} onClose={onClose} />);
    expect(screen.getByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).toHaveTextContent("พื้นที่ชุมชนทดสอบ");
    expect(screen.getByText("ช่วงระดับน้ำปัจจุบัน")).toBeInTheDocument();
    expect(screen.getAllByText("10–น้อยกว่า 30 ซม.").length).toBeGreaterThan(0);
    expect(screen.getByText("ช่วงระดับน้ำก่อนหน้า")).toBeInTheDocument();
    expect(screen.getByText("น้อยกว่า 10 ซม.")).toBeInTheDocument();
    expect(screen.getByText("สรุปจากรายงานชุมชน")).toBeInTheDocument();
    expect(screen.queryByText("ปานกลาง")).not.toBeInTheDocument();
    expect(screen.getByText("แนวโน้มคำนวณ")).toBeInTheDocument();
    expect(screen.queryByText("เนื้อหาต้นฉบับ")).not.toBeInTheDocument();
    fireEvent.keyDown(screen.getByRole("region", { name: "ข้อมูลสรุปบนแผนที่" }), { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "ดูรายละเอียด" }));
    expect(onDetails).toHaveBeenCalledWith({ layer: "reports", feature: currentArea });
  });
});
