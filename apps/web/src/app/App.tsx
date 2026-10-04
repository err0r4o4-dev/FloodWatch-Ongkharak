import { BrowserRouter } from "react-router";
import { AuthProvider } from "../features/auth/AuthProvider";
import { RealtimeProvider } from "../features/realtime/RealtimeProvider";
import { AppRoutes } from "./AppRoutes";
import { LanguageProvider } from "../i18n";
import { NotificationLanguageSync } from "../features/notifications/NotificationLanguageSync";

export function App() {
  return (
    <LanguageProvider><BrowserRouter>
      <AuthProvider>
        <NotificationLanguageSync />
        <RealtimeProvider><AppRoutes /></RealtimeProvider>
      </AuthProvider>
    </BrowserRouter></LanguageProvider>
  );
}
