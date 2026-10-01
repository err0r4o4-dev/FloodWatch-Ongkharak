import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { AppRoutes } from "./AppRoutes";

describe("AppRoutes", () => {
  it("shows the development placeholder at its bookmarkable route", () => {
    render(
      <MemoryRouter initialEntries={["/system-status"]}>
        <AppRoutes />
      </MemoryRouter>
    );

    expect(screen.getByRole("link", { name: "สถานะระบบ" })).toHaveAttribute("href", "/system-status");
    expect(
      screen.getByRole("heading", { name: "สถานะระบบสำหรับการพัฒนา" })
    ).toBeInTheDocument();
    expect(screen.getByText("ยังไม่มีการเผยแพร่ข้อมูลสถานการณ์น้ำ")).toBeInTheDocument();
  });
});
