import { useEffect, useMemo, useState, type ReactNode } from "react";
import type { Locale } from "./api";
import { createI18n, detectLocale, I18nContext, LOCALE_STORAGE_KEY } from "./i18n";

export function I18nProvider({ children, initialLocale }: { children: ReactNode; initialLocale?: Locale }) {
  const [locale, setLocaleState] = useState<Locale>(() => initialLocale ?? detectLocale());

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const value = useMemo(
    () =>
      createI18n(locale, (next) => {
        setLocaleState(next);
        try {
          localStorage.setItem(LOCALE_STORAGE_KEY, next);
        } catch {
          // Ignore: the choice still applies for this visit.
        }
      }),
    [locale],
  );
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}
