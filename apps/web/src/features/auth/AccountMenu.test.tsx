import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccountMenu } from "./AccountMenu";
import { LanguageProvider, LanguageSwitcher } from "../../i18n";
import { AuthProvider } from "./AuthProvider";
import { ApiError } from "./api";
import type { Account } from "./types";

const account: Account = { id: "account-1", email: "a.very.long.email.address@example.test", username: "original_user", role: "USER", createdAt: "2026-10-04T00:00:00Z" };

function setup(currentAccount: Account | null = account, updateUsername = vi.fn().mockResolvedValue({ ...account, username: "new_user" }), path = "/admin") {
  const api = { refresh: vi.fn().mockResolvedValue(undefined), me: vi.fn().mockResolvedValue(currentAccount), login: vi.fn(), logout: vi.fn(), register: vi.fn() };
  const onLogout = vi.fn();
  render(<MemoryRouter initialEntries={[path]}><AuthProvider api={api} accountApi={{ updateUsername }}><AccountMenu signingOut={false} onLogout={onLogout} /></AuthProvider></MemoryRouter>);
  return { updateUsername, onLogout };
}

beforeEach(() => localStorage.clear());

async function openMenu() {
  await waitFor(() => expect(screen.getByLabelText("เมนูผู้ใช้งาน")).not.toHaveAttribute("aria-disabled", "true"));
  fireEvent.click(screen.getByLabelText("เมนูผู้ใช้งาน"));
}

