import { isApiError } from "../lib/api";
import type { I18n } from "../lib/i18n";

export type Variant = "primary" | "secondary" | "ghost" | "danger";

const variants: Record<Variant, string> = {
  primary: "bg-ink text-ink-inverse hover:opacity-90 disabled:opacity-40",
  secondary: "border border-line-strong bg-surface text-ink hover:bg-canvas disabled:text-ink-muted disabled:opacity-60",
  ghost: "text-ink hover:bg-canvas disabled:text-ink-muted disabled:opacity-60",
  danger: "border border-danger bg-surface text-danger hover:bg-danger-soft disabled:opacity-50",
};

export function buttonClass(variant: Variant = "primary", extra = "") {
  return `inline-flex min-h-10 items-center justify-center gap-2 rounded-control px-4 py-2 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:cursor-not-allowed ${variants[variant]} ${extra}`;
}

export const inputClass =
  "block w-full min-h-10 rounded-control border border-line-strong bg-surface px-3 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-focus focus:outline-2 focus:outline-focus/30 aria-invalid:border-danger";

export function errorMessage(err: unknown, t: I18n["t"]): string {
  if (isApiError(err, "network")) return t("common.networkError");
  if (isApiError(err, "invalid_transition")) return t("apps.transitionError");
  return t("common.error");
}

