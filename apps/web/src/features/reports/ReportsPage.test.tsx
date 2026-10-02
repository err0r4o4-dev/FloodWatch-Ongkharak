import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../auth/AuthProvider";
import { RealtimeProvider } from "../realtime/RealtimeProvider";
import { reportsApi } from "./api";
import { ReportsPage } from "./ReportsPage";

vi.mock("./ReportLocationPicker", async () => {
  const React = await import("react");
  return {
    ReportLocationPicker: ({ onChange }: { onChange: (location: { type: "Point"; coordinates: [number, number] } | null) => void }) => {
      const [longitude, setLongitude] = React.useState("");
      const [latitude, setLatitude] = React.useState("");
      const change = (nextLongitude: string, nextLatitude: string) => {
        setLongitude(nextLongitude);
        setLatitude(nextLatitude);
        const lng = Number(nextLongitude);
        const lat = Number(nextLatitude);
        onChange(nextLongitude !== "" && nextLatitude !== "" && Number.isFinite(lng) && Number.isFinite(lat) ? { type: "Point", coordinates: [lng, lat] } : null);
      };
      return <><label>ลองจิจูด<input value={longitude} onChange={(event) => change(event.target.value, latitude)} /></label><label>ละติจูด<input value={latitude} onChange={(event) => change(longitude, event.target.value)} /></label></>;
    }
  };
});

const verifiedReport = {
  id: "report-1",
  userId: "user-1",
  type: "FLOOD",
  description: "น้ำท่วมขังหน้าตลาด",
  location: { type: "Point", coordinates: [101.1, 14.2] },
  estimatedWaterDepthM: 0.35,
  status: "VERIFIED",
  verificationStatus: "VERIFIED",
  sourceType: "COMMUNITY",
  createdAt: "2026-10-02T08:00:00Z",
  updatedAt: "2026-10-02T08:00:00Z",
  verifiedAt: "2026-10-02T09:00:00Z",
  verifiedBy: "admin-1",
  media: []
};

function response(body: unknown, status = 200): Response {
  return { ok: status >= 200 && status < 300, status, statusText: "Request failed", json: vi.fn().mockResolvedValue(body) } as unknown as Response;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

class ReportsEventSource {
  static connections: ReportsEventSource[] = [];
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(readonly url: string) {
    ReportsEventSource.connections.push(this);
  }

  addEventListener() {}
  close() {}
}

function renderReports() {
  return render(<MemoryRouter><ReportsPage /></MemoryRouter>);
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  ReportsEventSource.connections = [];
});

describe("report review API", () => {
  it("sends the reviewed revision when verifying and rejecting", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ report: verifiedReport }));
    vi.stubGlobal("fetch", fetchMock);

    await reportsApi.review("report-1", "verify", "", verifiedReport.updatedAt);
    expect(fetchMock).toHaveBeenLastCalledWith("/api/v1/admin/reports/report-1/verify", expect.objectContaining({
      method: "POST", body: JSON.stringify({ expectedUpdatedAt: verifiedReport.updatedAt })
    }));

    await reportsApi.review("report-1", "reject", "ไม่ตรงกับหลักฐาน", verifiedReport.updatedAt);
    expect(fetchMock).toHaveBeenLastCalledWith("/api/v1/admin/reports/report-1/reject", expect.objectContaining({
      method: "POST", body: JSON.stringify({ expectedUpdatedAt: verifiedReport.updatedAt, reason: "ไม่ตรงกับหลักฐาน" })
    }));
  });
});

describe("ReportsPage public list", () => {
  it("accepts a verified public report without private reviewer identity", async () => {
    const publicReport = Object.fromEntries(Object.entries(verifiedReport).filter(([key]) => key !== "verifiedBy"));
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ items: [publicReport], limit: 20, offset: 0 })));

    renderReports();

    expect(await screen.findByText(verifiedReport.description)).toBeInTheDocument();
    expect(screen.queryByText("ไม่สามารถโหลดรายงานได้")).not.toBeInTheDocument();
  });

  it("shows verified community reports to a guest and offers sign in", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ items: [verifiedReport], limit: 20, offset: 0 }));
    vi.stubGlobal("fetch", fetchMock);

    renderReports();

    expect(screen.getByRole("status")).toHaveTextContent("กำลังโหลดข้อมูล");
    expect(await screen.findByText(verifiedReport.description)).toBeInTheDocument();
    expect(screen.getByText("ยืนยันแล้ว")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "เข้าสู่ระบบเพื่อส่งรายงาน" })).toHaveAttribute("href", "/login?returnTo=%2Freports");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/reports?limit=20&offset=0",
      expect.objectContaining({ credentials: "same-origin" })
    );
  });

  it("keeps unverified items out of the public list", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({
      items: [verifiedReport, { ...verifiedReport, id: "pending", status: "PENDING", verificationStatus: "PENDING" }],
      limit: 20,
      offset: 0
    })));

    renderReports();

    expect(await screen.findByText(verifiedReport.description)).toBeInTheDocument();
    expect(screen.queryByText("pending")).not.toBeInTheDocument();
  });

  it("does not let an older public reload overwrite a newer realtime result", async () => {
    const first = deferred<Response>();
    const second = deferred<Response>();
    const fetchMock = vi.fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("EventSource", ReportsEventSource);

    render(<MemoryRouter><RealtimeProvider><ReportsPage /></RealtimeProvider></MemoryRouter>);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    act(() => ReportsEventSource.connections[0].onopen?.());
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));

    second.resolve(response({ items: [verifiedReport], limit: 20, offset: 0 }));
    expect(await screen.findByText(verifiedReport.description)).toBeInTheDocument();
    first.resolve(response({ items: [], limit: 20, offset: 0 }));
    await waitFor(() => expect(screen.getByText(verifiedReport.description)).toBeInTheDocument());
    expect(screen.queryByText("ยังไม่มีรายงานที่เผยแพร่")).not.toBeInTheDocument();
  });

  it("shows retryable empty and unavailable states honestly", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(response({ items: [], limit: 20, offset: 0 }));
    vi.stubGlobal("fetch", fetchMock);

    const emptyView = renderReports();
    expect(await screen.findByText("ยังไม่มีรายงานที่เผยแพร่")).toBeInTheDocument();
    emptyView.unmount();
    fetchMock.mockResolvedValueOnce(response({ error: { message: "Offline" } }, 503));
    renderReports();
    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดรายงานได้");
    fireEvent.click(screen.getByRole("button", { name: "โหลดรายงานอีกครั้ง" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
  });
});

