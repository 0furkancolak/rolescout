import { Check } from "lucide-react";
import type { MatchReason } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { enumLabel } from "../../lib/labels";

// MatchReasons lists, in words, why a job matched the profile.
export function MatchReasons({ reasons }: { reasons: MatchReason[] }) {
  const { t } = useI18n();
  return (
    <ul className="space-y-1.5">
      {reasons.map((r, i) => {
        const params = { ...r.params };
        if (r.code === "seniority" && params.value) params.value = enumLabel(t, "seniority", params.value);
        return (
          <li key={i} className="flex items-start gap-2 text-sm text-ink">
            <Check aria-hidden className="mt-0.5 size-4 shrink-0 text-sage-deep" />
            {t(`reason.${r.code}`, params)}
          </li>
        );
      })}
    </ul>
  );
}
