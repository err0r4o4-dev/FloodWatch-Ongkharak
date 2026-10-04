import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { AdminDashboardPage } from "./AdminDashboardPage";
import { adminApi } from "./api";
import { AuthProvider } from "../auth/AuthProvider";
import type { AuthApi } from "../auth/api";
import type { Account } from "../auth/types";

vi.mock("./api", () => ({
  adminApi: { getDashboardStats: vi.fn() },
}));

const adminAccount: Account = {
  id: "admin-id",
  email: "admin@test.com",
  role: "ADMIN",
  createdAt: "2024-01-01T00:00:00Z",
};

const authApi: AuthApi = {
  register: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(),
  refresh: vi.fn().mockResolvedValue({ user: adminAccount, accessToken: "token", expiresIn: 900 }),
  me: vi.fn().mockResolvedValue(adminAccount),
};

function renderPage() {
  return render(
    <BrowserRouter>
      <AuthProvider api={authApi}>
        <AdminDashboardPage />
      </AuthProvider>
    </BrowserRouter>
  );
}

describe("AdminDashboardPage dashboard stats", () => {
  beforeEach(() => {
    vi.mocked(adminApi.getDashboardStats).mockReset();
  });

  it("shows loading and repository-backed stats for an authenticated admin", async () => {
    let resolveStats: (value: {
      totalUsers: number;
      activeUsers: number;
      totalReports: number;
      pendingReports: number;
      recentActivity: number;
    }) => void = () => undefined;
    vi.mocked(adminApi.getDashboardStats).mockReturnValue(new Promise((resolve) => {
      resolveStats = resolve;
    }));

    renderPage();

    expect(await screen.findByText("กำลังโหลดสถิติแดชบอร์ด...")).toBeInTheDocument();
    resolveStats({
      totalUsers: 12,
      activeUsers: 9,
      totalReports: 7,
      pendingReports: 3,
      recentActivity: 5,
    });
    expect(await screen.findByText("ผู้ใช้งานทั้งหมด")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("รายงานที่ยังไม่ตรวจสอบ")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();

    const reportsLink = screen.getByRole("link", { name: /จัดการรายงาน/ });
    expect(reportsLink).toHaveAttribute("href", "/admin/reports");
  });

  it("shows an error when dashboard stats cannot be loaded", async () => {
    vi.mocked(adminApi.getDashboardStats).mockRejectedValue(new Error("database unavailable"));

    renderPage();

    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดสถิติแดชบอร์ดได้");
    expect(screen.queryByText("database unavailable")).not.toBeInTheDocument();
  });

  it("retries failed stats without substituting mock zeroes", async () => {
    vi.mocked(adminApi.getDashboardStats).mockRejectedValueOnce(new Error("database unavailable"))
      .mockResolvedValueOnce({ totalUsers: 12, activeUsers: 9, totalReports: 7, pendingReports: 3, recentActivity: 5 });
    renderPage();
    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดสถิติแดชบอร์ดได้");
    expect(screen.queryByText("0")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ลองโหลดสถิติอีกครั้ง" }));
    expect(await screen.findByText("12")).toBeInTheDocument();
  });
});
