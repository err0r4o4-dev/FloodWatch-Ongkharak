import { Navigate, Route, Routes, useLocation } from "react-router";
import { AnnouncementsPage } from "../features/announcements/AnnouncementsPage";
import { AuthPage } from "../features/auth/AuthPage";
import { GoogleCompletePage, GoogleLinkPage } from "../features/auth/GoogleAuthPages";
import { ForgotPasswordPage } from "../features/auth/ForgotPasswordPage";
import { HealthPage } from "../features/health/HealthPage";
import { MapPage } from "../features/map/MapPage";
import { OverviewPage } from "../features/overview/OverviewPage";
import { ReportsPage } from "../features/reports/ReportsPage";
import { NotificationsPage } from "../features/notifications/NotificationsPage";
import { AdminDashboardPage } from "../features/admin/AdminDashboardPage";
import { UsersPage } from "../features/admin/UsersPage";
import { UserDetailPage } from "../features/admin/UserDetailPage";
import { SettingsPage } from "../features/admin/SettingsPage";
import { AuditLogsPage } from "../features/admin/AuditLogsPage";
import { AIEvidenceReview } from "../features/admin/AIEvidenceReview";
import { RequireAdmin } from "../features/admin/RequireAdmin";
import { AppShell } from "./AppShell";

function LegacyRiskRedirect() {
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  params.set("panel", "risk");
  return <Navigate to={`/map?${params}${location.hash}`} replace />;
}

export function AppRoutes() {
  return <AppShell><Routes>
    <Route path="/" element={<OverviewPage />} />
    <Route path="/announcements" element={<AnnouncementsPage />} />
    <Route path="/notifications" element={<NotificationsPage />} />
    <Route path="/risk" element={<LegacyRiskRedirect />} />
    <Route path="/map" element={<MapPage />} />
    <Route path="/reports" element={<ReportsPage />} />
    <Route path="/login" element={<AuthPage mode="login" />} />
    <Route path="/register" element={<AuthPage mode="register" />} />
    <Route path="/auth/complete" element={<GoogleCompletePage />} />
    <Route path="/auth/google/link" element={<GoogleLinkPage />} />
    <Route path="/forgot-password" element={<ForgotPasswordPage />} />
    <Route path="/system-status" element={<HealthPage />} />
    <Route path="/admin/*" element={<RequireAdmin><Routes>
      <Route index element={<AdminDashboardPage />} />
      <Route path="users" element={<UsersPage />} />
      <Route path="users/:id" element={<UserDetailPage />} />
      <Route path="settings" element={<SettingsPage />} />
      <Route path="audit-logs" element={<AuditLogsPage />} />
      <Route path="ai-evidence" element={<AIEvidenceReview />} />
      <Route path="*" element={<Navigate to="/admin" replace />} />
    </Routes></RequireAdmin>} />
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes></AppShell>;
}
