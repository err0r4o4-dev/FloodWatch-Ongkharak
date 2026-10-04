import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider, LanguageSwitcher } from "../../i18n";
import { AdminDashboardPage } from "./AdminDashboardPage";
import { SettingsPage } from "./SettingsPage";
import { UsersPage } from "./UsersPage";
import { adminApi } from "./api";
import { AuthProvider } from "../auth/AuthProvider";
import type { AuthApi } from "../auth/api";
import type { Account } from "../auth/types";

vi.mock("./api", () => ({
  adminApi: {
    getDashboardStats: vi.fn(),
    listUsers: vi.fn(),
    listSettings: vi.fn(),
  },
}));

const adminAccount: Account = {
  id: "admin-id",
  email: "admin@example.test",
  role: "ADMIN",
  createdAt: "2024-01-01T00:00:00Z",
};

const authApi: AuthApi = {
  register: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(),
  refresh: vi.fn(),
  me: vi.fn().mockResolvedValue(adminAccount),
};

describe("admin English localization", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.setItem("floodwatch.locale", "en");
    vi.mocked(adminApi.getDashboardStats).mockResolvedValue({
      totalUsers: 12,
      activeUsers: 9,
      totalReports: 7,
      pendingReports: 3,
      recentActivity: 5,
    });
    vi.mocked(adminApi.listUsers).mockResolvedValue({
      users: [],
      pagination: { total: 0, limit: 50, offset: 0 },
    });
  });

  it("renders dashboard headings and actions in English while preserving the account email", async () => {
    render(
      <BrowserRouter>
        <LanguageProvider>
          <AuthProvider api={authApi}>
            <AdminDashboardPage />
          </AuthProvider>
        </LanguageProvider>
      </BrowserRouter>,
    );

    expect(await screen.findByRole("heading", { name: "Admin dashboard" })).toBeInTheDocument();
    expect(screen.getByText("admin@example.test")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open user management" })).toHaveAttribute("href", "/admin/users");
  });

  it("renders the user search form in English", async () => {
    render(
      <BrowserRouter>
        <LanguageProvider>
          <UsersPage />
        </LanguageProvider>
      </BrowserRouter>,
    );

    await waitFor(() => expect(screen.getByText("No users found")).toBeInTheDocument());
    expect(screen.getByPlaceholderText("Search email...")).toBeInTheDocument();
    expect(screen.getByLabelText("Role:")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Search" })).toBeInTheDocument();
  });

  it("re-renders a stored settings error after switching locale", async () => {
    localStorage.setItem("floodwatch.locale", "th");
    vi.mocked(adminApi.listSettings).mockRejectedValue(new Error("server detail"));

    render(
      <BrowserRouter>
        <LanguageProvider>
          <LanguageSwitcher />
          <SettingsPage />
        </LanguageProvider>
      </BrowserRouter>,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดการตั้งค่าได้");
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Unable to load settings");
  });
});
