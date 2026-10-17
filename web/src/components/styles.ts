import { isApiError } from "../lib/api";
import type { I18n } from "../lib/i18n";

export type Variant = "primary" | "secondary" | "ghost" | "danger";

const variants: Record<Variant, string> = {
  primary: "bg-blue-700 text-white hover:bg-blue-800 disabled:bg-blue-300",
  secondary: "border border-slate-300 bg-white text-slate-800 hover:bg-slate-50 disabled:text-slate-400",
  ghost: "text-slate-700 hover:bg-slate-100 disabled:text-slate-400",
  danger: "border border-red-200 bg-white text-red-700 hover:bg-red-50 disabled:text-red-300",
};

export function buttonClass(variant: Variant = "primary", extra = "") {
  return `inline-flex min-h-10 items-center justify-center gap-2 rounded-md px-4 py-2 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700 disabled:cursor-not-allowed ${variants[variant]} ${extra}`;
}

export const inputClass =
  "block w-full min-h-10 rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm placeholder:text-slate-400 focus:border-blue-700 focus:outline-2 focus:outline-blue-700/30 aria-invalid:border-red-500";

export function errorMessage(err: unknown, t: I18n["t"]): string {
  if (isApiError(err, "network")) return t("common.networkError");
  if (isApiError(err, "invalid_transition")) return t("apps.transitionError");
  return t("common.error");
}

