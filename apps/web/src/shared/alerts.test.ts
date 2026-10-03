import { beforeEach, describe, expect, it, vi } from "vitest";
import Swal from "sweetalert2";
import type { SweetAlertOptions, SweetAlertResult } from "sweetalert2";
import type { Mock } from "vitest";
import { confirmAction, notifyError, notifyInfo, notifySuccess } from "./alerts";

vi.mock("sweetalert2", () => ({ default: { fire: vi.fn(), update: vi.fn(), disableButtons: vi.fn(), isLoading: vi.fn(() => false) } }));

const confirmed = { isConfirmed: true, isDenied: false, isDismissed: false };
const canceled = { isConfirmed: false, isDenied: false, isDismissed: true };
const fire = Swal.fire as unknown as Mock<(options: SweetAlertOptions) => Promise<SweetAlertResult>>;

describe("shared alert flow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fire.mockResolvedValue(confirmed);
  });

  it("does not execute an action when canceled and defaults focus to cancel", async () => {
    fire.mockResolvedValueOnce(canceled);
    const action = vi.fn();
    expect(await confirmAction({ title: "Sign out?", text: "Message", confirmText: "Sign out", action, successTitle: "Done" })).toBe(false);
    expect(action).not.toHaveBeenCalled();
    expect(Swal.fire).toHaveBeenCalledWith(expect.objectContaining({ focusCancel: true, reverseButtons: true, showCancelButton: true, titleText: "Sign out?" }));
  });

  it("executes only after confirmation and shows success after the request", async () => {
    const action = vi.fn().mockResolvedValue(undefined);
    fire.mockImplementationOnce(async (options) => {
      const value = await (options as { preConfirm: () => Promise<unknown> }).preConfirm();
      return { ...confirmed, value };
    });
    expect(await confirmAction({ title: "Confirm", text: "Message", confirmText: "Save", action, successTitle: "Saved" })).toBe(true);
    expect(action).toHaveBeenCalledTimes(1);
    expect(Swal.disableButtons).toHaveBeenCalledTimes(1);
    expect(Swal.update).toHaveBeenCalledWith({ titleText: expect.any(String) });
    expect(Swal.fire).toHaveBeenLastCalledWith(expect.objectContaining({ icon: "success", titleText: "Saved" }));
  });

  it("reports failures without showing success or retrying the mutation", async () => {
    const action = vi.fn().mockRejectedValue(new Error("offline"));
    fire.mockImplementationOnce(async (options) => {
      const value = await (options as { preConfirm: () => Promise<unknown> }).preConfirm();
      return { ...confirmed, value };
    });
    expect(await confirmAction({ title: "Confirm", text: "Message", confirmText: "Save", action, successTitle: "Saved", errorText: "Unable to save" })).toBe(false);
    expect(action).toHaveBeenCalledTimes(1);
    expect(Swal.fire).toHaveBeenLastCalledWith(expect.objectContaining({ icon: "error", text: "Unable to save" }));
    expect(fire.mock.calls.some(([options]) => typeof options === "object" && options.icon === "success")).toBe(false);
  });

  it("blocks Escape and backdrop dismissal while a request is pending", async () => {
    await confirmAction({ title: "Confirm", text: "Message", confirmText: "Save", action: vi.fn(), successTitle: "Done" });
    const options = fire.mock.calls[0][0] as { allowEscapeKey: () => boolean; allowOutsideClick: () => boolean };
    vi.mocked(Swal.isLoading).mockReturnValue(true);
    expect(options.allowEscapeKey()).toBe(false);
    expect(options.allowOutsideClick()).toBe(false);
  });

  it("uses text-only content for external messages", async () => {
    await notifyError("<img src=x onerror=alert(1)>", "<script>unsafe</script>");
    expect(Swal.fire).toHaveBeenCalledWith(expect.objectContaining({ titleText: "<img src=x onerror=alert(1)>", text: "<script>unsafe</script>" }));
    expect(Swal.fire).toHaveBeenCalledWith(expect.not.objectContaining({ html: expect.anything() }));
  });

  it("queues notifications instead of replacing an active dialog", async () => {
    let dismiss: () => void = () => undefined;
    fire.mockImplementationOnce(() => new Promise((resolve) => { dismiss = () => resolve(confirmed); }));
    const first = notifySuccess("First");
    const second = notifyInfo("Next");
    await Promise.resolve();
    expect(Swal.fire).toHaveBeenCalledTimes(1);
    dismiss();
    await Promise.all([first, second]);
    expect(Swal.fire).toHaveBeenCalledTimes(2);
    expect(Swal.fire).toHaveBeenLastCalledWith(expect.objectContaining({ titleText: "Next", icon: "info" }));
  });
});
