import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { OfficialAnnouncementsPanel } from "./OfficialAnnouncementsPanel";
import { fetchOfficialAnnouncements, type OfficialAnnouncementsResponse } from "./api";
import { OFFICIAL_ANNOUNCEMENTS_REFRESH_INTERVAL_MS, OFFICIAL_ANNOUNCEMENTS_REQUEST_TIMEOUT_MS } from "./useOfficialAnnouncements";
import { RealtimeProvider } from "../realtime/RealtimeProvider";

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return { ...actual, fetchOfficialAnnouncements: vi.fn() };
});

const response: OfficialAnnouncementsResponse = {
  retrievedAt: "2026-10-04T01:00:00Z",
  source: {
    code: "TMD_CAP",
    name: "Thai Meteorological Department",
    sourceType: "OFFICIAL" as const,
    url: "https://www.tmd.go.th",
    enabled: true,
    status: "AVAILABLE" as const,
    freshness: "FRESH" as const,
    lastSuccessAt: "2026-10-04T00:59:00Z"
  },
  items: [{
    id: "123e4567-e89b-12d3-a456-426614174000",
    identifier: "tmd-cap-1",
    headline: "ประกาศทดสอบ",
    description: "รายละเอียดประกาศทดสอบ",
    status: "UPDATED" as const,
    effectiveStatus: "UNKNOWN" as const,
    freshness: "FRESH" as const,
    issuedAt: "2026-10-04T00:00:00Z",
    expiresAt: null,
    areaCodes: ["TH"],
    reason: "GEOGRAPHIC_SCOPE_UNCONFIRMED",
    historyTotal: 1,
    history: [{
      evidenceId: "123e4567-e89b-12d3-a456-426614174001",
      identifier: "tmd-cap-1",
      messageType: "Update" as const,
      issuedAt: "2026-10-04T00:00:00Z",
      retrievedAt: "2026-10-04T00:59:00Z",
      expiresAt: "2026-10-04T03:00:00Z",
      headline: "ประกาศทดสอบ",
      description: "รายละเอียดประวัติทดสอบ",
      areaCodes: ["TH"],
      reason: null
    }]
  }],
  limit: 20,
  offset: 0,
  total: 1
};

class FakeEventSource {
  static connections: FakeEventSource[] = [];
  readonly listeners = new Map<string, (event: Event) => void>();

  constructor(readonly url: string) {
    FakeEventSource.connections.push(this);
  }

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.set(type, listener as (event: Event) => void);
  }

  close() {}

  send(type: string, data: unknown) {
    this.listeners.get(type)?.(new MessageEvent(type, { data: JSON.stringify(data) }));
  }
}

beforeEach(() => {
  vi.mocked(fetchOfficialAnnouncements).mockReset();
  vi.stubGlobal("fetch", vi.fn());
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  FakeEventSource.connections = [];
});

