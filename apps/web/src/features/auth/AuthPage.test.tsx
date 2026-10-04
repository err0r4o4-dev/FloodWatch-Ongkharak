import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api";
import { AuthPage } from "./AuthPage";
import { AuthProvider } from "./AuthProvider";
import { ForgotPasswordPage } from "./ForgotPasswordPage";

const alertMocks = vi.hoisted(() => ({
  notifySuccess: vi.fn().mockResolvedValue(undefined),
  notifyError: vi.fn().mockResolvedValue(undefined),
  notifyWarning: vi.fn().mockResolvedValue(undefined)
}));

vi.mock("../../shared/alerts", () => alertMocks);

const account = { id: "user-1", email: "user@example.com", role: "USER" as const, createdAt: "2026-10-02T00:00:00Z" };

function renderAuth(mode: "login" | "register", api: { refresh: () => Promise<never>; me: () => Promise<typeof account>; login: (input: { email: string; password: string }) => Promise<{ user: typeof account; accessToken: string; expiresIn: number }>; register: (input: { email: string; username: string; password: string }) => Promise<typeof account>; logout: () => Promise<void> }) {
  render(<MemoryRouter initialEntries={[`/${mode}?returnTo=%2Freports`]}><AuthProvider api={api}><Routes>
    <Route path="/login" element={<AuthPage mode="login" />} /><Route path="/register" element={<AuthPage mode="register" />} />
    <Route path="/forgot-password" element={<ForgotPasswordPage />} />
    <Route path="/reports" element={<p>Reports destination</p>} />
  </Routes></AuthProvider></MemoryRouter>);
}

