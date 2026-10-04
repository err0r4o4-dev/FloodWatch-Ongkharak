import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ReportCard } from "./ReportCard";
import type { CommunityReport } from "./types";

vi.mock("./ReportMediaImage", () => ({ ReportMediaImage: ({ media }: { media: { id: string } }) => <span data-testid="report-thumbnail">{media.id}</span> }));

const report: CommunityReport = {
  id: "report-1", userId: "user-1", type: "FLOOD", description: "น้ำขังหน้าตลาด",
  location: { type: "Point", coordinates: [101.1, 14.2] }, estimatedWaterDepthM: null,
  status: "VERIFIED", verificationStatus: "VERIFIED", sourceType: "COMMUNITY",
  createdAt: "2026-10-02T08:00:00Z", updatedAt: "2026-10-02T08:00:00Z",
  verifiedAt: "2026-10-02T09:00:00Z", verifiedBy: null,
  media: [{ id: "first", url: "/api/v1/media/first" }, { id: "second", url: "/api/v1/media/second" }]
};

it("loads only the visible thumbnail and distinguishes the detail action", () => {
  render(<ReportCard report={report} showDetailsControl onShowDetails={vi.fn()} />);
  expect(screen.getAllByTestId("report-thumbnail")).toHaveLength(1);
  expect(screen.getByTestId("report-thumbnail")).toHaveTextContent("first");
  expect(screen.getByRole("button", { name: "ดูรายละเอียด: น้ำท่วม · น้ำขังหน้าตลาด" })).toBeInTheDocument();
});

it("shows user-supplied place labels instead of coordinates", () => {
  render(<ReportCard report={{ ...report, locationName: "อาคารทดสอบ", locationNote: "ฝั่งประตู" }} />);
  expect(screen.getByText(/อาคารทดสอบ/)).toBeInTheDocument();
  expect(screen.getByText(/ฝั่งประตู/)).toBeInTheDocument();
  expect(screen.queryByText(/14\.20000/)).not.toBeInTheDocument();
});

it("does not invent a place name or show coordinates for historical records", () => {
  render(<ReportCard report={report} />);
  expect(screen.getByText(/ไม่ได้ระบุชื่อสถานที่/)).toBeInTheDocument();
  expect(screen.queryByText(/14\.20000/)).not.toBeInTheDocument();
});
