import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider, LanguageSwitcher } from "../../i18n";
import { ReportForm } from "./ReportForm";
import { reportsApi } from "./api";

vi.mock("./ReportLocationPicker", () => ({
  ReportLocationPicker: () => <div aria-label="Location picker" />,
}));
vi.mock("../../shared/alerts", () => ({ notifyError: vi.fn(), notifySuccess: vi.fn(), notifyWarning: vi.fn() }));

describe("report form localization", () => {
  afterEach(() => vi.restoreAllMocks());

  beforeEach(() => {
    localStorage.setItem("floodwatch.locale", "th");
    vi.clearAllMocks();
  });

  it("keeps the report draft when switching the form UI to English", () => {
    const createSpy = vi.spyOn(reportsApi, "create");
    render(
      <LanguageProvider>
        <LanguageSwitcher />
        <ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />
      </LanguageProvider>,
    );

    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "ข้อความจากผู้ใช้" } });
    fireEvent.click(screen.getByRole("button", { name: "EN" }));

    expect(screen.getByRole("heading", { name: "Submit community report" })).toBeInTheDocument();
    expect(screen.getByLabelText("Description")).toHaveValue("ข้อความจากผู้ใช้");
    expect(createSpy).not.toHaveBeenCalled();
  });
});
