import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsPage } from "./SettingsPage";
import { adminApi } from "./api";
import { notifyError, notifySuccess } from "../../shared/alerts";

vi.mock("./api", () => ({ adminApi: { listSettings: vi.fn(), updateSetting: vi.fn() } }));
vi.mock("../../shared/alerts", () => ({ notifyError: vi.fn(), notifySuccess: vi.fn() }));
const setting = { key: "alert.threshold", value: 10, valueType: "number" as const, category: "alert" as const, description: "ค่าแจ้งเตือน", editable: true, createdAt: "2026-10-02T00:00:00Z", updatedAt: "2026-10-02T00:00:00Z", updatedBy: null };

describe("setting save notifications", () => {
  beforeEach(() => { vi.clearAllMocks(); vi.mocked(adminApi.listSettings).mockResolvedValue({ settings: [setting] }); vi.mocked(adminApi.updateSetting).mockResolvedValue(undefined); });
  async function edit(value: string) {
    render(<MemoryRouter><SettingsPage /></MemoryRouter>);
    fireEvent.click(await screen.findByRole("button", { name: "แก้ไข" }));
    fireEvent.change(screen.getByPlaceholderText("ใส่ค่าในรูปแบบ JSON"), { target: { value } });
    fireEvent.click(screen.getByRole("button", { name: "บันทึก" }));
  }

  it("notifies after saving the parsed setting", async () => {
    await edit("20");
    await waitFor(() => expect(notifySuccess).toHaveBeenCalledTimes(1));
    expect(adminApi.updateSetting).toHaveBeenCalledWith(setting.key, 20);
  });

  it("keeps JSON validation inline without a popup or request", async () => {
    await edit("not-json");
    expect(adminApi.updateSetting).not.toHaveBeenCalled();
    expect(notifyError).not.toHaveBeenCalled();
    expect(notifySuccess).not.toHaveBeenCalled();
    expect(screen.getByPlaceholderText("ใส่ค่าในรูปแบบ JSON")).toHaveValue("not-json");
  });

  it("reports API failures and retains the unsaved draft", async () => {
    vi.mocked(adminApi.updateSetting).mockRejectedValue(new Error("offline"));
    await edit("20");
    await waitFor(() => expect(notifyError).toHaveBeenCalledTimes(1));
    expect(notifySuccess).not.toHaveBeenCalled();
    expect(screen.getByPlaceholderText("ใส่ค่าในรูปแบบ JSON")).toHaveValue("20");
  });
});
