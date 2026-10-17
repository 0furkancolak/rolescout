import { useEffect, useMemo, useState, type ReactNode } from "react";
import { storedTheme, systemPrefersDark, THEME_STORAGE_KEY, ThemeContext, type Theme } from "./theme";

// ThemeProvider applies the chosen theme as the "dark" class on <html> and
// follows the system setting while the choice is "system".
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(storedTheme);
  const [systemDark, setSystemDark] = useState(systemPrefersDark);

  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => setSystemDark(mq.matches);
    mq.addEventListener?.("change", onChange);
    return () => mq.removeEventListener?.("change", onChange);
  }, []);

  const resolved = theme === "system" ? (systemDark ? "dark" : "light") : theme;
  useEffect(() => {
    document.documentElement.classList.toggle("dark", resolved === "dark");
  }, [resolved]);

  const value = useMemo(
    () => ({
      theme,
      resolved,
      setTheme: (t: Theme) => {
        setThemeState(t);
        try {
          localStorage.setItem(THEME_STORAGE_KEY, t);
        } catch {
          // Not remembered when storage is unavailable.
        }
      },
    }),
    [theme, resolved],
  );
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