describe("account dropdown", () => {
  it("translates the guest menu and accessible labels without losing the unsaved display name", async () => {
    const api = { refresh: vi.fn().mockRejectedValue(new Error("guest")), me: vi.fn(), login: vi.fn(), logout: vi.fn(), register: vi.fn() };
    render(<LanguageProvider><LanguageSwitcher /><MemoryRouter initialEntries={["/reports"]}><AuthProvider api={api}><AccountMenu signingOut={false} onLogout={vi.fn()} /></AuthProvider></MemoryRouter></LanguageProvider>);
    fireEvent.click(screen.getByLabelText("เมนูผู้ใช้งาน"));
    fireEvent.change(screen.getByRole("textbox", { name: "ชื่อที่แสดง" }), { target: { value: "ชื่อฉัน" } });
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    expect(screen.getByLabelText("Account menu")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Display name" })).toHaveValue("ชื่อฉัน");
    expect(screen.getByRole("button", { name: "Save" })).toBeInTheDocument();
  });
  it("shows the editable default guest identity without signed-out actions", async () => {
    setup(null, undefined, "/overview");
    await openMenu();
    expect(screen.getByRole("textbox", { name: "ชื่อที่แสดง" })).toHaveValue("Guest");
    expect(screen.getByRole("button", { name: "บันทึก" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "สมัครสมาชิก" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "เข้าสู่ระบบ" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "ออกจากระบบ" })).not.toBeInTheDocument();
  });

  it("keeps the guest menu on public paths for an authenticated ADMIN", async () => {
    setup({ ...account, role: "ADMIN" }, undefined, "/overview");
    await openMenu();
    expect(screen.getByRole("textbox", { name: "ชื่อที่แสดง" })).toHaveValue("Guest");
    expect(screen.getByLabelText("เมนูผู้ใช้งาน").querySelector(".account-user-icon__check")).not.toBeInTheDocument();
    expect(screen.queryByText(account.email)).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "จัดการระบบ" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "ออกจากระบบ" })).not.toBeInTheDocument();
  });

  it("keeps public guest editing available while auth bootstrap is pending", async () => {
    const api = { refresh: vi.fn().mockImplementation(() => new Promise<void>(() => undefined)), me: vi.fn(), login: vi.fn(), logout: vi.fn(), register: vi.fn() };
    render(<MemoryRouter initialEntries={["/map"]}><AuthProvider api={api}><AccountMenu signingOut={false} onLogout={vi.fn()} /></AuthProvider></MemoryRouter>);
    expect(screen.getByLabelText("เมนูผู้ใช้งาน")).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(screen.getByLabelText("เมนูผู้ใช้งาน"));
    expect(screen.getByRole("textbox", { name: "ชื่อที่แสดง" })).toHaveValue("Guest");
  });

  it("keeps spaces in the draft until the guest name is saved", async () => {
    setup(null, undefined, "/reports");
    await openMenu();
    const input = screen.getByRole("textbox", { name: "ชื่อที่แสดง" });
    fireEvent.change(input, { target: { value: "  Local guest  " } });
    expect(input).toHaveValue("  Local guest  ");
    fireEvent.click(screen.getByRole("button", { name: "บันทึก" }));
    expect(input).toHaveValue("Local guest");
  });

  it("shows the checked icon, full email, username and logout without a profile link", async () => {
    const { onLogout } = setup();
    await openMenu();
    expect(screen.getByLabelText("เมนูผู้ใช้งาน").querySelector(".account-user-icon__check")).toBeInTheDocument();
    expect(screen.getByText(account.email)).toBeInTheDocument();
    expect(screen.getByText(account.username!)).toBeInTheDocument();
    expect(screen.queryByText("โปรไฟล์")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ออกจากระบบ" }));
    expect(onLogout).toHaveBeenCalledOnce();
  });

  it("edits in place, normalizes and updates the account", async () => {
    const { updateUsername } = setup();
    await openMenu();
    fireEvent.click(screen.getByRole("button", { name: "แก้ไขชื่อผู้ใช้" }));
    const input = screen.getByRole("textbox", { name: "ชื่อผู้ใช้งาน" });
    expect(input).toHaveFocus();
    expect(input).toHaveValue("original_user");
    fireEvent.change(input, { target: { value: " NEW_USER " } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(updateUsername).toHaveBeenCalledExactlyOnceWith("new_user"));
    expect(await screen.findByText("บันทึกชื่อผู้ใช้แล้ว")).toBeInTheDocument();
    expect(screen.getByText("new_user")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "แก้ไขชื่อผู้ใช้" })).toHaveFocus();
  });

  it("cancels without saving", async () => {
    const { updateUsername } = setup();
    await openMenu();
    fireEvent.click(screen.getByRole("button", { name: "แก้ไขชื่อผู้ใช้" }));
    fireEvent.change(screen.getByRole("textbox", { name: "ชื่อผู้ใช้งาน" }), { target: { value: "new_user" } });
    fireEvent.click(screen.getByRole("button", { name: "ยกเลิก" }));
    expect(screen.getByText("original_user")).toBeInTheDocument();
    expect(updateUsername).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "แก้ไขชื่อผู้ใช้" })).toHaveFocus();
  });

  it("rejects invalid names without sending a request", async () => {
    const { updateUsername } = setup();
    await openMenu();
    fireEvent.click(screen.getByRole("button", { name: "แก้ไขชื่อผู้ใช้" }));
    fireEvent.change(screen.getByRole("textbox", { name: "ชื่อผู้ใช้งาน" }), { target: { value: "ชื่อไม่ถูกต้อง" } });
    fireEvent.click(screen.getByRole("button", { name: "บันทึก" }));
    expect(screen.getByRole("alert")).toHaveTextContent("3–32");
    expect(screen.getByRole("textbox", { name: "ชื่อผู้ใช้งาน" })).toHaveAttribute("aria-invalid", "true");
    expect(updateUsername).not.toHaveBeenCalled();
  });

  it.each([
    [new ApiError("duplicate", 409, "USERNAME_EXISTS"), "ชื่อผู้ใช้นี้ถูกใช้งานแล้ว"],
    [new Error("network"), "บันทึกไม่สำเร็จ"],
    [new ApiError("expired", 401), "เซสชันหมดอายุ"]
  ])("keeps the draft when saving fails (%s)", async (error, message) => {
    setup(account, vi.fn().mockRejectedValue(error));
    await openMenu();
    fireEvent.click(screen.getByRole("button", { name: "แก้ไขชื่อผู้ใช้" }));
    fireEvent.change(screen.getByRole("textbox", { name: "ชื่อผู้ใช้งาน" }), { target: { value: "new_user" } });
    fireEvent.click(screen.getByRole("button", { name: "บันทึก" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(screen.getByRole("textbox", { name: "ชื่อผู้ใช้งาน" })).toHaveValue("new_user");
  });

  it("prevents duplicate submission and logout while saving", async () => {
    let resolveSave!: (value: Account) => void;
    const { updateUsername } = setup(account, vi.fn().mockImplementation(() => new Promise<Account>((resolve) => { resolveSave = resolve; })));
    await openMenu();
    fireEvent.click(screen.getByRole("button", { name: "แก้ไขชื่อผู้ใช้" }));
    const input = screen.getByRole("textbox", { name: "ชื่อผู้ใช้งาน" });
    fireEvent.change(input, { target: { value: "new_user" } });
    fireEvent.submit(input.closest("form")!);
    fireEvent.submit(input.closest("form")!);
    expect(updateUsername).toHaveBeenCalledOnce();
    expect(screen.getByRole("button", { name: "กำลังบันทึก…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "ออกจากระบบ" })).toBeDisabled();
    resolveSave({ ...account, username: "new_user" });
    expect(await screen.findByText("บันทึกชื่อผู้ใช้แล้ว")).toBeInTheDocument();
  });

  it("supports accounts without usernames and retains the admin link", async () => {
    setup({ ...account, username: null, role: "ADMIN" });
    await openMenu();
    expect(screen.getByText("ยังไม่ได้ตั้งชื่อ")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "จัดการระบบ" })).toHaveAttribute("href", "/admin");
    fireEvent.click(screen.getByRole("button", { name: "ตั้งชื่อผู้ใช้" }));
    expect(screen.getByRole("textbox", { name: "ชื่อผู้ใช้งาน" })).toHaveValue("");
  });

  it("closes with Escape and returns focus to the trigger", async () => {
    setup();
    await openMenu();
    fireEvent.click(screen.getByRole("button", { name: "แก้ไขชื่อผู้ใช้" }));
    fireEvent.keyDown(screen.getByRole("textbox", { name: "ชื่อผู้ใช้งาน" }), { key: "Escape" });
    expect(screen.getByLabelText("เมนูผู้ใช้งาน").closest("details")).not.toHaveAttribute("open");
    expect(screen.getByLabelText("เมนูผู้ใช้งาน")).toHaveFocus();
  });
});
