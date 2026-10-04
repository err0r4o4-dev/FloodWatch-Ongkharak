import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router";
import { useAuth } from "../auth/AuthProvider";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/admin";
import en from "../../i18n/locales/en/admin";

type RequireAdminProps = {
  children: ReactNode;
};

export function safeAdminReturnTo(value: string | null): string {
  const fallback = "/admin";
  if (!value || value.length > 2048 || !value.startsWith("/") || value.startsWith("//")) return fallback;
  try {
    const decoded = decodeURIComponent(value);
    const path = value.split(/[?#]/u)[0];
    const decodedPath = decodeURIComponent(path);
    if (/[\\\u0000-\u001f\u007f-\u009f]/u.test(decoded)) return fallback;
    if (decodedPath.includes("%") || decodedPath !== path || path.includes("//") || path.split("/").some((part) => part === "." || part === "..")) return fallback;
    if (!(path === "/admin" || path.startsWith("/admin/")) || path === "/admin/login") return fallback;
    return value;
  } catch {
    return fallback;
  }
}

export function RequireAdmin({ children }: RequireAdminProps) {
  const { account, status } = useAuth();
  const location = useLocation();
  const { t } = useTranslation({ th, en });

  if (status === "loading") {
    return (
      <div className="page-container">
        <div className="panel loading-state">
          <p>{t("common.loadingPermissions")}</p>
        </div>
      </div>
    );
  }

  if (status === "anonymous") {
    const returnTo = safeAdminReturnTo(`${location.pathname}${location.search}${location.hash}`);
    return <Navigate to={`/admin/login?returnTo=${encodeURIComponent(returnTo)}`} replace />;
  }

  if (account?.role !== "ADMIN") {
    return (
      <div className="page-container">
        <div className="panel error-state">
          <h1>{t("common.accessDenied")}</h1>
          <p>{t("common.adminOnly")}</p>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
