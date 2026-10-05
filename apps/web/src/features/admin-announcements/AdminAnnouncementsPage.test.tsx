import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { AdminAnnouncementsPage, canAdvanceAnnouncementPage } from "./AdminAnnouncementsPage";
import type { ManualAnnouncement } from "../manual-announcements/api";

const api = vi.hoisted(() => ({
  adminList: vi.fn(),
  adminGet: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  publish: vi.fn(),
  withdraw: vi.fn(),
}));

vi.mock("../manual-announcements/api", () => ({ manualAnnouncementApi: api }));
vi.mock("../manual-announcements/validation", () => ({
  publicationIssues: vi.fn(() => []),
  bangkokDateTimeValue: (value: string | null) => value ?? "",
  fromBangkokDateTime: (value: string) => value ? `${value}:00+07:00` : null,
}));
vi.mock("../manual-announcements/AnnouncementCard", () => ({
  AnnouncementCard: ({ announcement, preview }: { announcement: { title: string }; preview?: boolean }) => (
    <article data-testid="announcement-card">
      {preview ? "ตัวอย่าง: " : ""}{announcement.title}
    </article>
  ),
}));

const baseAnnouncement: ManualAnnouncement = {
  id: "announcement-1",
  title: "ประกาศทดสอบ",
  summary: "สรุปทดสอบ",
  body: "รายละเอียดทดสอบ",
  advice: "ติดตามข้อมูล",
  area: "องครักษ์",
  severity: "WATCH" as const,
  observedAt: "2026-10-05T08:00:00+07:00",
  expiresAt: "2026-10-06T08:00:00+07:00",
  references: [],
  sourceType: "COMMUNITY" as const,
  publisherType: "ADMIN" as const,
  referenceKind: "MEDIA" as const,
  status: "DRAFT" as const,
  effectiveStatus: "WATCH" as const,
  createdAt: "2026-10-05T01:00:00Z",
  updatedAt: "2026-10-05T01:00:00Z",
  publishedAt: null,
  withdrawnAt: null,
  withdrawalReason: "",
  revision: 1,
};

function page(items = [baseAnnouncement], total = items.length) {
  return { items, total, limit: 20, offset: 0 };
}

function detail(announcement = baseAnnouncement) {
  return { announcement, history: [] };
}

function renderPage() {
  return render(<AdminAnnouncementsPage />);
}

