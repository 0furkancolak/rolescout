import { CheckCircle2, Clock, Loader2, SkipForward, XCircle } from "lucide-react";
import type { Stage, StageProgress } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { secondsBetween } from "./duration";

const icons = {
  pending: { icon: Clock, className: "text-ink-muted" },
  running: { icon: Loader2, className: "animate-spin text-lavender-deep" },
  done: { icon: CheckCircle2, className: "text-sage-deep" },
  skipped: { icon: SkipForward, className: "text-ink-muted" },
  failed: { icon: XCircle, className: "text-danger" },
};

const borders = {
  pending: "border border-dashed border-line-strong opacity-70 shadow-none",
  running: "ring-2 ring-ink",
  done: "",
  skipped: "",
  failed: "ring-2 ring-danger",
};

// StageCard shows one research stage: its state, progress and counts.
export function StageCard({ stage, index, progress, runEnd }: { stage: Stage; index: number; progress: StageProgress; runEnd?: string | null }) {
  const { t, formatDate } = useI18n();
  const title = t(`run.stage.${stage}`);
  const secs = secondsBetween(progress.started_at, progress.finished_at ?? runEnd ?? null);
  const { icon: Icon, className } = icons[progress.state] ?? icons.pending;
  const pct = progress.total > 0 ? Math.min(100, Math.round((progress.done / progress.total) * 100)) : progress.state === "done" ? 100 : 0;
  const detail = Object.entries(progress.detail ?? {}).filter(([, n]) => n > 0);
  const showProgress = stage !== "result" && progress.state !== "pending";

  return (
    <section aria-label={title} className={`rounded-card bg-surface p-5 shadow-card ${borders[progress.state] ?? borders.pending}`}>
      <div className="flex items-start gap-3">
        <span className="grid size-10 shrink-0 place-items-center rounded-full bg-canvas">
          <Icon aria-hidden className={`size-5 ${className}`} />
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline justify-between gap-x-3">
            <h2 className="font-semibold text-ink">
              {index}. {title}
            </h2>
            <span className="text-sm text-ink-muted">{t(`run.stageState.${progress.state}`)}</span>
          </div>
          <p className="text-xs text-ink-muted">{progress.note || t(`run.stageHint.${stage}`)}</p>
          {progress.started_at && stage !== "result" && (
            <p className="mt-0.5 text-xs tabular-nums text-ink-muted">
              {t("run.stageStarted", { time: formatDate(progress.started_at, true) })}
              {secs !== null && ` · ${t("run.elapsed", { minutes: Math.floor(secs / 60), seconds: secs % 60 })}`}
            </p>
          )}
          {showProgress && (
            <div className="mt-3">
              <div
                role="progressbar"
                aria-label={title}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={pct}
                className="h-1.5 overflow-hidden rounded-full bg-canvas"
              >
                <div
                  className={`h-full rounded-full transition-all ${progress.state === "failed" ? "bg-danger" : progress.state === "done" || progress.state === "skipped" ? "bg-sage-deep" : "bg-ink"}`}
                  style={{ width: `${pct}%` }}
                />
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
                {progress.total > 0 && (
                  <span className="text-lg font-bold tabular-nums text-ink">
                    {t("run.progress", { done: progress.done.toLocaleString(), total: progress.total.toLocaleString() })}
                  </span>
                )}
                {progress.failed > 0 && <span className="text-danger">{t("run.failedCount", { n: progress.failed })}</span>}
              </div>
            </div>
          )}
          {stage !== "result" && detail.length > 0 && (
            <ul className="mt-2 flex flex-wrap gap-1.5">
              {detail.map(([k, n]) => (
                <li key={k} className="rounded-full bg-canvas px-2.5 py-0.5 text-xs text-ink">
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
