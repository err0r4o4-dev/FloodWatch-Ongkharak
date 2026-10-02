import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RiskPage } from "./RiskPage";

vi.mock("../map/components/SituationMap", async () => {
  const React = await import("react");
  return { SituationMap: ({ onBoundsChange }: { onBoundsChange: (bounds: { west: number; south: number; east: number; north: number }) => void }) => {
    React.useEffect(() => onBoundsChange({ west: 100.9, south: 14, east: 101.2, north: 14.3 }), [onBoundsChange]);
    return <div aria-label="แผนที่สถานการณ์แบบโต้ตอบ" />;
  } };
});

const source = (sourceType: "OFFICIAL" | "COMMUNITY" | "AI") => ({
  id: `${sourceType.toLowerCase()}-source`, name: `แหล่ง${sourceType}`, code: sourceType.toLowerCase(), sourceType
});

const risk = (overrides: Record<string, unknown> = {}) => ({
  id: "risk-1", name: "พื้นที่ทดสอบ", description: "ขอบเขตประเมินจากข้อมูลต้นทาง",
  source: source("OFFICIAL"), riskLevel: "HIGH", effectiveRiskLevel: "HIGH", freshness: "FRESH",
  validFrom: "2026-10-02T00:00:00Z", validTo: "2026-10-03T00:00:00Z", ...overrides
});

function mockRisks(items: unknown[]) {
  const request = vi.fn((_url: string) => Promise.resolve({ ok: true, json: () => Promise.resolve({ items, limit: 20, offset: 0 }) }));
  vi.stubGlobal("fetch", request);
  return request;
}

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("RiskPage", () => {
  it("loads published risk areas and filters by their effective level", async () => {
    const request = mockRisks([risk(), risk({ id: "risk-2", name: "พื้นที่ปานกลาง", source: source("COMMUNITY"), riskLevel: "MODERATE", effectiveRiskLevel: "MODERATE" }), risk({ id: "risk-3", name: "พื้นที่เสี่ยงสูงมาก", riskLevel: "VERY_HIGH", effectiveRiskLevel: "VERY_HIGH" })]);

    render(<RiskPage />);

    expect(screen.getByRole("status", { name: "กำลังโหลดข้อมูล" })).toBeInTheDocument();
    expect(await screen.findByText("พื้นที่ทดสอบ")).toBeInTheDocument();
    expect(screen.getAllByText("แหล่งทางการ")).toHaveLength(2);
    expect(screen.getByText("รายงานชุมชน")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "เปิดแผนที่พื้นที่เสี่ยง" })).toHaveAttribute("href", "/map");
    expect(screen.getByLabelText("แผนที่สถานการณ์แบบโต้ตอบ")).toBeInTheDocument();
    await waitFor(() => expect(request.mock.calls.some(([url]) => url.includes("/risk-areas?") && url.includes("bbox="))).toBe(true));
    fireEvent.click(screen.getByRole("button", { name: "ความเสี่ยงสูง" }));
    expect(screen.getByText("พื้นที่ทดสอบ")).toBeInTheDocument();
    expect(screen.queryByText("พื้นที่ปานกลาง")).not.toBeInTheDocument();
    expect(screen.queryByText("พื้นที่เสี่ยงสูงมาก")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ความเสี่ยงสูงมาก" }));
    expect(screen.getByText("พื้นที่เสี่ยงสูงมาก")).toBeInTheDocument();
    expect(screen.queryByText("พื้นที่ทดสอบ")).not.toBeInTheDocument();
    expect(request).toHaveBeenCalledWith("/api/v1/risk-areas?limit=20", expect.objectContaining({ signal: expect.any(AbortSignal) }));
  });

  it("keeps stale evidence unknown rather than presenting its reported level", async () => {
    mockRisks([risk({ freshness: "STALE", effectiveRiskLevel: "HIGH", source: source("AI") })]);

    render(<RiskPage />);

    expect(await screen.findByText("พื้นที่ทดสอบ")).toBeInTheDocument();
    expect(screen.getByText("หลักฐาน AI")).toBeInTheDocument();
    expect(screen.getByText("ข้อมูลเก่า")).toBeInTheDocument();
    expect(screen.getAllByText("ไม่ทราบสถานะ").length).toBeGreaterThan(1);
    expect(screen.getAllByText(/ข้อมูลเก่าหรือไม่พร้อมใช้งาน/).length).toBeGreaterThan(1);
    expect(screen.queryByText("ปลอดภัย")).not.toBeInTheDocument();
  });

  it("separates an empty collection from a loading failure", async () => {
    mockRisks([]);
    const view = render(<RiskPage />);
    expect(await screen.findByText("ยังไม่มีพื้นที่เสี่ยงที่เผยแพร่")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "เปิดแผนที่พื้นที่เสี่ยง" })).toHaveAttribute("href", "/map");
    expect(within(screen.getByRole("region", { name: "สรุประดับความเสี่ยงจากรายการที่เรียกดู" })).getAllByText("—")).toHaveLength(5);

    view.unmount();
    vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new Error("offline"))));
    render(<RiskPage />);
    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดข้อมูลสถานการณ์ได้");
    expect(screen.queryByText("ยังไม่มีพื้นที่เสี่ยงที่เผยแพร่")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "เปิดแผนที่พื้นที่เสี่ยง" })).toHaveAttribute("href", "/map");
    expect(within(screen.getByRole("region", { name: "สรุประดับความเสี่ยงจากรายการที่เรียกดู" })).getAllByText("—")).toHaveLength(5);
  });
});