describe("OfficialAnnouncementsPanel", () => {
  it("shows UNKNOWN rather than a current warning and expands retained history", async () => {
    vi.mocked(fetchOfficialAnnouncements).mockResolvedValue(response);
    render(<OfficialAnnouncementsPanel />);

    const panel = await screen.findByRole("region", { name: "ประกาศทางการจากกรมอุตุนิยมวิทยา" });
    expect(panel).toHaveTextContent("ไม่ทราบสถานะ");
    expect(panel).toHaveTextContent("ยังไม่ยืนยันขอบเขตพื้นที่");
    expect(panel).not.toHaveTextContent("GEOGRAPHIC_SCOPE_UNCONFIRMED");
    expect(panel).not.toHaveTextContent("เตือนภัย");

    const history = screen.getByText(/ประวัติหลักฐาน/).closest("details");
    expect(history).not.toHaveAttribute("open");
    fireEvent.click(screen.getByText(/ประวัติหลักฐาน/));
    expect(history).toHaveAttribute("open");
    expect(panel).toHaveTextContent("Update");
    expect(panel).toHaveTextContent("รายละเอียดประวัติทดสอบ");
    expect(panel).toHaveTextContent("รับหลักฐานเมื่อ");
    expect(panel).toHaveTextContent("หมดอายุเมื่อ");
    expect(panel).toHaveTextContent("พื้นที่: TH");
    expect(panel.querySelector("a[href='https://www.tmd.go.th']")).toBeTruthy();
  });

  it("localizes known backend reason codes", async () => {
    vi.mocked(fetchOfficialAnnouncements).mockResolvedValue({
      ...response,
      items: ["SOURCE_UNAVAILABLE", "MISSING_EXPIRY", "NOT_YET_EFFECTIVE"].map((reason, index) => ({ ...response.items[0], id: `123e4567-e89b-12d3-a456-42661417400${index + 2}`, reason })),
      total: 3
    });
    render(<OfficialAnnouncementsPanel />);
    const panel = await screen.findByRole("region", { name: "ประกาศทางการจากกรมอุตุนิยมวิทยา" });
    expect(panel).toHaveTextContent("แหล่งข้อมูลต้นทางไม่พร้อมใช้งาน");
    expect(panel).toHaveTextContent("ไม่มีเวลาหมดอายุของหลักฐาน");
    expect(panel).toHaveTextContent("หลักฐานยังไม่ถึงเวลาเริ่มมีผล");
    expect(panel).not.toHaveTextContent("SOURCE_UNAVAILABLE");
    expect(panel).not.toHaveTextContent("MISSING_EXPIRY");
    expect(panel).not.toHaveTextContent("NOT_YET_EFFECTIVE");
  });

  it("keeps non-fresh active evidence UNKNOWN while preserving terminal statuses", async () => {
    vi.mocked(fetchOfficialAnnouncements).mockResolvedValue({
      ...response,
      items: [
        { ...response.items[0], id: "123e4567-e89b-12d3-a456-426614174002", status: "ACTIVE", effectiveStatus: "ACTIVE", freshness: "STALE" },
        { ...response.items[0], id: "123e4567-e89b-12d3-a456-426614174003", status: "CANCELLED", effectiveStatus: "CANCELLED" },
        { ...response.items[0], id: "123e4567-e89b-12d3-a456-426614174004", status: "EXPIRED", effectiveStatus: "EXPIRED" }
      ],
      total: 3
    });
    render(<OfficialAnnouncementsPanel />);
    const panel = await screen.findByRole("region", { name: "ประกาศทางการจากกรมอุตุนิยมวิทยา" });
    expect(panel).toHaveTextContent("ไม่ทราบสถานะ");
    expect(panel).toHaveTextContent("ยกเลิกแล้ว");
    expect(panel).toHaveTextContent("หมดอายุแล้ว");
    expect(panel).not.toHaveTextContent("ยังมีผล");
    expect(panel.querySelector(".official-announcements__status--cancelled")).toBeTruthy();
    expect(panel.querySelector(".official-announcements__status--expired")).toBeTruthy();
  });

  it("renders disabled, empty, stale and outage states without claiming safety", async () => {
    vi.mocked(fetchOfficialAnnouncements).mockResolvedValue({ ...response, items: [], total: 0, source: { ...response.source, enabled: false, status: "NOT_CONFIGURED" } });
    const firstView = render(<OfficialAnnouncementsPanel />);
    expect(await screen.findByText("ยังไม่ได้ตั้งค่าประกาศทางการ")).toBeInTheDocument();
    firstView.unmount();

    vi.mocked(fetchOfficialAnnouncements).mockResolvedValue({ ...response, items: [], total: 0 });
    const secondView = render(<OfficialAnnouncementsPanel />);
    await waitFor(() => expect(screen.getByText("ไม่พบหลักฐานประกาศที่ตรงกัน")).toBeInTheDocument());
    expect(screen.getByText(/ไม่ได้หมายความว่าพื้นที่ปลอดภัย/)).toBeInTheDocument();
    secondView.unmount();

    vi.mocked(fetchOfficialAnnouncements).mockRejectedValue(new Error("offline"));
    render(<OfficialAnnouncementsPanel />);
    await waitFor(() => expect(screen.getByText("ประกาศทางการไม่พร้อมใช้งาน")).toBeInTheDocument());
    expect(screen.queryByText("ประกาศทดสอบ")).not.toBeInTheDocument();
  });

  it("registers visible-tab polling, aborts on unmount, and times out stalled requests", () => {
    vi.useFakeTimers();
    let signal: AbortSignal | undefined;
    vi.mocked(fetchOfficialAnnouncements).mockImplementation((requestSignal) => {
      signal = requestSignal;
      return new Promise<OfficialAnnouncementsResponse>(() => undefined);
    });
    const setInterval = vi.spyOn(window, "setInterval");
    const clearInterval = vi.spyOn(window, "clearInterval");
    const view = render(<OfficialAnnouncementsPanel />);

    expect(setInterval).toHaveBeenCalledWith(expect.any(Function), OFFICIAL_ANNOUNCEMENTS_REFRESH_INTERVAL_MS);
    act(() => vi.advanceTimersByTime(OFFICIAL_ANNOUNCEMENTS_REQUEST_TIMEOUT_MS));
    expect(screen.getByText("ประกาศทางการไม่พร้อมใช้งาน")).toBeInTheDocument();
    view.unmount();
    expect(signal?.aborted).toBe(true);
    expect(clearInterval).toHaveBeenCalled();
  });

  it("refreshes when the realtime external-data event arrives", async () => {
    vi.stubGlobal("EventSource", FakeEventSource);
    vi.mocked(fetchOfficialAnnouncements).mockResolvedValue(response);
    render(<RealtimeProvider><OfficialAnnouncementsPanel /></RealtimeProvider>);
    await screen.findByRole("heading", { level: 3, name: "ประกาศทดสอบ" });
    expect(fetchOfficialAnnouncements).toHaveBeenCalledTimes(1);

    act(() => FakeEventSource.connections[0].send("external_data.updated", {
      eventType: "external_data.updated",
      eventId: "external-announcement-1",
      resourceId: "TMD_CAP",
      resourceType: "EXTERNAL_DATA",
      payloadVersion: 1,
      sourceType: "OFFICIAL"
    }));
    await waitFor(() => expect(fetchOfficialAnnouncements).toHaveBeenCalledTimes(2));
  });
});
