import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { bookmarklet, type AutofillProfile } from "../../lib/autofill";
import { useI18n } from "../../lib/i18n";

// BookmarkletTools hands over the fill-in bookmark in two ways, because
// dragging a link to the bookmarks bar does not work everywhere (touch screens,
// some browsers, a hidden bar): drag it, or copy its code and paste it as the
// address of a new bookmark.
export function BookmarkletTools({ profile }: { profile: AutofillProfile }) {
  const { t } = useI18n();
  const [copied, setCopied] = useState(false);
  const copy = () => {
    void navigator.clipboard?.writeText(bookmarklet(profile)).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={copy}
          className="inline-flex min-h-10 cursor-pointer items-center gap-2 rounded-control bg-ink px-4 text-sm font-medium text-ink-inverse transition-opacity hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
        >
          {copied ? <Check aria-hidden className="size-4" /> : <Copy aria-hidden className="size-4" />}
          {copied ? t("autofill.copied") : t("autofill.copyCode")}
        </button>
        <a
          // React refuses javascript: URLs as a prop, so the address is set on the element itself.
          ref={(el) => el?.setAttribute("href", bookmarklet(profile))}
          onClick={(e) => e.preventDefault()}
          draggable
          className="inline-flex min-h-10 cursor-grab items-center rounded-control border border-dashed border-line-strong bg-surface px-3 text-sm font-medium text-ink transition-colors hover:bg-canvas focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
        >
          {t("autofill.bookmarklet")}
        </a>
      </div>
      <ol className="list-decimal space-y-1 pl-5 text-sm text-ink-muted">
        <li>{t("autofill.step1")}</li>
        <li>{t("autofill.step2")}</li>
        <li>{t("autofill.step3")}</li>
      </ol>
    </div>
  );
}
