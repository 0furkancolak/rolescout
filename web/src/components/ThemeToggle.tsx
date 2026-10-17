import { Monitor, Moon, Sun } from "lucide-react";
import { useI18n } from "../lib/i18n";
import { useTheme, type Theme } from "../lib/theme";

const next: Record<Theme, Theme> = { system: "light", light: "dark", dark: "system" };
const icons = { system: Monitor, light: Sun, dark: Moon };

// ThemeToggle cycles system → light → dark.
export function ThemeToggle({ className = "" }: { className?: string }) {
  const { t } = useI18n();
  const { theme, setTheme } = useTheme();
  const Icon = icons[theme];
  const label = t("theme.label", { mode: t(`theme.${theme}`) });
  return (
    <button
      type="button"
      onClick={() => setTheme(next[theme])}
      title={label}
      aria-label={label}
      className={`inline-flex size-10 items-center justify-center rounded-lg text-slate-600 hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-blue-700 ${className}`}
    >
      <Icon aria-hidden className="size-5" />
    </button>
  );
}
