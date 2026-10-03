import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AppShell } from "./AppShell";
import { confirmAction } from "../shared/alerts";

const { logout } = vi.hoisted(() => ({ logout: vi.fn() }));
vi.mock("../features/auth/AuthProvider", () => ({ useOptionalAuth: () => ({ account: { email: "member@example.test", role: "USER" }, logout }) }));
vi.mock("../shared/alerts", () => ({ confirmAction: vi.fn() }));

function CurrentPath() { return <p>{useLocation().pathname}</p>; }

describe("sign out confirmation", () => {
  beforeEach(() => { vi.clearAllMocks(); vi.mocked(confirmAction).mockResolvedValue(false); });

  it("requests confirmation without immediately logging out", async () => {
    render(<MemoryRouter initialEntries={["/overview"]}><AppShell>Content</AppShell></MemoryRouter>);
    fireEvent.click(screen.getByRole("button", { name: "ออกจากระบบ" }));
    await waitFor(() => expect(confirmAction).toHaveBeenCalledTimes(1));
    expect(logout).not.toHaveBeenCalled();
    expect(confirmAction).toHaveBeenCalledWith(expect.objectContaining({ title: "ออกจากระบบหรือไม่?", danger: true, action: logout, cancelText: "อยู่ในระบบต่อ" }));
  });

  it("ignores duplicate clicks while a confirmation is active", async () => {
    let finish: (value: boolean) => void = () => undefined;
    vi.mocked(confirmAction).mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    render(<MemoryRouter initialEntries={["/overview"]}><AppShell>Content</AppShell></MemoryRouter>);
    const button = screen.getByRole("button", { name: "ออกจากระบบ" });
    fireEvent.click(button);
    fireEvent.click(button);
    expect(confirmAction).toHaveBeenCalledTimes(1);
    expect(button).toBeDisabled();
    await act(async () => { finish(false); });
    expect(button).not.toBeDisabled();
  });

  it("returns to the landing page only after successful sign out", async () => {
    vi.mocked(confirmAction).mockImplementation(async (options) => { await options.action(); return true; });
    render(<MemoryRouter initialEntries={["/reports"]}><AppShell><CurrentPath /></AppShell></MemoryRouter>);
    fireEvent.click(screen.getByRole("button", { name: "ออกจากระบบ" }));
    await waitFor(() => expect(screen.getByText("/")).toBeInTheDocument());
    expect(logout).toHaveBeenCalledTimes(1);
  });
});
