import { Navigate, Route, Routes } from "react-router";
import { HealthPage } from "../features/health/HealthPage";
import { AppShell } from "./AppShell";

export function AppRoutes() {
  return (
    <AppShell>
      <Routes>
        <Route path="/" element={<HealthPage />} />
        <Route path="/system-status" element={<HealthPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AppShell>
  );
}
