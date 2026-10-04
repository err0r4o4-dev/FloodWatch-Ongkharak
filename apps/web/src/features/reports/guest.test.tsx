import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, expect, it, vi } from "vitest";
import { ReportsPage } from "./ReportsPage";

const api = vi.hoisted(() => ({ listPublic: vi.fn(), mine: vi.fn() }));
vi.mock("./api", () => ({ reportsApi: api }));
vi.mock("../auth/AuthProvider", () => ({ useOptionalAuth: () => ({ account: null, status: "anonymous" }) }));
vi.mock("../realtime/RealtimeProvider", () => ({ useRealtimeRevision: () => 0 }));
vi.mock("./ReportForm", () => ({ ReportForm: () => <p>Guest report form</p> }));
vi.mock("./AdminReviewPanel", () => ({ AdminReviewPanel: () => <p>Admin review</p> }));

beforeEach(() => { vi.clearAllMocks(); api.listPublic.mockResolvedValue({ items: [], limit: 20, offset: 0 }); });

it("lets guests submit without login or private report access", async () => {
  render(<MemoryRouter><ReportsPage /></MemoryRouter>);
  expect(screen.getByText("Guest report form")).toBeInTheDocument();
  await waitFor(() => expect(api.listPublic).toHaveBeenCalled());
  expect(api.mine).not.toHaveBeenCalled();
  expect(document.querySelector('a[href*="login"]')).toBeNull();
  expect(screen.queryByText("Admin review")).not.toBeInTheDocument();
});
