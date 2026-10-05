import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../auth/AuthProvider";
import { AdminDashboardPage } from "./AdminDashboardPage";
vi.mock("./api", () => ({ adminApi: { getDashboardStats: vi.fn().mockResolvedValue({ totalUsers: 0, activeUsers: 0, totalReports: 0, pendingReports: 0, recentActivity: 0 }) } }));
describe("manual announcement navigation", () => {
  it("provides an administrator entry to the announcement workflow", async () => {
    const api = { refresh: vi.fn(), me: vi.fn().mockResolvedValue({ id: "admin-id", email: "admin@example.com", role: "ADMIN", createdAt: "2026-10-04T00:00:00Z" }), login: vi.fn(), register: vi.fn(), logout: vi.fn() };
    render(<MemoryRouter><AuthProvider api={api}><AdminDashboardPage /></AuthProvider></MemoryRouter>);
    expect(await screen.findByRole("link", { name: /เปิดจัดการประกาศ/ })).toHaveAttribute("href", "/admin/announcements");
  });
});
