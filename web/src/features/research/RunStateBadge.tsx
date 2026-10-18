import type { RunState } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

const colors: Record<RunState, string> = {
  queued: "bg-canvas text-ink",
  running: "bg-lavender text-ink",
  succeeded: "bg-sage text-ink",
  failed: "bg-danger-soft text-danger",
  cancelled: "bg-amber text-ink",
};

export function RunStateBadge({ state }: { state: RunState }) {
  const { t } = useI18n();
  return <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${colors[state]}`}>{t(`run.state.${state}`)}</span>;
}
