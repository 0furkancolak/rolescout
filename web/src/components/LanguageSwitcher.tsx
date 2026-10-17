import type { Locale } from "../lib/api";
import { useI18n } from "../lib/i18n";

export function LanguageSwitcher() {
  const { locale, setLocale, t } = useI18n();
  return (
    <label className="flex items-center gap-2 text-sm text-slate-600">
      <span className="sr-only">{t("nav.language")}</span>
      <select
        value={locale}
        onChange={(e) => setLocale(e.target.value as Locale)}
        className="rounded-md border border-slate-300 bg-white px-2 py-1 text-sm"
      >
        <option value="tr">Türkçe</option>
        <option value="en">English</option>
      </select>
    </label>
  );
}