afterEach(() => {
  vi.clearAllMocks();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("AuthPage", () => {
  it("keeps admin login password-only and preserves an admin return target", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account), login: vi.fn(), register: vi.fn(), logout: vi.fn()
    };
    render(<MemoryRouter initialEntries={["/admin/login?returnTo=%2Fadmin%2Freports"]}><AuthProvider api={api}><AuthPage mode="login" adminMode /></AuthProvider></MemoryRouter>);

    expect(await screen.findByRole("heading", { name: "เข้าสู่ระบบผู้ดูแลระบบ" })).toBeInTheDocument();
    expect(screen.getByText("เข้าสู่ระบบด้วยบัญชีผู้ดูแลระบบเพื่อเข้าถึงพื้นที่จัดการ")).toBeInTheDocument();
    expect(await screen.findByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /ดำเนินการต่อด้วย Google/ })).not.toBeInTheDocument();
    expect(screen.queryByText("ยังไม่มีบัญชี?")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /ลืมรหัสผ่าน/ })).toHaveAttribute("href", "/forgot-password?returnTo=%2Fadmin%2Freports");
  });

  it("lets an authenticated non-admin sign out before using the admin login", async () => {
    const api = {
      refresh: vi.fn().mockResolvedValue(undefined),
      me: vi.fn().mockResolvedValue(account), login: vi.fn(), register: vi.fn(), logout: vi.fn().mockResolvedValue(undefined)
    };
    render(<MemoryRouter initialEntries={["/admin/login"]}><AuthProvider api={api}><AuthPage mode="login" adminMode /></AuthProvider></MemoryRouter>);

    expect(await screen.findByRole("heading", { name: "บัญชีนี้ไม่มีสิทธิ์ผู้ดูแลระบบ" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ออกจากระบบเพื่อเปลี่ยนบัญชี" }));
    expect(api.logout).toHaveBeenCalledOnce();
    expect(await screen.findByRole("heading", { name: "เข้าสู่ระบบผู้ดูแลระบบ" })).toBeInTheDocument();
  });

  it("does not navigate or report admin success when admin login returns a USER", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn().mockResolvedValue({ user: account, accessToken: "token", expiresIn: 900 }),
      register: vi.fn(), logout: vi.fn()
    };
    render(<MemoryRouter initialEntries={["/admin/login?returnTo=%2Fadmin%2Freports"]}><AuthProvider api={api}><AuthPage mode="login" adminMode /></AuthProvider></MemoryRouter>);

    fireEvent.change(await screen.findByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "password1234" } });
    fireEvent.click(screen.getByRole("button", { name: "เข้าสู่ระบบผู้ดูแลระบบ" }));

    expect(await screen.findByRole("heading", { name: "บัญชีนี้ไม่มีสิทธิ์ผู้ดูแลระบบ" })).toBeInTheDocument();
    expect(screen.queryByText("Reports destination")).not.toBeInTheDocument();
    expect(alertMocks.notifySuccess).not.toHaveBeenCalled();
  });

  it("keeps pending integrations visibly unavailable while allowing password visibility", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account), login: vi.fn(), register: vi.fn(), logout: vi.fn()
    };
    renderAuth("login", api);
    const password = await screen.findByLabelText("รหัสผ่าน");
    fireEvent.change(password, { target: { value: "secret" } });
    fireEvent.click(screen.getByRole("checkbox", { name: "แสดงรหัสผ่าน" }));
    expect(password).toHaveAttribute("type", "text");
    expect(screen.getByRole("button", { name: /ดำเนินการต่อด้วย Google/ })).toBeDisabled();
    expect(screen.getByText(/ลืมรหัสผ่าน/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("link", { name: /ลืมรหัสผ่าน/ }));
    expect(await screen.findByRole("heading", { name: "ลืมรหัสผ่าน?" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ส่งรหัส OTP/ })).toBeInTheDocument();
  });

  it("logs in and returns to the requested reports route", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn().mockResolvedValue({ user: account, accessToken: "token", expiresIn: 900 }),
      register: vi.fn(),
      logout: vi.fn()
    };
    renderAuth("login", api);

    fireEvent.change(await screen.findByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "password1234" } });
    fireEvent.click(screen.getByRole("button", { name: /เข้าสู่ระบบ/ }));

    expect(await screen.findByText("Reports destination")).toBeInTheDocument();
    expect(api.login).toHaveBeenCalledWith({ email: account.email, password: "password1234" });
    expect(alertMocks.notifySuccess).toHaveBeenCalledWith("เข้าสู่ระบบสำเร็จ");
  });

  it("shows API failures inline and in an error notification", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn().mockRejectedValue(new ApiError("บัญชีไม่พร้อมใช้งาน", 503, "SERVICE_UNAVAILABLE")),
      register: vi.fn(),
      logout: vi.fn()
    };
    renderAuth("login", api);

    fireEvent.change(await screen.findByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "password1234" } });
    fireEvent.click(screen.getByRole("button", { name: /เข้าสู่ระบบ/ }));

    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถเข้าสู่ระบบได้ กรุณาลองใหม่");
    expect(alertMocks.notifyError).toHaveBeenCalledWith("เข้าสู่ระบบไม่สำเร็จ", "ไม่สามารถเข้าสู่ระบบได้ กรุณาลองใหม่");
    expect(alertMocks.notifySuccess).not.toHaveBeenCalled();
  });

  it("shows network failures inline and in an error notification", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn().mockRejectedValue(new Error("network failure")),
      register: vi.fn(),
      logout: vi.fn()
    };
    renderAuth("login", api);

    fireEvent.change(await screen.findByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "password1234" } });
    fireEvent.click(screen.getByRole("button", { name: /เข้าสู่ระบบ/ }));

    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถเข้าสู่ระบบได้");
    expect(alertMocks.notifyError).toHaveBeenCalledWith("เข้าสู่ระบบไม่สำเร็จ", "ไม่สามารถเข้าสู่ระบบได้ กรุณาลองใหม่");
  });

  it("does not submit twice while the request is pending", async () => {
    let resolveLogin: ((value: { user: typeof account; accessToken: string; expiresIn: number }) => void) | undefined;
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn(() => new Promise<{ user: typeof account; accessToken: string; expiresIn: number }>((resolve) => {
        resolveLogin = resolve;
      })),
      register: vi.fn(),
      logout: vi.fn()
    };
    renderAuth("login", api);

    fireEvent.change(await screen.findByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "password1234" } });
    const submit = screen.getByRole("button", { name: /เข้าสู่ระบบ/ });
    fireEvent.click(submit);
    fireEvent.click(submit);

    expect(api.login).toHaveBeenCalledOnce();
    resolveLogin?.({ user: account, accessToken: "token", expiresIn: 900 });
  });

  it("registers with a normalized username before creating a session", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn().mockResolvedValue({ user: account, accessToken: "token", expiresIn: 900 }),
      register: vi.fn().mockResolvedValue(account),
      logout: vi.fn()
    };
    renderAuth("register", api);

    fireEvent.change(await screen.findByLabelText("อีเมล"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("ชื่อผู้ใช้งาน"), { target: { value: "Test_User" } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "password1234" } });
    fireEvent.click(screen.getByRole("button", { name: /สมัครสมาชิก/ }));

    expect(await screen.findByText("Reports destination")).toBeInTheDocument();
    expect(api.register).toHaveBeenCalledWith({ email: account.email, username: "test_user", password: "password1234" });
    expect(api.login).toHaveBeenCalledWith({ email: account.email, password: "password1234" });
    expect(alertMocks.notifySuccess).toHaveBeenCalledWith("สมัครสมาชิกสำเร็จ");
  });

  it("distinguishes a successful registration from an automatic login failure", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn().mockRejectedValue(new ApiError("บัญชียังเข้าสู่ระบบไม่ได้", 503, "SERVICE_UNAVAILABLE")),
      register: vi.fn().mockResolvedValue(account),
      logout: vi.fn()
    };
    renderAuth("register", api);

    fireEvent.change(await screen.findByLabelText("อีเมล"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("ชื่อผู้ใช้งาน"), { target: { value: "testuser" } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "password1234" } });
    fireEvent.click(screen.getByRole("button", { name: /สมัครสมาชิก/ }));

    expect(await screen.findByRole("status", { name: "สมัครสมาชิกสำเร็จ" })).toHaveTextContent("สมัครสมาชิกสำเร็จ");
    expect(screen.getByRole("link", { name: "เข้าสู่ระบบด้วยบัญชีที่สร้างไว้" })).toHaveAttribute("href", "/login?returnTo=%2Freports");
    expect(screen.getByRole("alert")).toHaveTextContent("เข้าสู่ระบบอัตโนมัติไม่สำเร็จ");
    expect(alertMocks.notifySuccess).toHaveBeenCalledWith("สมัครสมาชิกสำเร็จ");
  });

  it("rejects passwords shorter than the backend policy before network access", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn()
    };
    renderAuth("login", api);

    fireEvent.change(await screen.findByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "short" } });
    fireEvent.click(screen.getByRole("button", { name: /เข้าสู่ระบบ/ }));

    expect(screen.getByRole("alert")).toHaveTextContent("รหัสผ่านสั้นเกินไป");
    expect(api.login).not.toHaveBeenCalled();
  });

  it.each([
    ["ไทยไทยไทยไทย", true],
    ["😀😀😀", true],
    ["😀".repeat(33), false]
  ])("uses UTF-8 byte length for Thai and emoji passwords (%s)", async (password, accepted) => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn().mockResolvedValue({ user: account, accessToken: "token", expiresIn: 900 }),
      register: vi.fn(),
      logout: vi.fn()
    };
    renderAuth("login", api);

    fireEvent.change(await screen.findByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: password } });
    fireEvent.click(screen.getByRole("button", { name: /เข้าสู่ระบบ/ }));

    if (accepted) {
      expect(await screen.findByText("Reports destination")).toBeInTheDocument();
      expect(api.login).toHaveBeenCalledWith({ email: account.email, password });
    } else {
      expect(await screen.findByRole("alert")).toHaveTextContent("รหัสผ่านยาวเกินไป");
      expect(api.login).not.toHaveBeenCalled();
    }
  });

  it("maps backend account conflicts to honest Thai guidance", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn().mockResolvedValue(account),
      login: vi.fn(),
      register: vi.fn().mockRejectedValue(new ApiError("conflict", 409, "USERNAME_EXISTS")),
      logout: vi.fn()
    };
    renderAuth("register", api);

    fireEvent.change(await screen.findByLabelText("อีเมล"), { target: { value: account.email } });
    fireEvent.change(screen.getByLabelText("ชื่อผู้ใช้งาน"), { target: { value: "testuser" } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "password1234" } });
    fireEvent.click(screen.getByRole("button", { name: /สมัครสมาชิก/ }));

    expect(await screen.findByRole("alert")).toHaveTextContent("ชื่อผู้ใช้นี้ถูกใช้งานแล้ว");
    expect(api.login).not.toHaveBeenCalled();
  });
});
