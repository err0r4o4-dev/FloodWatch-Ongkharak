import { act, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FM91NewsPanel } from "./FM91NewsPanel";
import { fetchFM91News, type FM91NewsResponse } from "./api";
import { FM91_NEWS_REFRESH_INTERVAL_MS, FM91_NEWS_REQUEST_TIMEOUT_MS } from "./useFM91News";

const realtime = vi.hoisted(() => ({ revision: 0 }));

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return { ...actual, fetchFM91News: vi.fn() };
});

vi.mock("../realtime/RealtimeProvider", () => ({
  useRealtimeRevision: () => realtime.revision
}));

const response = (overrides: Partial<FM91NewsResponse> = {}): FM91NewsResponse => ({
  retrievedAt: "2026-10-05T01:00:00Z",
  source: {
    code: "FM91_NEWS",
    name: "FM91",
    sourceType: "MEDIA",
    url: "https://fm91bkk.com/news",
    enabled: true,
    status: "AVAILABLE",
    freshness: "FRESH",
    lastSuccessAt: "2026-10-05T01:00:00Z",
    errorCode: null
  },
  items: [{
    id: "news-1", articleId: "123", title: "ข่าวต้นฉบับ FM91", description: "รายละเอียดข่าวจากสื่อ", url: "https://fm91bkk.com/newsarticle/123", category: "ข่าว", publishedAt: "2026-10-05T00:00:00Z", observedAt: null, retrievedAt: "2026-10-05T01:00:00Z", updatedAt: "2026-10-05T01:00:00Z", historyTotal: 1, provenance: "MEDIA", status: "PUBLISHED", freshness: "FRESH", reason: null, imageURLs: [], locations: ["องครักษ์"], roads: [], eventTimes: [], waterLevels: [], travelAdvice: [], matchedKeywords: [], latitude: null, longitude: null
  }],
  limit: 20,
  offset: 0,
  total: 1,
  ...overrides
});

beforeEach(() => {
  vi.mocked(fetchFM91News).mockReset();
  realtime.revision = 0;
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("FM91NewsPanel", () => {
  it("keeps automatically published FM91 media separate from official alerts and preserves literal text", async () => {
    vi.mocked(fetchFM91News).mockResolvedValue(response());

    render(<FM91NewsPanel />);

    const panel = await screen.findByRole("region", { name: "ข่าว FM91" });
    expect(panel).toHaveTextContent("แหล่งสื่อ FM91");
    expect(panel).toHaveTextContent("ข่าวต้นฉบับ FM91");
    expect(panel).toHaveTextContent("เผยแพร่อัตโนมัติ");
    expect(panel).toHaveTextContent("สื่อ ไม่ใช่ประกาศทางการ");
    expect(panel.querySelector("a[href='https://fm91bkk.com/newsarticle/123']")).toBeTruthy();
    expect(panel.querySelector("script")).toBeNull();
    expect(panel).not.toHaveTextContent("ปลอดภัย");
  });

  it("shows unknown values for nullable fields and does not fabricate a map pin", async () => {
    vi.mocked(fetchFM91News).mockResolvedValue(response({
      items: [{ ...response().items[0], description: "", publishedAt: null, observedAt: null, locations: [], latitude: null, longitude: null }]
    }));

    render(<FM91NewsPanel />);

    const panel = await screen.findByRole("region", { name: "ข่าว FM91" });
    expect(panel).toHaveTextContent("ไม่ทราบข้อมูล");
    expect(panel).toHaveTextContent("ยังไม่มีพิกัดข่าว");
    expect(within(panel).queryByTestId("fm91-news-map-marker")).not.toBeInTheDocument();
  });

  it("does not present stale or unavailable data as current-looking news", async () => {
    vi.mocked(fetchFM91News).mockResolvedValue(response({
      source: { ...response().source, status: "DEGRADED", freshness: "STALE" },
      items: [{ ...response().items[0], freshness: "STALE", reason: "SOURCE_EXPIRED" }]
    }));

    render(<FM91NewsPanel />);

    const panel = await screen.findByRole("region", { name: "ข่าว FM91" });
    expect(panel).toHaveTextContent("ข้อมูลเก่า");
    expect(panel).toHaveTextContent(/ไม่ทราบความสดใหม่/);
    expect(panel).toHaveTextContent("เหตุผลสถานะข้อมูล: SOURCE_EXPIRED");
    expect(panel).toHaveTextContent("ข่าวต้นฉบับ FM91");
  });

  it("hides cards when the media source is unavailable", async () => {
    vi.mocked(fetchFM91News).mockResolvedValue(response({ source: { ...response().source, status: "UNAVAILABLE", freshness: "UNKNOWN" }, items: [] }));
    render(<FM91NewsPanel />);
    expect(await screen.findByText("ข่าว FM91 ไม่พร้อมใช้งาน")).toBeInTheDocument();
    expect(screen.queryByText("ข่าวต้นฉบับ FM91")).not.toBeInTheDocument();
  });

  it("clears current-looking cards while a slow request is unresolved", async () => {
    vi.useFakeTimers();
    vi.mocked(fetchFM91News).mockResolvedValueOnce(response()).mockImplementationOnce(() => new Promise(() => {}));
    render(<FM91NewsPanel />);
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(screen.getByText("ข่าวต้นฉบับ FM91")).toBeInTheDocument();

    await act(async () => { vi.advanceTimersByTime(FM91_NEWS_REFRESH_INTERVAL_MS); });
    await act(async () => { await Promise.resolve(); });
    expect(screen.queryByText("ข่าวต้นฉบับ FM91")).not.toBeInTheDocument();
    await act(async () => { vi.advanceTimersByTime(FM91_NEWS_REQUEST_TIMEOUT_MS); });
    expect(screen.getByText("ข่าว FM91 ไม่พร้อมใช้งาน")).toBeInTheDocument();
  });

  it("refreshes when the external data realtime revision changes", async () => {
    vi.mocked(fetchFM91News).mockResolvedValueOnce(response()).mockResolvedValueOnce(response({ items: [{ ...response().items[0], id: "news-2", title: "ข่าวใหม่จาก FM91" }] }));
    const view = render(<FM91NewsPanel />);
    expect(await screen.findByText("ข่าวต้นฉบับ FM91")).toBeInTheDocument();

    realtime.revision = 1;
    view.rerender(<FM91NewsPanel />);
    await waitFor(() => expect(screen.getByText("ข่าวใหม่จาก FM91")).toBeInTheDocument());
  });
});
