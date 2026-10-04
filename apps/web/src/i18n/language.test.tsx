import { fireEvent, render, screen } from "@testing-library/react";
import { StrictMode } from "react";
import { act } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider, LanguageSwitcher, formatDate, translate, useTranslation } from "./index";

const messages = { th: { title: "ชื่อ", count: "ทั้งหมด {count} รายการ" }, en: { title: "Name", count: "{count} items" } };
function Form() {
  const { t } = useTranslation(messages);
  return <><LanguageSwitcher /><label>{t("title")}<input aria-label={t("title")} defaultValue="" /></label><p>{t("count", { count: 2 })}</p></>;
}
beforeEach(() => localStorage.clear());
describe("application language", () => {
  it("switches instantly, keeps entered values, and remembers the choice", () => {
    const view = render(<LanguageProvider><Form /></LanguageProvider>);
    fireEvent.change(screen.getByLabelText("ชื่อ"), { target: { value: "draft" } });
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    expect(screen.getByLabelText("Name")).toHaveValue("draft");
    expect(screen.getByText("2 items")).toBeInTheDocument();
    expect(document.documentElement.lang).toBe("en");
    expect(localStorage.getItem("floodwatch.locale")).toBe("en");
    view.unmount();
    render(<LanguageProvider><Form /></LanguageProvider>);
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "TH" }));
    expect(document.documentElement.lang).toBe("th");
  });
  it("defaults unsupported saved languages to Thai", () => {
    localStorage.setItem("floodwatch.locale", "unsupported");
    render(<LanguageProvider><Form /></LanguageProvider>);
    expect(screen.getByLabelText("ชื่อ")).toBeInTheDocument();
  });
  it("formats the same Bangkok time with Thai and English calendars", () => {
    expect(formatDate("2026-10-04T08:00:00Z", "th", { year: "numeric" })).toContain("2569");
    expect(formatDate("2026-10-04T08:00:00Z", "en", { year: "numeric" })).toContain("2026");
    expect(formatDate("invalid", "en")).toBe("—");
  });
  it("keeps non-React messages in the selected language under StrictMode", () => {
    localStorage.setItem("floodwatch.locale", "en");
    render(<StrictMode><LanguageProvider><Form /></LanguageProvider></StrictMode>);
    expect(translate(messages, "title")).toBe("Name");
    act(() => window.dispatchEvent(new StorageEvent("storage", { key: "floodwatch.locale", newValue: "th" })));
    expect(screen.getByLabelText("ชื่อ")).toBeInTheDocument();
    expect(translate(messages, "title")).toBe("ชื่อ");
  });
  it("still switches when device storage is blocked", () => {
    const read = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
    const save = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("blocked"); });
    render(<LanguageProvider><Form /></LanguageProvider>);
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
    read.mockRestore();
    save.mockRestore();
  });
});
