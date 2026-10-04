import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";

export type Locale = "th" | "en";
export type MessageParams = Record<string, string | number>;
export type Messages<Key extends string = string> = { th: Record<Key, string>; en: Record<Key, string> };
export const LANGUAGE_STORAGE_KEY = "floodwatch.locale";
let currentLocale: Locale = "th";

function savedLocale(): Locale {
  try { return localStorage.getItem(LANGUAGE_STORAGE_KEY) === "en" ? "en" : "th"; }
  catch { return "th"; }
}

export function getLocale(): Locale { return currentLocale; }

function interpolate(message: string, params: MessageParams = {}): string {
  return message.replace(/\{([a-zA-Z0-9_]+)\}/g, (placeholder, key: string) => Object.hasOwn(params, key) ? String(params[key]) : placeholder);
}

export function translate<Key extends string>(messages: Messages<Key>, key: Key, params?: MessageParams): string {
  return interpolate(messages[currentLocale][key], params);
}

const LanguageContext = createContext({ locale: "th" as Locale, setLocale: (_locale: Locale) => {} });

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [locale, updateLocale] = useState<Locale>(savedLocale);
  currentLocale = locale;
  const setLocale = useCallback((next: Locale) => {
    if (next !== "th" && next !== "en") return;
    currentLocale = next;
    updateLocale(next);
    try { localStorage.setItem(LANGUAGE_STORAGE_KEY, next); } catch {}
  }, []);
  useEffect(() => {
    currentLocale = locale;
    document.documentElement.lang = locale;
    document.title = locale === "th" ? "Flood Watch องครักษ์" : "Flood Watch Ongkharak";
    document.querySelector('meta[name="description"]')?.setAttribute("content", locale === "th" ? "ติดตามสถานการณ์น้ำและการแจ้งเตือน อำเภอองครักษ์ จังหวัดนครนายก" : "Water situation monitoring and alerts for Ongkharak, Nakhon Nayok.");
  }, [locale]);
  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.key === LANGUAGE_STORAGE_KEY) updateLocale(event.newValue === "en" ? "en" : "th");
    };
    window.addEventListener("storage", onStorage);
    return () => { window.removeEventListener("storage", onStorage); currentLocale = "th"; };
  }, []);
  const value = useMemo(() => ({ locale, setLocale }), [locale, setLocale]);
  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
}

export function useLanguage() { return useContext(LanguageContext); }

export function useTranslation<Key extends string>(messages: Messages<Key>) {
  const { locale, setLocale } = useLanguage();
  const t = useCallback((key: Key, params?: MessageParams) => interpolate(messages[locale][key], params), [locale, messages]);
  return { locale, setLocale, t };
}

export function LanguageSwitcher({ className = "locale-toggle" }: { className?: string }) {
  const { locale, setLocale } = useLanguage();
  return <div className={className} role="group" aria-label={locale === "th" ? "ภาษา" : "Language"}>
    {(["th", "en"] as const).map((language) => <button key={language} type="button" aria-pressed={locale === language} onClick={() => setLocale(language)}>{language.toUpperCase()}</button>)}
  </div>;
}

export function formatDate(value: string | Date | null | undefined, locale: Locale, options: Intl.DateTimeFormatOptions = { dateStyle: "medium", timeStyle: "short" }): string {
  if (!value) return "—";
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat(locale === "th" ? "th-TH" : "en-GB", { ...options, timeZone: "Asia/Bangkok" }).format(date);
}

export function formatNumber(value: number, locale: Locale, options?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(locale === "th" ? "th-TH" : "en-GB", options).format(value);
}
