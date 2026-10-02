import type { ReactNode } from "react";
import { Navigate } from "react-router";
import { useAuth } from "../auth/AuthProvider";

type RequireAdminProps = {
  children: ReactNode;
};

export function RequireAdmin({ children }: RequireAdminProps) {
  const { account, status } = useAuth();

  if (status === "loading") {
    return (
      <div className="page-container">
        <div className="panel loading-state">
          <p>กำลังตรวจสอบสิทธิ์...</p>
        </div>
      </div>
    );
  }

  if (status === "anonymous") {
    return <Navigate to="/login" replace />;
  }

  if (account?.role !== "ADMIN") {
    return (
      <div className="page-container">
        <div className="panel error-state">
          <h1>ไม่มีสิทธิ์เข้าถึง</h1>
          <p>คุณต้องเป็นผู้ดูแลระบบเพื่อเข้าถึงหน้านี้</p>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
