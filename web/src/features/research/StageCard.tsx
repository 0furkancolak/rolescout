import { CheckCircle2, Clock, Loader2, SkipForward, XCircle } from "lucide-react";
import type { Stage, StageProgress } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

const icons = {
  pending: { icon: Clock, className: "text-slate-400" },
  running: { icon: Loader2, className: "animate-spin text-blue-700" },
  done: { icon: CheckCircle2, className: "text-green-700" },
  skipped: { icon: SkipForward, className: "text-slate-500" },
  failed: { icon: XCircle, className: "text-red-700" },
};

const borders = {
  pending: "border-dashed border-slate-300 opacity-70",
  running: "border-blue-700 ring-1 ring-blue-700/20",
  done: "border-slate-200",
  skipped: "border-slate-200",
  failed: "border-red-200",
};

// StageCard shows one research stage: its state, progress and counts.
export function StageCard({ stage, index, progress }: { stage: Stage; index: number; progress: StageProgress }) {
  const { t } = useI18n();
  const title = t(`run.stage.${stage}`);
  const { icon: Icon, className } = icons[progress.state] ?? icons.pending;
  const pct = progress.total > 0 ? Math.min(100, Math.round((progress.done / progress.total) * 100)) : progress.state === "done" ? 100 : 0;
  const detail = Object.entries(progress.detail ?? {}).filter(([, n]) => n > 0);
  const showProgress = stage !== "result" && progress.state !== "pending";

  return (
    <section aria-label={title} className={`rounded-xl border bg-white p-4 shadow-sm ${borders[progress.state] ?? borders.pending}`}>
      <div className="flex items-start gap-3">
        <Icon aria-hidden className={`mt-0.5 size-5 shrink-0 ${className}`} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline justify-between gap-x-3">
            <h2 className="font-semibold text-slate-900">
              {index}. {title}
            </h2>
            <span className="text-sm text-slate-500">{t(`run.stageState.${progress.state}`)}</span>
          </div>
          <p className="text-xs text-slate-500">{progress.note || t(`run.stageHint.${stage}`)}</p>
          {showProgress && (
            <div className="mt-3">
              <div
                role="progressbar"
                aria-label={title}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={pct}
                className="h-1.5 overflow-hidden rounded-full bg-slate-100"
              >
                <div
                  className={`h-full rounded-full transition-all ${progress.state === "failed" ? "bg-red-600" : progress.state === "done" || progress.state === "skipped" ? "bg-green-600" : "bg-blue-600"}`}
                  style={{ width: `${pct}%` }}
                />
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
                {progress.total > 0 && (
                  <span className="font-medium tabular-nums text-slate-800">
                    {t("run.progress", { done: progress.done.toLocaleString(), total: progress.total.toLocaleString() })}
                  </span>
                )}
                {progress.failed > 0 && <span className="text-red-700">{t("run.failedCount", { n: progress.failed })}</span>}
              </div>
            </div>
          )}
          {stage !== "result" && detail.length > 0 && (
            <ul className="mt-2 flex flex-wrap gap-1.5">
              {detail.map(([k, n]) => (
                <li key={k} className="rounded-md bg-slate-100 px-2 py-0.5 text-xs text-slate-700">
                  {detailLabel(t, k)} <span className="font-semibold tabular-nums">{n.toLocaleString()}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </section>
  );
}

function detailLabel(t: ReturnType<typeof useI18n>["t"], key: string): string {
  const known = { jobs: "run.detail.jobs", matches: "run.detail.matches", new: "run.detail.new" } as const;
  return key in known ? t(known[key as keyof typeof known]) : key;
}
