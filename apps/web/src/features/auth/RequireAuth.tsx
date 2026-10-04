import type { ReactNode } from "react";
import { useTranslation } from "../../i18n";
import en from "../../i18n/locales/en/auth";
import th from "../../i18n/locales/th/auth";
import { useAuth } from "./AuthProvider";

const authMessages = { th, en };

type RequireAuthProps = {
  children: ReactNode;
};

export function RequireAuth({ children }: RequireAuthProps) {
  const { t } = useTranslation(authMessages);
  const { status } = useAuth();

  if (status === "loading") {
    return <p role="status">{t("checkingSession")}</p>;
  }

  if (status === "anonymous") {
    return <p role="alert">{t("signInRequired")}</p>;
  }

  return <>{children}</>;
}

export function RequireAdmin({ children }: RequireAuthProps) {
  const { t } = useTranslation(authMessages);
  const { account, status } = useAuth();

  if (status === "loading") {
    return <p role="status">{t("checkingSession")}</p>;
  }

  if (status === "anonymous") {
    return <p role="alert">{t("signInRequired")}</p>;
  }

  if (account?.role !== "ADMIN") {
    return <p role="alert">{t("adminForbidden")}</p>;
  }

  return <>{children}</>;
}
