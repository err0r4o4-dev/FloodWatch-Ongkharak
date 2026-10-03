import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UserDetailPage } from "./UserDetailPage";
import { adminApi } from "./api";
import { confirmAction } from "../../shared/alerts";

vi.mock("./api", () => ({ adminApi: { getUser: vi.fn(), updateUserRole: vi.fn(), disableUser: vi.fn(), enableUser: vi.fn() } }));
vi.mock("../../shared/alerts", () => ({ confirmAction: vi.fn() }));

const user = { id: "member", email: "member@example.test", role: "USER" as const, createdAt: "2026-10-02T00:00:00Z", updatedAt: "2026-10-02T00:00:00Z", disabledAt: null };
const stats = { reportCount: 0, lastActivity: null, verifiedReports: 0, rejectedReports: 0, auditLogCount: 0, refreshSessionCount: 1 };

describe("admin account confirmations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(window, "confirm").mockReturnValue(false);
    vi.mocked(adminApi.getUser).mockResolvedValue({ user, stats });
    vi.mocked(confirmAction).mockResolvedValue(false);
  });

  function open() { render(<MemoryRouter initialEntries={["/admin/users/member"]}><Routes><Route path="/admin/users/:id" element={<UserDetailPage />} /></Routes></MemoryRouter>); }

  it.each(["ตั้งเป็นผู้ดูแลระบบ", "ปิดการใช้งานบัญชี", "เปิดการใช้งานบัญชี"])("requires confirmation for %s and leaves cancellation untouched", async (label) => {
    if (label === "เปิดการใช้งานบัญชี") vi.mocked(adminApi.getUser).mockResolvedValue({ user: { ...user, disabledAt: user.createdAt }, stats });
    open();
    fireEvent.click(await screen.findByRole("button", { name: label }));
    await waitFor(() => expect(confirmAction).toHaveBeenCalledTimes(1));
    expect(adminApi.updateUserRole).not.toHaveBeenCalled();
    expect(adminApi.disableUser).not.toHaveBeenCalled();
    expect(adminApi.enableUser).not.toHaveBeenCalled();
    expect(window.confirm).not.toHaveBeenCalled();
  });

  it("performs the confirmed mutation once and reloads the account", async () => {
    vi.mocked(confirmAction).mockImplementation(async (options) => { await options.action(); return true; });
    open();
    fireEvent.click(await screen.findByRole("button", { name: "ตั้งเป็นผู้ดูแลระบบ" }));
    await waitFor(() => expect(adminApi.updateUserRole).toHaveBeenCalledWith("member", "ADMIN"));
    expect(adminApi.updateUserRole).toHaveBeenCalledTimes(1);
    expect(adminApi.getUser).toHaveBeenCalledTimes(2);
  });
});
