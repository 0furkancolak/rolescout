import type { I18n } from "./i18n";
import type { MessageKey } from "./messages";

type Group = "work" | "employment" | "seniority";

// enumLabel translates an API enumeration value, falling back to the raw value
// if the server sends one the UI does not know yet.
export function enumLabel(t: I18n["t"], group: Group, value: string): string {
  const key = `${group}.${value}` as MessageKey;
  const text = t(key);
  return text === key ? value : text;
}

export function hostOf(url: string): string {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}
