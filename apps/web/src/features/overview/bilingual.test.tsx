import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider, LanguageSwitcher, LANGUAGE_STORAGE_KEY } from "../../i18n";
import { OverviewPage } from "./OverviewPage";

function renderOverview() {
  return render(<LanguageProvider><MemoryRouter><OverviewPage /><LanguageSwitcher /></MemoryRouter></LanguageProvider>);
}

beforeEach(() => localStorage.setItem(LANGUAGE_STORAGE_KEY, "th"));
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); localStorage.removeItem(LANGUAGE_STORAGE_KEY); });

describe("overview bilingual states", () => {
  it("keeps the loading state localized when the language changes", async () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise(() => undefined)));
    renderOverview();
    expect(screen.getByRole("status", { name: "กำลังโหลดข้อมูล" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    await waitFor(() => expect(screen.getByRole("status", { name: "Loading data" })).toBeInTheDocument());
  });

  it("localizes empty and error evidence without changing the honest state", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [] }) }));
    const view = renderOverview();
    await screen.findByText("ยังไม่มีเหตุการณ์ในรายการ");
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    await waitFor(() => {
      expect(screen.getByRole("region", { name: "Reported flood events" })).toHaveTextContent("No events in the loaded list");
      expect(screen.getByText(/Old or unavailable data is shown as unknown/)).toBeInTheDocument();
    });

    view.unmount();
    vi.unstubAllGlobals();
    localStorage.setItem(LANGUAGE_STORAGE_KEY, "th");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
    renderOverview();
    await waitFor(() => expect(screen.getAllByRole("alert").length).toBeGreaterThan(0));
    expect(screen.getByText("ไม่สามารถโหลดข้อมูลสถานการณ์ได้")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    await waitFor(() => expect(screen.getByText("Could not load situation data")).toBeInTheDocument());
  });
});