describe("ReportsPage authenticated workflow", () => {
  it("validates the report form, uploads selected media, and shows pending ownership", async () => {
    const api = {
      refresh: vi.fn().mockResolvedValue({ accessToken: "token" }),
      me: vi.fn().mockResolvedValue({ id: "user-1", email: "user@example.com", role: "USER" as const, createdAt: "2026-10-02T00:00:00Z" }),
      logout: vi.fn(),
      login: vi.fn(),
      register: vi.fn()
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ items: [], limit: 20, offset: 0 }))
      .mockResolvedValueOnce(response({ items: [], limit: 20, offset: 0 }))
      .mockResolvedValueOnce(response({ report: { ...verifiedReport, status: "PENDING", verificationStatus: "PENDING" } }))
      .mockResolvedValueOnce(response({ media: { id: "media-1", url: "/api/v1/reports/report-1/media/media-1" } }))
      .mockResolvedValueOnce(response({ items: [{ ...verifiedReport, status: "PENDING", verificationStatus: "PENDING" }], limit: 20, offset: 0 }));
    vi.stubGlobal("fetch", fetchMock);

    render(<MemoryRouter><AuthProvider api={api}><ReportsPage /></AuthProvider></MemoryRouter>);

    await screen.findByRole("heading", { name: "ส่งรายงานชุมชน" });
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำท่วมขังหน้าตลาด" } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    expect(screen.getByRole("alert")).toHaveTextContent("เลือกตำแหน่งบนแผนที่");

    fireEvent.change(screen.getByLabelText("ลองจิจูด"), { target: { value: "101.1" } });
    fireEvent.change(screen.getByLabelText("ละติจูด"), { target: { value: "14.2" } });
    const file = new File(["evidence"], "water.jpg", { type: "image/jpeg" });
    fireEvent.change(screen.getByLabelText("ภาพประกอบ (JPEG, PNG หรือ WebP; ไม่บังคับ)"), { target: { files: [file] } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/reports",
      expect.objectContaining({ method: "POST" })
    ));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/reports/report-1/media",
      expect.objectContaining({ body: expect.any(FormData), method: "POST" })
    ));
    expect(await screen.findByText("รอตรวจสอบ")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "แก้ไขรายงานที่รอตรวจสอบ" })).toBeInTheDocument();
  });

  it("aligns descriptions, depth, and media selection with backend limits", async () => {
    const api = {
      refresh: vi.fn().mockResolvedValue({ accessToken: "token" }),
      me: vi.fn().mockResolvedValue({ id: "user-1", email: "user@example.com", role: "USER" as const, createdAt: "2026-10-02T00:00:00Z" }),
      logout: vi.fn(), login: vi.fn(), register: vi.fn()
    };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ items: [], limit: 20, offset: 0 })));
    render(<MemoryRouter><AuthProvider api={api}><ReportsPage /></AuthProvider></MemoryRouter>);

    await screen.findByRole("heading", { name: "ส่งรายงานชุมชน" });
    expect(screen.getByLabelText("รายละเอียด")).toHaveAttribute("maxlength", "2000");
    expect(screen.getByLabelText("ระดับน้ำโดยประมาณ (เมตร, ไม่บังคับ)")).toHaveAttribute("max", "100");
    const files = ["a.jpg", "b.jpg", "c.jpg", "d.jpg"].map((name) => new File(["image"], name, { type: "image/jpeg" }));
    fireEvent.change(screen.getByLabelText("ภาพประกอบ (JPEG, PNG หรือ WebP; ไม่บังคับ)"), { target: { files } });
    expect(screen.getByRole("alert")).toHaveTextContent("เลือกภาพได้ไม่เกิน 3 ไฟล์");
  });
});
