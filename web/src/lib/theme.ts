import { createContext, useContext } from "react";

export type Theme = "system" | "light" | "dark";

export const THEME_STORAGE_KEY = "rolescout.theme";

export interface ThemeState {
  theme: Theme;
  resolved: "light" | "dark";
  setTheme: (t: Theme) => void;
}

export const ThemeContext = createContext<ThemeState | null>(null);

export function useTheme(): ThemeState {
  const state = useContext(ThemeContext);
  if (!state) throw new Error("useTheme must be used inside ThemeProvider");
  return state;
}

export function storedTheme(): Theme {
  try {
    const v = localStorage.getItem(THEME_STORAGE_KEY);
    if (v === "light" || v === "dark" || v === "system") return v;
  } catch {
    // Storage may be unavailable; follow the system.
  }
  return "system";
}

export function systemPrefersDark(): boolean {
  return typeof window.matchMedia === "function" && window.matchMedia("(prefers-color-scheme: dark)").matches;
}