describe("AdminAnnouncementsPage", () => {
  beforeEach(() => {
    Object.values(api).forEach((mock) => mock.mockReset());
    api.adminList.mockResolvedValue(page());
    api.adminGet.mockResolvedValue(detail());
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("loads status-filtered pages and changes the offset by twenty", async () => {
    api.adminList.mockResolvedValueOnce(page()).mockResolvedValueOnce(page([baseAnnouncement], 21));
    renderPage();

    expect(await screen.findByText("ประกาศทดสอบ")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "เผยแพร่แล้ว" }));
    await waitFor(() => expect(api.adminList).toHaveBeenLastCalledWith("PUBLISHED", 0, expect.any(AbortSignal)));

    fireEvent.click(screen.getByRole("button", { name: /ถัดไป/ }));
    await waitFor(() => expect(api.adminList).toHaveBeenLastCalledWith("PUBLISHED", 20, expect.any(AbortSignal)));
  });

  it("saves a partial draft and preserves unsaved fields when saving fails", async () => {
    api.create.mockRejectedValue(new Error("server unavailable"));
    renderPage();
    await screen.findByText("ประกาศทดสอบ");

    fireEvent.click(screen.getByRole("button", { name: "สร้างประกาศ" }));
    const title = screen.getByLabelText("หัวข้อ");
    fireEvent.change(title, { target: { value: "ร่างที่ยังแก้ไข" } });
    fireEvent.click(screen.getByRole("button", { name: "บันทึกร่าง" }));

    await screen.findByRole("alert");
    expect(title).toHaveValue("ร่างที่ยังแก้ไข");
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({ title: "ร่างที่ยังแก้ไข" }));
  });

  it("previews unsaved editor input without publishing it", async () => {
    renderPage();
    await screen.findByText("ประกาศทดสอบ");
    fireEvent.click(screen.getByRole("button", { name: "สร้างประกาศ" }));
    fireEvent.change(screen.getByLabelText("หัวข้อ"), { target: { value: "ตัวอย่างที่ยังไม่บันทึก" } });
    fireEvent.click(screen.getByRole("button", { name: "ดูตัวอย่าง" }));

    expect(screen.getByTestId("announcement-card")).toHaveTextContent("ตัวอย่างที่ยังไม่บันทึก");
    expect(api.create).not.toHaveBeenCalled();
  });

  it("requires an explicit confirmation before publishing a draft", async () => {
    renderPage();
    await screen.findByText("ประกาศทดสอบ");
    fireEvent.click(screen.getByRole("button", { name: /แก้ไข.*ประกาศทดสอบ/ }));
    await screen.findByLabelText("หัวข้อ");
    fireEvent.click(screen.getByRole("button", { name: "เผยแพร่" }));

    expect(api.publish).not.toHaveBeenCalled();
    expect(screen.getByRole("group", { name: "ยืนยันการเผยแพร่" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันเผยแพร่" }));
    await waitFor(() => expect(api.publish).toHaveBeenCalledWith("announcement-1", 1));
  });

  it("saves dirty draft input before allowing publish and uses the saved revision", async () => {
    const saved = { ...baseAnnouncement, title: "ร่างที่บันทึกแล้ว", revision: 2 };
    const published = { ...saved, status: "PUBLISHED" as const, publishedAt: "2026-10-05T02:00:00Z" };
    api.adminList.mockResolvedValueOnce(page([baseAnnouncement])).mockResolvedValue(page([saved]));
    api.adminGet.mockResolvedValueOnce(detail(baseAnnouncement)).mockResolvedValueOnce(detail(saved));
    api.update.mockResolvedValue({ announcement: saved });
    api.publish.mockResolvedValue({ announcement: published });
    renderPage();
    await screen.findByText("ประกาศทดสอบ");
    fireEvent.click(screen.getByRole("button", { name: /แก้ไข.*ประกาศทดสอบ/ }));
    await screen.findByLabelText("หัวข้อ");
    fireEvent.change(screen.getByLabelText("หัวข้อ"), { target: { value: "ร่างที่บันทึกแล้ว" } });
    fireEvent.click(screen.getByRole("button", { name: "เผยแพร่" }));

    expect(api.publish).not.toHaveBeenCalled();
    expect(screen.getByText(/บันทึกร่างก่อนเผยแพร่/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "บันทึกร่าง" }));
    await waitFor(() => expect(api.update).toHaveBeenCalledWith("announcement-1", expect.objectContaining({ title: "ร่างที่บันทึกแล้ว" }), 1));
    expect(await screen.findByRole("status")).toHaveTextContent("อัปเดตสำเร็จ");
    fireEvent.click(await screen.findByRole("button", { name: /แก้ไข.*ร่างที่บันทึกแล้ว/ }));
    await screen.findByLabelText("หัวข้อ");
    fireEvent.click(screen.getByRole("button", { name: "เผยแพร่" }));
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันเผยแพร่" }));
    await waitFor(() => expect(api.publish).toHaveBeenCalledWith("announcement-1", 2));
  });

  it("reloads detail and immutable history when opening the same announcement again", async () => {
    const latest = { ...baseAnnouncement, title: "ฉบับล่าสุด" };
    api.adminGet.mockResolvedValueOnce(detail(baseAnnouncement)).mockResolvedValueOnce(detail(latest));
    renderPage();
    await screen.findByText("ประกาศทดสอบ");
    fireEvent.click(screen.getByRole("button", { name: /แก้ไข.*ประกาศทดสอบ/ }));
    await screen.findByDisplayValue("ประกาศทดสอบ");
    fireEvent.click(screen.getByRole("button", { name: /แก้ไข.*ประกาศทดสอบ/ }));
    await waitFor(() => expect(api.adminGet).toHaveBeenCalledTimes(2));
    expect(await screen.findByDisplayValue("ฉบับล่าสุด")).toBeInTheDocument();
  });

  it("invalidates an unsaved preview when a reference is added", async () => {
    renderPage();
    await screen.findByText("ประกาศทดสอบ");
    fireEvent.click(screen.getByRole("button", { name: "สร้างประกาศ" }));
    fireEvent.change(screen.getByLabelText("หัวข้อ"), { target: { value: "ตัวอย่าง" } });
    fireEvent.click(screen.getByRole("button", { name: "ดูตัวอย่าง" }));
    expect(screen.getByTestId("announcement-card")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "เพิ่มแหล่งอ้างอิง" }));
    expect(screen.queryByTestId("announcement-card")).not.toBeInTheDocument();
  });

  it("does not allow navigation controls to replace an editor during a pending mutation", async () => {
    let resolveCreate: (value: { announcement: ManualAnnouncement }) => void = () => undefined;
    api.create.mockReturnValue(new Promise((resolve) => { resolveCreate = resolve; }));
    renderPage();
    await screen.findByText("ประกาศทดสอบ");
    fireEvent.click(screen.getByRole("button", { name: "สร้างประกาศ" }));
    fireEvent.change(screen.getByLabelText("หัวข้อ"), { target: { value: "กำลังบันทึก" } });
    fireEvent.click(screen.getByRole("button", { name: "บันทึกร่าง" }));
    expect(screen.getByRole("button", { name: "สร้างประกาศ" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "ยกเลิก" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /แก้ไข.*ประกาศทดสอบ/ })).toBeDisabled();
    resolveCreate({ announcement: { ...baseAnnouncement, title: "กำลังบันทึก" } });
    await screen.findByRole("status");
  });

  it("previews edits to published content, requires confirmation, and preserves conflict input", async () => {
    const published = { ...baseAnnouncement, status: "PUBLISHED" as const, publishedAt: "2026-10-05T02:00:00Z" };
    api.adminList.mockResolvedValue(page([published]));
    api.adminGet.mockResolvedValue(detail(published));
    api.update.mockRejectedValue({ status: 409 });
    renderPage();
    await screen.findByText("ประกาศทดสอบ");

    fireEvent.click(screen.getByRole("button", { name: /แก้ไข.*ประกาศทดสอบ/ }));
    await screen.findByLabelText("หัวข้อ");
    const title = screen.getByLabelText("หัวข้อ");
    fireEvent.change(title, { target: { value: "ฉบับแก้ไขที่ยังไม่โหลดใหม่" } });
    fireEvent.click(screen.getByRole("button", { name: "ดูตัวอย่าง" }));

    expect(api.update).not.toHaveBeenCalled();
    expect(screen.getByTestId("announcement-card")).toHaveTextContent("ตัวอย่าง");
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันอัปเดต" }));
    await screen.findByRole("alert");
    expect(screen.getByText(/ข้อมูลเปลี่ยนแปลงแล้ว/)).toBeInTheDocument();
    expect(title).toHaveValue("ฉบับแก้ไขที่ยังไม่โหลดใหม่");
    expect(screen.getByText(/โหลดข้อมูลใหม่/)).toBeInTheDocument();
  });

  it("requires a withdrawal reason and confirmation for published content", async () => {
    const published = { ...baseAnnouncement, status: "PUBLISHED" as const, publishedAt: "2026-10-05T02:00:00Z" };
    api.adminList.mockResolvedValue(page([published]));
    api.adminGet.mockResolvedValue(detail(published));
    api.withdraw.mockResolvedValue({ announcement: { ...published, status: "WITHDRAWN" as const } });
    renderPage();
    await screen.findByText("ประกาศทดสอบ");
    fireEvent.click(screen.getByRole("button", { name: /แก้ไข.*ประกาศทดสอบ/ }));
    await screen.findByLabelText("หัวข้อ");
    fireEvent.click(screen.getByRole("button", { name: "ถอนประกาศ" }));

    expect(api.withdraw).not.toHaveBeenCalled();
    expect(screen.getByText(/คลัง/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("เหตุผลการถอน"), { target: { value: "ข้อมูลไม่ถูกต้อง" } });
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันถอนประกาศ" }));
    await waitFor(() => expect(api.withdraw).toHaveBeenCalledWith("announcement-1", 1, "ข้อมูลไม่ถูกต้อง"));
  });

  it("caps admin paging before the API offset limit", () => {
    expect(canAdvanceAnnouncementPage(9_980, 10_040)).toBe(true);
    expect(canAdvanceAnnouncementPage(10_000, 10_040)).toBe(false);
  });

  it("shows a retryable error when the admin list request stalls", async () => {
    vi.useFakeTimers();
    api.adminList.mockImplementation(() => new Promise(() => undefined));
    renderPage();
    await act(async () => { vi.advanceTimersByTime(10_001); });
    expect(screen.getByRole("alert")).toHaveTextContent("ไม่สามารถโหลดประกาศได้");
    expect(screen.getByRole("button", { name: "ลองโหลดอีกครั้ง" })).toBeInTheDocument();
  });

  it("shows a retryable error when the admin detail request stalls", async () => {
    vi.useFakeTimers();
    api.adminGet.mockImplementation(() => new Promise(() => undefined));
    renderPage();
    await act(async () => { await Promise.resolve(); });
    fireEvent.click(screen.getByRole("button", { name: /แก้ไข.*ประกาศทดสอบ/ }));
    await act(async () => { vi.advanceTimersByTime(10_001); });
    expect(screen.getByRole("alert")).toHaveTextContent("ไม่สามารถโหลดรายละเอียดประกาศได้");
    expect(screen.getByRole("button", { name: "ลองโหลดอีกครั้ง" })).toBeInTheDocument();
  });
});
