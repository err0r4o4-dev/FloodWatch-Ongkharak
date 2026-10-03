import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api";
import type { PasswordRecoveryApi } from "./api";
import { ForgotPasswordPage } from "./ForgotPasswordPage";

const email = "user@example.com";

function renderPage(api: PasswordRecoveryApi, entry = "/forgot-password?returnTo=%2Freports") {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route path="/forgot-password" element={<ForgotPasswordPage api={api} />} />
        <Route path="/login" element={<p>Login destination</p>} />
      </Routes>
    </MemoryRouter>
  );
}

function successfulApi(): PasswordRecoveryApi {
  return {
    requestOtp: vi.fn().mockResolvedValue({ challengeId: "challenge-1", expiresIn: 600, resendAfter: 60, message: "หากอีเมลนี้มีบัญชี คุณจะได้รับคำแนะนำสำหรับดำเนินการต่อ" }),
    verifyOtp: vi.fn().mockResolvedValue({ resetToken: "reset-1", expiresIn: 300 }),
    resetPassword: vi.fn().mockResolvedValue(undefined)
  };
}

afterEach(() => {
  vi.useRealTimers();
});

describe("ForgotPasswordPage", () => {
  it("requests an OTP without claiming delivery and applies the server cooldown", async () => {
    const api = successfulApi();
    renderPage(api);

    fireEvent.change(screen.getByLabelText("อีเมล"), { target: { value: email } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรหัส OTP" }));

    expect(await screen.findByRole("heading", { name: "ยืนยันรหัส OTP" })).toBeInTheDocument();
    expect(screen.getByText(`รหัส OTP สำหรับ ${email}`)).toBeInTheDocument();
    expect(screen.getByRole("status", { name: "ข้อมูลการกู้คืน" })).toHaveTextContent("หากอีเมลนี้มีบัญชี");
    expect(screen.getByRole("timer", { name: "เวลารอส่งรหัสอีกครั้ง" })).toHaveAttribute("aria-live", "off");
    expect(screen.getByRole("button", { name: /ส่งรหัสอีกครั้ง/ })).toBeDisabled();
    expect(api.requestOtp).toHaveBeenCalledWith(email);
  });

  it("keeps verification errors on the OTP step and maps the agreed error code", async () => {
    const api = successfulApi();
    vi.mocked(api.verifyOtp).mockRejectedValue(new ApiError("invalid", 400, "INVALID_OTP"));
    renderPage(api);

    fireEvent.change(screen.getByLabelText("อีเมล"), { target: { value: email } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรหัส OTP" }));
    await screen.findByRole("heading", { name: "ยืนยันรหัส OTP" });
    fireEvent.change(screen.getByLabelText("รหัส OTP"), { target: { value: "123456" } });
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันรหัส OTP" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่ถูกต้อง หมดอายุ หรือถูกใช้ไปแล้ว");
    expect(screen.getByRole("heading", { name: "ยืนยันรหัส OTP" })).toBeInTheDocument();
  });

  it("replaces the challenge when a resend is allowed", async () => {
    const api = successfulApi();
    vi.mocked(api.requestOtp)
      .mockResolvedValueOnce({ challengeId: "challenge-1", expiresIn: 600, resendAfter: 0, message: "first" })
      .mockResolvedValueOnce({ challengeId: "challenge-2", expiresIn: 600, resendAfter: 60, message: "second" });
    renderPage(api);

    fireEvent.change(screen.getByLabelText("อีเมล"), { target: { value: email } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรหัส OTP" }));
    await screen.findByRole("heading", { name: "ยืนยันรหัส OTP" });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรหัสอีกครั้ง" }));

    expect(await screen.findByRole("status", { name: "ข้อมูลการกู้คืน" })).toHaveTextContent("second");
    expect(api.requestOtp).toHaveBeenCalledTimes(2);
    expect(api.requestOtp).toHaveBeenLastCalledWith(email);
  });

  it("does not call the API for a mismatched password confirmation", async () => {
    const api = successfulApi();
    renderPage(api);

    fireEvent.change(screen.getByLabelText("อีเมล"), { target: { value: email } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรหัส OTP" }));
    await screen.findByRole("heading", { name: "ยืนยันรหัส OTP" });
    fireEvent.change(screen.getByLabelText("รหัส OTP"), { target: { value: "123456" } });
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันรหัส OTP" }));
    await screen.findByRole("heading", { name: "ตั้งรหัสผ่านใหม่" });
    fireEvent.change(screen.getByLabelText("รหัสผ่านใหม่"), { target: { value: "password1234" } });
    fireEvent.change(screen.getByLabelText("ยืนยันรหัสผ่านใหม่"), { target: { value: "password5678" } });
    fireEvent.click(screen.getByRole("button", { name: "เปลี่ยนรหัสผ่าน" }));

    expect(screen.getByRole("alert")).toHaveTextContent("รหัสผ่านไม่ตรงกัน");
    expect(api.resetPassword).not.toHaveBeenCalled();
  });

  it("returns to recovery when the reset token is invalid", async () => {
    const api = successfulApi();
    vi.mocked(api.resetPassword).mockRejectedValue(new ApiError("invalid", 401, "INVALID_RESET_TOKEN"));
    renderPage(api);

    fireEvent.change(screen.getByLabelText("อีเมล"), { target: { value: email } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรหัส OTP" }));
    await screen.findByRole("heading", { name: "ยืนยันรหัส OTP" });
    fireEvent.change(screen.getByLabelText("รหัส OTP"), { target: { value: "123456" } });
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันรหัส OTP" }));
    await screen.findByRole("heading", { name: "ตั้งรหัสผ่านใหม่" });
    fireEvent.change(screen.getByLabelText("รหัสผ่านใหม่"), { target: { value: "password1234" } });
    fireEvent.change(screen.getByLabelText("ยืนยันรหัสผ่านใหม่"), { target: { value: "password1234" } });
    fireEvent.click(screen.getByRole("button", { name: "เปลี่ยนรหัสผ่าน" }));

    expect(await screen.findByRole("heading", { name: "ลืมรหัสผ่าน?" })).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("หมดอายุ");
    expect(screen.queryByDisplayValue("password1234")).not.toBeInTheDocument();
  });

  it("starts over from Step 3 and clears password fields", async () => {
    const api = successfulApi();
    renderPage(api);

    fireEvent.change(screen.getByLabelText("อีเมล"), { target: { value: email } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรหัส OTP" }));
    await screen.findByRole("heading", { name: "ยืนยันรหัส OTP" });
    fireEvent.change(screen.getByLabelText("รหัส OTP"), { target: { value: "123456" } });
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันรหัส OTP" }));
    await screen.findByRole("heading", { name: "ตั้งรหัสผ่านใหม่" });
    fireEvent.change(screen.getByLabelText("รหัสผ่านใหม่"), { target: { value: "password1234" } });
    fireEvent.change(screen.getByLabelText("ยืนยันรหัสผ่านใหม่"), { target: { value: "password1234" } });
    fireEvent.click(screen.getByRole("button", { name: "เริ่มกู้คืนใหม่" }));

    expect(await screen.findByRole("heading", { name: "ลืมรหัสผ่าน?" })).toBeInTheDocument();
    expect(screen.queryByDisplayValue("password1234")).not.toBeInTheDocument();
    expect(screen.getByLabelText("อีเมล")).toHaveValue(email);
  });

  it("keeps a persistent success state and preserves returnTo on the login action", async () => {
    const api = successfulApi();
    renderPage(api);

    fireEvent.change(screen.getByLabelText("อีเมล"), { target: { value: email } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรหัส OTP" }));
    await screen.findByRole("heading", { name: "ยืนยันรหัส OTP" });
    fireEvent.change(screen.getByLabelText("รหัส OTP"), { target: { value: "123456" } });
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันรหัส OTP" }));
    await screen.findByRole("heading", { name: "ตั้งรหัสผ่านใหม่" });
    fireEvent.change(screen.getByLabelText("รหัสผ่านใหม่"), { target: { value: "password1234" } });
    fireEvent.change(screen.getByLabelText("ยืนยันรหัสผ่านใหม่"), { target: { value: "password1234" } });
    fireEvent.click(screen.getByRole("button", { name: "เปลี่ยนรหัสผ่าน" }));

    expect(await screen.findByRole("heading", { name: "เปลี่ยนรหัสผ่านสำเร็จ" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "กลับไปหน้าเข้าสู่ระบบ" })).toBeInTheDocument();
    expect(api.resetPassword).toHaveBeenCalledWith({ resetToken: "reset-1", password: "password1234" });
    fireEvent.click(screen.getByRole("button", { name: "กลับไปหน้าเข้าสู่ระบบ" }));
    expect(await screen.findByText("Login destination")).toBeInTheDocument();
  });

  it("rejects malformed OTP locally", async () => {
    const api = successfulApi();
    renderPage(api);

    fireEvent.change(screen.getByLabelText("อีเมล"), { target: { value: email } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรหัส OTP" }));
    await screen.findByRole("heading", { name: "ยืนยันรหัส OTP" });
    fireEvent.change(screen.getByLabelText("รหัส OTP"), { target: { value: "12ab" } });
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันรหัส OTP" }));

    expect(screen.getByRole("alert")).toHaveTextContent("รหัส OTP ต้องเป็นตัวเลข 6 หลัก");
    expect(api.verifyOtp).not.toHaveBeenCalled();
  });

  it.each([
    ["INVALID_REQUEST", "ข้อมูลคำขอไม่ถูกต้อง"],
    ["RECOVERY_RATE_LIMITED", "ดำเนินการกู้คืนรหัสผ่านบ่อยเกินไป"],
    ["RATE_LIMITED", "ดำเนินการกู้คืนรหัสผ่านบ่อยเกินไป"],
    ["RECOVERY_UNAVAILABLE", "บริการกู้คืนรหัสผ่านไม่พร้อมใช้งาน"]
  ])("maps %s recovery failures honestly", async (code, message) => {
    const api = successfulApi();
    vi.mocked(api.requestOtp).mockRejectedValue(new ApiError("server detail", 400, code));
    renderPage(api);

    fireEvent.change(screen.getByLabelText("อีเมล"), { target: { value: email } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรหัส OTP" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(screen.getByRole("heading", { name: "ลืมรหัสผ่าน?" })).toBeInTheDocument();
  });
});
