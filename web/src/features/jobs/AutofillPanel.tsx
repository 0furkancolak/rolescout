import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { autofillFields, bookmarklet, type AutofillProfile } from "../../lib/autofill";
import { useI18n } from "../../lib/i18n";
import { useUserState } from "../state/api";
import { useWorkspace } from "../workspaces/context";

// AutofillPanel offers the saved application details for the career page the
// user is about to fill in: a bookmarklet that fills the page, and a copy
// button for every field.
export function AutofillPanel() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const state = useUserState<AutofillProfile>(workspace.id, "autofill");
  const [copied, setCopied] = useState("");
  const profile = state.data ?? null;
  const filled = profile ? autofillFields.filter((f) => profile[f]?.trim()) : [];

  if (state.isPending) return null;
  if (filled.length === 0) {
    return (
      <p className="rounded-card bg-canvas p-3 text-sm text-ink-muted">
        {t("autofill.empty")}{" "}
        <Link to="/autofill" className="font-medium text-ink underline">
          {t("autofill.setup")}
        </Link>
      </p>
    );
  }
  const copy = (field: string, value: string) => {
    void navigator.clipboard?.writeText(value).then(() => {
      setCopied(field);
      setTimeout(() => setCopied((c) => (c === field ? "" : c)), 1500);
    });
  };
  return (
    <div className="space-y-3 rounded-card bg-canvas p-3 text-sm">
      <p className="text-ink-muted">{t("autofill.howTo")}</p>
      <a
        // React refuses javascript: URLs as a prop, so the address is set on the element itself.
        ref={(el) => el?.setAttribute("href", bookmarklet(profile!))}
        onClick={(e) => e.preventDefault()}
        draggable
        className="inline-flex min-h-10 cursor-grab items-center rounded-control border border-dashed border-line-strong bg-surface px-3 font-medium text-ink"
      >
        {t("autofill.bookmarklet")}
      </a>
      <ul className="grid gap-1.5 sm:grid-cols-2">
        {filled.map((f) => (
          <li key={f}>
            <button
              type="button"
              onClick={() => copy(f, profile![f]!)}
              className="flex w-full items-center justify-between gap-2 rounded-control bg-surface px-3 py-2 text-left hover:bg-raised"
            >
              <span className="min-w-0">
                <span className="block text-xs text-ink-muted">{t(`autofill.field.${f}`)}</span>
                <span className="block truncate text-ink">{profile![f]}</span>
              </span>
              {copied === f ? <Check aria-label={t("autofill.copied")} className="size-4 shrink-0 text-sage-deep" /> : <Copy aria-hidden className="size-4 shrink-0 text-ink-muted" />}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
