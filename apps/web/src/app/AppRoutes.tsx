import { Navigate, Route, Routes } from "react-router";
import { AnnouncementsPage } from "../features/announcements/AnnouncementsPage";
import { CCTVPage } from "../features/cctv/CCTVPage";
import { HealthPage } from "../features/health/HealthPage";
import { MapPage } from "../features/map/MapPage";
import { OverviewPage } from "../features/overview/OverviewPage";
import { ReportsPage } from "../features/reports/ReportsPage";
import { AppShell } from "./AppShell";

export function AppRoutes() {
  return <AppShell><Routes>
    <Route path="/" element={<OverviewPage />} />
    <Route path="/announcements" element={<AnnouncementsPage />} />
    <Route path="/map" element={<MapPage />} />
    <Route path="/cctv" element={<CCTVPage />} />
    <Route path="/reports" element={<ReportsPage />} />
    <Route path="/system-status" element={<HealthPage />} />
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes></AppShell>;
}
