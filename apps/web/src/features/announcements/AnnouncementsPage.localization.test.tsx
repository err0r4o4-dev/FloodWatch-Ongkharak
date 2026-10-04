import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider, LanguageSwitcher } from "../../i18n";
import { AnnouncementsPage } from "./AnnouncementsPage";
import { listPublicAlerts } from "./api";

vi.mock("./api", () => ({ listPublicAlerts: vi.fn() }));

describe("announcements localization", () => {
  beforeEach(() => {
    localStorage.setItem("floodwatch.locale", "th");
    vi.mocked(listPublicAlerts).mockResolvedValue([{
      id: "alert-1", ruleId: "rule-1", severity: "WARNING", title: "ประกาศจากแหล่งข้อมูล", message: "ข้อความประกาศต้นฉบับ",
      sourceType: "OFFICIAL", resourceType: "road", resourceId: "road-1", triggeredAt: "2026-10-02T08:00:00Z", createdAt: "2026-10-02T08:01:00Z"
    }]);
  });

  it("switches UI chrome to English while preserving original announcement content and filter state", async () => {
    render(<LanguageProvider><LanguageSwitcher /><AnnouncementsPage /></LanguageProvider>);

    fireEvent.click(await screen.findByRole("button", { name: "เตือนภัย (1)" }));
    fireEvent.click(screen.getByRole("button", { name: "EN" }));

    expect(await screen.findByRole("heading", { name: "Announcements and alerts" })).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { name: "ประกาศจากแหล่งข้อมูล" })).toHaveLength(2);
    expect(screen.getAllByText("ข้อความประกาศต้นฉบับ")).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Warning (1)" })).toHaveAttribute("aria-pressed", "true");
    expect(listPublicAlerts).toHaveBeenCalledTimes(1);
  });
});
