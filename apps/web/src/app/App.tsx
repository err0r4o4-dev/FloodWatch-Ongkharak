import { BrowserRouter } from "react-router";
import { AuthProvider } from "../features/auth/AuthProvider";
import { RealtimeProvider } from "../features/realtime/RealtimeProvider";
import { AppRoutes } from "./AppRoutes";

export function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <RealtimeProvider><AppRoutes /></RealtimeProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}
