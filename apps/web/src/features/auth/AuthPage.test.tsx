import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthPage } from "./AuthPage";
import { AuthProvider } from "./AuthProvider";

const account = { id: "user-1", email: "user@example.com", role: "USER" as const, createdAt: "2026-10-02T00:00:00Z" };

function renderAuth(mode: "login" | "register", api: { refresh: () => Promise<never>; me: () => Promise<typeof account>; login: (input: { email: string; password: string }) => Promise<{ user: typeof account; accessToken: string; expiresIn: number }>; register: (input: { email: string; password: string }) => Promise<typeof account>; logout: () => Promise<void> }) {
  render(<MemoryRouter initialEntries={[`/${mode}?returnTo=%2Freports`]}><AuthProvider api={api}><Routes>
    <Route path="/login" element={<AuthPage mode="login" />} /><Route path="/register" element={<AuthPage mode="register" />} />
    <Route path="/reports" element={<p>Reports destination</p>} />
  </Routes></AuthProvider></MemoryRouter>);
}

afterEach(() => vi.restoreAllMocks());

describe("AuthPage", () => {
  it("logs in and returns to the requested reports route", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn().mockResolvedValue({ user: account, accessToken: "token", expiresIn: 900 }),
      register: vi.fn(),
      logout: vi.fn()
    };
    renderAuth("login", api);

    fireEvent.change(await screen.findByLabelText("อีเมล"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "password" } });
    fireEvent.click(screen.getByRole("button", { name: "เข้าสู่ระบบ" }));

    expect(await screen.findByText("Reports destination")).toBeInTheDocument();
    expect(api.login).toHaveBeenCalledWith({ email: account.email, password: "password" });
  });

  it("registers before creating a session", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn().mockResolvedValue({ user: account, accessToken: "token", expiresIn: 900 }),
      register: vi.fn().mockResolvedValue(account),
      logout: vi.fn()
    };
    renderAuth("register", api);

    fireEvent.change(await screen.findByLabelText("อีเมล"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "password" } });
    fireEvent.click(screen.getByRole("button", { name: "สมัครสมาชิกและเข้าสู่ระบบ" }));

    expect(await screen.findByText("Reports destination")).toBeInTheDocument();
    expect(api.register).toHaveBeenCalledWith({ email: account.email, password: "password" });
    expect(api.login).toHaveBeenCalledWith({ email: account.email, password: "password" });
  });
});
