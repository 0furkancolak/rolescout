import type { RunState } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

const colors: Record<RunState, string> = {
  queued: "bg-slate-100 text-slate-700",
  running: "bg-blue-100 text-blue-800",
  succeeded: "bg-green-100 text-green-800",
  failed: "bg-red-100 text-red-800",
  cancelled: "bg-amber-100 text-amber-900",
};

export function RunStateBadge({ state }: { state: RunState }) {
  const { t } = useI18n();
  return <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${colors[state]}`}>{t(`run.state.${state}`)}</span>;
}
