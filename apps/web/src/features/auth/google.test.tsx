import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "./AuthProvider";
import { ApiError } from "./api";
import { googleAuthApi, safeReturnTo } from "./google";
import { GoogleSignInButton } from "./GoogleSignInButton";
import { GoogleCompletePage, GoogleLinkPage } from "./GoogleAuthPages";

const alertMocks = vi.hoisted(() => ({
  notifySuccess: vi.fn().mockResolvedValue(undefined),
  notifyError: vi.fn().mockResolvedValue(undefined),
  notifyInfo: vi.fn().mockResolvedValue(undefined),
  notifyWarning: vi.fn().mockResolvedValue(undefined)
}));

vi.mock("../../shared/alerts", () => alertMocks);

afterEach(() => {
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("Google login", () => {
  it("rejects external and ambiguous destinations", () => {
    for (const destination of ["//evil.test", "https://evil.test", "/\\evil.test", "/%2f%2fevil.test", "/%252fevil.test", "/auth/complete", "/login", "/reports/../login", "/reports\n"]) {
      expect(safeReturnTo(destination)).toBe("/reports");
    }
    expect(safeReturnTo("/map?mode=route")).toBe("/map?mode=route");
  });

  it("enables configured Google login and uses a same-origin start URL", async () => {
    const navigate = vi.fn();
    const api = { available: vi.fn().mockResolvedValue(true), link: vi.fn() };
    render(<GoogleSignInButton returnTo="/reports?mode=mine" api={api} navigate={navigate} />);
    const button = await screen.findByRole("button", { name: /Continue with Google/ });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);
    expect(navigate).toHaveBeenCalledWith("/api/v1/auth/google/start?returnTo=%2Freports%3Fmode%3Dmine");
    expect(button).toBeDisabled();
  });

  it("keeps disabled configuration distinct from a configuration outage", async () => {
    const api = { available: vi.fn().mockResolvedValue(false), link: vi.fn() };
    const { unmount } = render(<GoogleSignInButton returnTo="/reports" api={api} />);
    expect(await screen.findByText("Google Login ยังไม่ได้ตั้งค่า")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Continue with Google/ })).toBeDisabled();
    unmount();
    render(<GoogleSignInButton returnTo="/reports" api={{ ...api, available: vi.fn().mockRejectedValue(new Error("outage")) }} />);
    expect(await screen.findByText("ตรวจสอบ Google Login ไม่สำเร็จ ใช้อีเมลและรหัสผ่านได้ตามปกติ")).toBeInTheDocument();
  });

  it("requests only the password when linking and completes via bootstrap navigation", async () => {
    const events: string[] = [];
    alertMocks.notifySuccess.mockImplementation(() => Promise.resolve().then(() => {
      events.push("notification-complete");
    }));
    const navigate = vi.fn(() => {
      events.push("navigate");
    });
    const api = { available: vi.fn(), link: vi.fn().mockResolvedValue({ returnTo: "/reports?mode=mine" }) };
    render(<MemoryRouter><GoogleLinkPage api={api} navigate={navigate} /></MemoryRouter>);
    fireEvent.change(screen.getByLabelText("รหัสผ่านบัญชีเดิม"), { target: { value: "correct-password" } });
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันและเชื่อม Google" }));
    await waitFor(() => expect(navigate).toHaveBeenCalledWith("/auth/complete?returnTo=%2Freports%3Fmode%3Dmine"));
    expect(api.link).toHaveBeenCalledWith("correct-password");
    expect(alertMocks.notifySuccess).toHaveBeenCalledWith("เชื่อมบัญชี Google สำเร็จ");
    expect(events).toEqual(["notification-complete", "navigate"]);
  });

  it("shows link failure without success and blocks exhausted attempts", async () => {
    const navigate = vi.fn();
    const api = { available: vi.fn(), link: vi.fn().mockRejectedValue(new ApiError("private", 429, "GOOGLE_LINK_LIMIT")) };
    render(<MemoryRouter><GoogleLinkPage api={api} navigate={navigate} /></MemoryRouter>);
    fireEvent.change(screen.getByLabelText("รหัสผ่านบัญชีเดิม"), { target: { value: "wrong-password" } });
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันและเชื่อม Google" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("ครบจำนวนครั้ง");
    expect(screen.getByRole("button", { name: "ยืนยันและเชื่อม Google" })).toBeDisabled();
    expect(navigate).not.toHaveBeenCalled();
    expect(alertMocks.notifyError).toHaveBeenCalledWith("เชื่อมบัญชี Google ไม่สำเร็จ", expect.stringContaining("ครบจำนวนครั้ง"));
  });

  it("does not claim success when refresh bootstrap fails", async () => {
    const api = { refresh: vi.fn().mockRejectedValue(new Error("session unavailable")), me: vi.fn(), login: vi.fn(), register: vi.fn(), logout: vi.fn() };
    render(<MemoryRouter initialEntries={["/auth/complete?returnTo=/reports"]}><AuthProvider api={api}><GoogleCompletePage /></AuthProvider></MemoryRouter>);
    expect(await screen.findByRole("alert")).toHaveTextContent("ยืนยันการเข้าสู่ระบบไม่สำเร็จ");
    expect(screen.getByRole("link", { name: "กลับไปเข้าสู่ระบบ" })).toHaveAttribute("href", "/login?returnTo=%2Freports");
    expect(alertMocks.notifySuccess).not.toHaveBeenCalled();
    expect(alertMocks.notifyError).not.toHaveBeenCalled();
  });

  it("returns to the requested page only after refresh and me succeed", async () => {
    const account = { id: "user-1", email: "person@example.test", role: "USER" as const, createdAt: "2026-10-03T00:00:00Z" };
    const api = { refresh: vi.fn().mockResolvedValue({}), me: vi.fn().mockResolvedValue(account), login: vi.fn(), register: vi.fn(), logout: vi.fn() };
    render(<MemoryRouter initialEntries={["/auth/complete?returnTo=/reports"]}><AuthProvider api={api}><Routes><Route path="/auth/complete" element={<GoogleCompletePage />} /><Route path="/reports" element={<p>Reports destination</p>} /></Routes></AuthProvider></MemoryRouter>);
    expect(await screen.findByText("Reports destination")).toBeInTheDocument();
    expect(api.me).toHaveBeenCalledOnce();
  });

  it("uses cookies and strict typed responses without auth retries", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ enabled: true }), { status: 200 })).mockResolvedValueOnce(new Response(JSON.stringify({ returnTo: "/map" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    expect(await googleAuthApi.available()).toBe(true);
    expect(await googleAuthApi.link("password")).toEqual({ returnTo: "/map" });
    expect(fetchMock).toHaveBeenLastCalledWith("/api/v1/auth/google/link", expect.objectContaining({ method: "POST", credentials: "same-origin", body: JSON.stringify({ password: "password" }) }));
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ enabled: "yes" }), { status: 200 }));
    await expect(googleAuthApi.available()).rejects.toThrow();
  });
});
