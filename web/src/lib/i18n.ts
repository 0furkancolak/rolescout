import { createContext, useContext } from "react";
import type { Locale } from "./api";
import { messages, type MessageKey } from "./messages";

export interface I18n {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: MessageKey, vars?: Record<string, string | number>) => string;
  formatDate: (iso: string, withTime?: boolean) => string;
}

export const LOCALE_STORAGE_KEY = "rolescout.locale";

export function detectLocale(): Locale {
  try {
    const stored = localStorage.getItem(LOCALE_STORAGE_KEY);
    if (stored === "tr" || stored === "en") return stored;
  } catch {
    // Storage can be unavailable (private mode); fall back to the browser.
  }
  return navigator.language?.toLowerCase().startsWith("tr") ? "tr" : "en";
}

export function createI18n(locale: Locale, setLocale: (l: Locale) => void): I18n {
  const dict = messages[locale];
  const tag = locale === "tr" ? "tr-TR" : "en-GB";
  const dateFormat = new Intl.DateTimeFormat(tag, { dateStyle: "medium" });
  const dateTimeFormat = new Intl.DateTimeFormat(tag, { dateStyle: "medium", timeStyle: "short" });
  return {
    locale,
    setLocale,
    t(key, vars) {
      let text = dict[key] ?? key;
      if (vars) {
        for (const [name, value] of Object.entries(vars)) {
          text = text.replaceAll(`{${name}}`, String(value));
        }
      }
      return text;
    },
    // Timestamps are stored in UTC and shown in the viewer's time zone.
    formatDate(iso, withTime = false) {
      const d = new Date(iso);
      if (Number.isNaN(d.getTime())) return iso;
      return (withTime ? dateTimeFormat : dateFormat).format(d);
    },
  };
}

export const I18nContext = createContext<I18n>(createI18n("en", () => {}));

export function useI18n(): I18n {
  return useContext(I18nContext);
}
