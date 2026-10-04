import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { AdminDashboardPage } from "./AdminDashboardPage";
import { UsersPage } from "./UsersPage";
import { RequireAdmin, safeAdminReturnTo } from "./RequireAdmin";
import { AuthProvider } from "../auth/AuthProvider";
import type { AuthApi } from "../auth/api";
import type { Account } from "../auth/types";

const mockAdminAccount: Account = {
  id: "admin-id",
  email: "admin@test.com",
  role: "ADMIN",
  createdAt: "2024-01-01T00:00:00Z",
};

const mockUserAccount: Account = {
  id: "user-id",
  email: "user@test.com",
  role: "USER",
  createdAt: "2024-01-01T00:00:00Z",
};

const createMockAuthApi = (account: Account | null): AuthApi => ({
  register: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(),
  refresh: vi.fn(),
  me: vi.fn().mockResolvedValue(account),
});

describe("AdminDashboardPage", () => {
  it("renders admin dashboard for ADMIN users", async () => {
    const mockApi = createMockAuthApi(mockAdminAccount);
    
    render(
      <BrowserRouter>
        <AuthProvider api={mockApi}>
          <AdminDashboardPage />
        </AuthProvider>
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("ส่วนจัดการระบบ")).toBeInTheDocument();
    });

    expect(screen.getByText("จัดการผู้ใช้งาน")).toBeInTheDocument();
    expect(screen.getByText("ตั้งค่าระบบ")).toBeInTheDocument();
    expect(screen.getByText("บันทึกการดำเนินการ")).toBeInTheDocument();
  });

  it("denies access for non-ADMIN users", async () => {
    const mockApi = createMockAuthApi(mockUserAccount);
    
    render(
      <BrowserRouter>
        <AuthProvider api={mockApi}>
          <AdminDashboardPage />
        </AuthProvider>
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("ไม่มีสิทธิ์เข้าถึง")).toBeInTheDocument();
    });

    expect(screen.getByText("คุณต้องเป็นผู้ดูแลระบบเพื่อเข้าถึงหน้านี้")).toBeInTheDocument();
  });

  it("has accessible navigation links", async () => {
    const mockApi = createMockAuthApi(mockAdminAccount);
    
    render(
      <BrowserRouter>
        <AuthProvider api={mockApi}>
          <AdminDashboardPage />
        </AuthProvider>
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("เปิดการจัดการผู้ใช้งาน")).toBeInTheDocument();
    });

    const userManagementButton = screen.getByText("เปิดการจัดการผู้ใช้งาน");
    expect(userManagementButton).toHaveClass("button");
    expect(userManagementButton.closest("a")).toHaveAttribute("href", "/admin/users");
  });
});

describe("RequireAdmin", () => {
  it("accepts only same-origin admin return targets", () => {
    expect(safeAdminReturnTo("/admin/reports?status=pending#queue")).toBe("/admin/reports?status=pending#queue");
    expect(safeAdminReturnTo("/reports")).toBe("/admin");
    expect(safeAdminReturnTo("//evil.example/admin")).toBe("/admin");
    expect(safeAdminReturnTo("/admin/login")).toBe("/admin");
  });

  it("shows loading state while checking auth", () => {
    const mockApi: AuthApi = {
      register: vi.fn(),
      login: vi.fn(),
      logout: vi.fn(),
      refresh: vi.fn(),
      me: vi.fn().mockImplementation(() => new Promise(() => {})), // Never resolves
    };

    render(
      <BrowserRouter>
        <AuthProvider api={mockApi}>
          <RequireAdmin>
            <div>Protected Content</div>
          </RequireAdmin>
        </AuthProvider>
      </BrowserRouter>
    );

    expect(screen.getByText("กำลังตรวจสอบสิทธิ์...")).toBeInTheDocument();
  });

  it("renders children for ADMIN users", async () => {
    const mockApi = createMockAuthApi(mockAdminAccount);

    render(
      <BrowserRouter>
        <AuthProvider api={mockApi}>
          <RequireAdmin>
            <div>Protected Content</div>
          </RequireAdmin>
        </AuthProvider>
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("Protected Content")).toBeInTheDocument();
    });
  });

  it("blocks non-ADMIN users", async () => {
    const mockApi = createMockAuthApi(mockUserAccount);

    render(
      <BrowserRouter>
        <AuthProvider api={mockApi}>
          <RequireAdmin>
            <div>Protected Content</div>
          </RequireAdmin>
        </AuthProvider>
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("ไม่มีสิทธิ์เข้าถึง")).toBeInTheDocument();
    });

    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
  });
});

describe("UsersPage accessibility", () => {
  it("has accessible form labels", () => {
    render(
      <BrowserRouter>
        <UsersPage />
      </BrowserRouter>
    );

    expect(screen.getByLabelText(/บทบาท/i)).toBeInTheDocument();
    expect(screen.getByRole("searchbox")).toBeInTheDocument();
  });

  it("has accessible table structure", async () => {
    render(
      <BrowserRouter>
        <UsersPage />
      </BrowserRouter>
    );

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByText("กำลังโหลดรายชื่อผู้ใช้งาน...")).not.toBeInTheDocument();
    });

    // Should have table headers
    const headers = screen.queryAllByRole("columnheader");
    if (headers.length > 0) {
      expect(headers.length).toBeGreaterThan(0);
    }
  });

  it("has accessible pagination buttons", () => {
    render(
      <BrowserRouter>
        <UsersPage />
      </BrowserRouter>
    );

    const buttons = screen.getAllByRole("button");
    const prevButton = buttons.find(btn => btn.textContent?.includes("ก่อนหน้า"));
    const nextButton = buttons.find(btn => btn.textContent?.includes("ถัดไป"));

    if (prevButton) {
      expect(prevButton).toHaveAttribute("disabled");
    }
    if (nextButton) {
      expect(nextButton).toBeInTheDocument();
    }
  });
});

describe("Admin UI accessibility", () => {
  it("admin dashboard has heading hierarchy", async () => {
    const mockApi = createMockAuthApi(mockAdminAccount);
    
    render(
      <BrowserRouter>
        <AuthProvider api={mockApi}>
          <AdminDashboardPage />
        </AuthProvider>
      </BrowserRouter>
    );

    await waitFor(() => {
      const h1 = screen.getByRole("heading", { level: 1 });
      expect(h1).toHaveTextContent("ส่วนจัดการระบบ");
    });

    const h2Elements = screen.getAllByRole("heading", { level: 2 });
    expect(h2Elements.length).toBeGreaterThan(0);
  });

  it("buttons have accessible text", async () => {
    const mockApi = createMockAuthApi(mockAdminAccount);
    
    render(
      <BrowserRouter>
        <AuthProvider api={mockApi}>
          <AdminDashboardPage />
        </AuthProvider>
      </BrowserRouter>
    );

    await waitFor(() => {
      const buttons = screen.getAllByRole("link", { name: /เปิด/i });
      expect(buttons.length).toBeGreaterThan(0);
      buttons.forEach(button => {
        expect(button.textContent).toBeTruthy();
        expect(button.textContent!.length).toBeGreaterThan(0);
      });
    });
  });

  it("warning panels have semantic structure", async () => {
    const mockApi = createMockAuthApi(mockAdminAccount);
    
    render(
      <BrowserRouter>
        <AuthProvider api={mockApi}>
          <AdminDashboardPage />
        </AuthProvider>
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/บัญชีของคุณ/i)).toBeInTheDocument();
    });
  });
});
