import { ApiError } from "../auth/api";
import type th from "../../i18n/locales/th/admin";

export type AdminKey = keyof typeof th;
export type AdminTranslate = (key: AdminKey, params?: Record<string, string | number>) => string;
export type AdminErrorState = { key: AdminKey; params?: Record<string, string | number> };

export function adminErrorState(error: unknown, fallbackKey: AdminKey): AdminErrorState {
  if (error instanceof ApiError) {
    if (error.status === 401 || error.status === 403) return { key: "errors.forbidden" };
    if (error.status === 404) return { key: "errors.notFound" };
    if (error.status === 409) return { key: "errors.conflict" };

    switch (error.code) {
      case "INVALID_REQUEST":
        return { key: "errors.invalidRequest" };
      case "SETTING_READ_ONLY":
        return { key: "errors.readOnly" };
      case "CONFLICT":
        return { key: "errors.conflict" };
    }
  }

  return { key: fallbackKey };
}

export function adminErrorMessage(error: unknown, t: AdminTranslate, fallbackKey: AdminKey) {
  const message = adminErrorState(error, fallbackKey);
  return t(message.key, message.params);
}

export function formatAdminDate(
  value: string | null | undefined,
  locale: "th" | "en",
  formatDate: (value: string | Date | null | undefined, locale: "th" | "en", options?: Intl.DateTimeFormatOptions) => string,
  options: Intl.DateTimeFormatOptions = {},
) {
  return formatDate(value, locale, options);
}

export function formatAdminNumber(
  value: number,
  locale: "th" | "en",
  formatNumber: (value: number, locale: "th" | "en", options?: Intl.NumberFormatOptions) => string,
) {
  return formatNumber(value, locale);
}
