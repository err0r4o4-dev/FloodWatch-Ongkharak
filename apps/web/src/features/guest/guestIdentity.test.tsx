import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { DEFAULT_GUEST_DISPLAY_NAME, GUEST_DISPLAY_NAME_STORAGE_KEY, setGuestDisplayName, useGuestIdentity } from "./identity";

function GuestIdentityFields() {
  const { displayName, setDisplayName } = useGuestIdentity();
  return <label>{"ชื่อที่แสดง"}<input aria-label="ชื่อที่แสดง" value={displayName} onChange={(event) => setDisplayName(event.target.value)} /></label>;
}

describe("guest identity", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    localStorage.clear();
  });

  it("defaults to Guest and reacts to same-tab updates", () => {
    render(<><GuestIdentityFields /><GuestIdentityFields /></>);

    const fields = screen.getAllByRole("textbox", { name: "ชื่อที่แสดง" });
    expect(fields[0]).toHaveValue(DEFAULT_GUEST_DISPLAY_NAME);
    fireEvent.change(fields[0], { target: { value: "  ชุมชนองครักษ์  " } });

    expect(fields[0]).toHaveValue("ชุมชนองครักษ์");
    expect(fields[1]).toHaveValue("ชุมชนองครักษ์");
    expect(localStorage.getItem(GUEST_DISPLAY_NAME_STORAGE_KEY)).toBe("ชุมชนองครักษ์");
  });

  it("trims and limits Unicode code points to 80 characters", () => {
    render(<GuestIdentityFields />);
    const field = screen.getByRole("textbox", { name: "ชื่อที่แสดง" });
    fireEvent.change(field, { target: { value: `  ${"😀".repeat(81)}  ` } });

    expect(field).toHaveValue("😀".repeat(80));
    expect(Array.from((field as HTMLInputElement).value)).toHaveLength(80);
  });

  it("keeps the in-memory identity working when storage is denied", () => {
    const originalSetItem = Storage.prototype.setItem;
    Storage.prototype.setItem = () => { throw new DOMException("blocked", "SecurityError"); };
    try {
      const view = render(<GuestIdentityFields />);
      const field = screen.getByRole("textbox", { name: "ชื่อที่แสดง" });
      fireEvent.change(field, { target: { value: "Offline guest" } });
      expect(field).toHaveValue("Offline guest");
      view.unmount();
      render(<GuestIdentityFields />);
      expect(screen.getByRole("textbox", { name: "ชื่อที่แสดง" })).toHaveValue("Offline guest");
    } finally {
      Storage.prototype.setItem = originalSetItem;
      setGuestDisplayName(DEFAULT_GUEST_DISPLAY_NAME);
    }
  });
});
