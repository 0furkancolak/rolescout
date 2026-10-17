import type { ApplicationEvent } from "../../lib/api";
import type { I18n } from "../../lib/i18n";

// toISODate turns a yyyy-mm-dd input into a timestamp at local noon, so the
// stored UTC value shows as the same day in nearby time zones.
export function toISODate(value: string): string {
  return new Date(`${value}T12:00:00`).toISOString();
}

export function todayInput(now = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

export function eventLabel(e: ApplicationEvent, t: I18n["t"]): string {
  if (e.type === "status_changed" && e.from_status && e.to_status) {
    return t("event.status_changed", { from: t(`status.${e.from_status}`), to: t(`status.${e.to_status}`) });
  }
  return t(`event.${e.type}`);
}
