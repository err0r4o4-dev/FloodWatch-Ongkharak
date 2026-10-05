import { derivePublicationStatus } from "../reports/types";
import type { CommunityReport } from "../reports/types";
import type { CommunityReportFeature, Position } from "./types";

export const reportClusterRadiusM = 150;
const earthRadiusM = 6_371_008.8;

function distanceM(first: Position, second: Position): number {
  const toRadians = Math.PI / 180;
  const latitudeDifference = (second[1] - first[1]) * toRadians;
  const longitudeDifference = (second[0] - first[0]) * toRadians;
  const chord = Math.sin(latitudeDifference / 2) ** 2 +
    Math.cos(first[1] * toRadians) * Math.cos(second[1] * toRadians) * Math.sin(longitudeDifference / 2) ** 2;
  return 2 * earthRadiusM * Math.asin(Math.sqrt(Math.min(1, Math.max(0, chord))));
}

export function groupCommunityReports(reports: readonly CommunityReport[]): CommunityReportFeature[] {
  const published = reports.filter((report) => derivePublicationStatus(report.status) === "PUBLISHED" && report.publicationStatus !== "REMOVED" && !(report.type === "ROAD" && report.roadSegmentId));
  const originals = [...new Map(published.map((report) => [report.id, report])).values()]
    .sort((first, second) => first.id.localeCompare(second.id));
  const groups: CommunityReportFeature[] = [];
  for (const report of originals) {
    const group = groups.find((candidate) => distanceM(candidate.location.coordinates, report.location.coordinates) <= reportClusterRadiusM);
    if (group) {
      group.reports.push(report);
    } else {
      groups.push({
        id: `report-group:${report.id}`, name: report.locationName?.trim() ?? "", location: report.location,
        sourceType: "COMMUNITY", reports: [report]
      });
    }
  }
  return groups;
}

