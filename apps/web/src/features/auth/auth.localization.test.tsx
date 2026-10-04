import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider } from "../../i18n";
import th from "../../i18n/locales/th/auth";
import en from "../../i18n/locales/en/auth";
import { AuthPage } from "./AuthPage";
import { AuthProvider } from "./AuthProvider";

vi.mock("./GoogleSignInButton", () => ({ GoogleSignInButton: () => null }));

beforeEach(() => localStorage.clear());

describe("auth localization catalogs", () => {
  it("keeps Thai and English catalogs aligned with plain-language password guidance", () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(th).sort());
    expect(th.authPasswordHint).toContain("วลีที่จำง่าย");
    expect(th.authPasswordHint).toContain("ภาษาไทยและอีโมจิใช้ได้");
    expect(en.authPasswordHint).toContain("memorable phrase");
    expect(en.authPasswordHint).toContain("12–128 characters");
    expect(en.authPasswordHint).toContain("Thai and emoji are allowed");
    expect(en.authPasswordHint).not.toMatch(/ASCII|bytes/i);
    expect(en.passwordTechnicalDetails).toContain("bytes");
    for (const copy of [th, en]) {
      expect([copy.passwordTooShort, copy.passwordTooLong, copy.newPasswordPlaceholder].join(" ")).not.toMatch(/ไบต์|bytes|ASCII/i);
    }
  });

  it("switches the auth form copy without losing entered values", async () => {
    const api = { refresh: vi.fn().mockRejectedValue(new Error("anonymous")), me: vi.fn(), login: vi.fn(), register: vi.fn(), logout: vi.fn() };
    render(
      <LanguageProvider>
        <MemoryRouter initialEntries={["/login"]}>
          <AuthProvider api={api}>
            <AuthPage mode="login" />
          </AuthProvider>
        </MemoryRouter>
      </LanguageProvider>
    );

    const email = await screen.findByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน");
    fireEvent.change(email, { target: { value: "person@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "EN" }));

    expect(screen.getByRole("heading", { name: "Sign in" })).toBeInTheDocument();
    expect(screen.getByLabelText("Email or username")).toHaveValue("person@example.test");
    expect(screen.getByText(/memorable phrase/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "TH" }));
    expect(screen.getByRole("heading", { name: "เข้าสู่ระบบ" })).toBeInTheDocument();
    expect(screen.getByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน")).toHaveValue("person@example.test");
  });

  it("re-renders a stored validation error in the selected locale", async () => {
    const api = { refresh: vi.fn().mockRejectedValue(new Error("anonymous")), me: vi.fn(), login: vi.fn(), register: vi.fn(), logout: vi.fn() };
    render(
      <LanguageProvider>
        <MemoryRouter initialEntries={["/login"]}>
          <AuthProvider api={api}>
            <AuthPage mode="login" />
          </AuthProvider>
        </MemoryRouter>
      </LanguageProvider>
    );

    fireEvent.change(await screen.findByLabelText("อีเมล หรือ ชื่อผู้ใช้งาน"), { target: { value: "person@example.test" } });
    fireEvent.change(screen.getByLabelText("รหัสผ่าน"), { target: { value: "short" } });
    fireEvent.click(screen.getByRole("button", { name: "เข้าสู่ระบบ" }));
    expect(screen.getByRole("alert")).toHaveTextContent("รหัสผ่านสั้นเกินไป");

    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    expect(screen.getByRole("alert")).toHaveTextContent("Password is too short");
  });
});
