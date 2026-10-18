import { useI18n } from "../../lib/i18n";

// AiBadge marks a value that came from the local model rather than from the
// posting's own fields or the rules.
export function AiBadge({ model }: { model: string }) {
  const { t } = useI18n();
  return (
    <span
      role="img"
      aria-label={t("ai.badgeLabel", { model })}
      title={model}
      className="inline-flex shrink-0 items-center rounded-full bg-lavender px-2 text-[0.6875rem] font-semibold leading-5 text-ink"
    >
      {t("ai.badge")}
    </span>
  );
}
