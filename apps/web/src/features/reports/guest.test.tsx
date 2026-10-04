import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { fireEvent } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ReportsPage } from "./ReportsPage";
import { CreateReportPage } from "./CreateReportPage";

const api = vi.hoisted(() => ({ listPublic: vi.fn(), mine: vi.fn() }));
vi.mock("./api", () => ({ reportsApi: api }));
vi.mock("../auth/AuthProvider", () => ({ useOptionalAuth: () => ({ account: null, status: "anonymous" }) }));
vi.mock("../realtime/RealtimeProvider", () => ({ useRealtimeRevision: () => 0 }));
vi.mock("./ReportForm", () => ({ ReportForm: () => <p>Guest report form</p> }));
vi.mock("./AdminReviewPanel", () => ({ AdminReviewPanel: () => <p>Admin review</p> }));

beforeEach(() => { vi.clearAllMocks(); api.listPublic.mockResolvedValue({ items: [], limit: 20, offset: 0 }); });

it("lets guests submit without login or private report access", async () => {
  render(<MemoryRouter initialEntries={["/reports"]}><Routes>
    <Route path="/reports" element={<ReportsPage />} />
    <Route path="/reports/new" element={<CreateReportPage />} />
  </Routes></MemoryRouter>);
  expect(screen.queryByText("Guest report form")).not.toBeInTheDocument();
  await waitFor(() => expect(api.listPublic).toHaveBeenCalled());
  fireEvent.click(screen.getByRole("link", { name: "ส่งรายงานสถานการณ์" }));
  expect(screen.getByRole("heading", { name: "เพิ่มรายงานสถานการณ์" })).toBeInTheDocument();
  expect(screen.getByText("Guest report form")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "กลับไปหน้ารายงานสถานการณ์" })).toHaveAttribute("href", "/reports");
  expect(api.mine).not.toHaveBeenCalled();
  expect(document.querySelector('a[href*="login"]')).toBeNull();
  expect(screen.queryByText("Admin review")).not.toBeInTheDocument();
});
