import { ArrowRight, Square } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { BackLink } from "../../components/BackLink";
import { Button, ErrorState, PageTitle, Spinner } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import { stages, type ResearchRun, type Stage, type StageProgress } from "../../lib/api";
import { RunSystem } from "./RunSystem";
import { inputClass } from "../../components/styles";
import { useI18n } from "../../lib/i18n";
import { useWorkspace } from "../workspaces/context";
import { isActive, useCancelRun, useProfile, useRun, useRunEvents } from "./api";
import { RunLog } from "./RunLog";
import { StageCard } from "./StageCard";
import { RunStateBadge } from "./RunStateBadge";

const summaryKeys = ["companies_known", "companies_checked", "companies_failed", "jobs_collected", "jobs_open", "matches", "new_matches"] as const;

export function RunPage() {
  const { t, formatDate } = useI18n();
  const { runId = "" } = useParams();
  const { workspace } = useWorkspace();
  const run = useRun(workspace.id, runId);
  const profile = useProfile(workspace.id, run.data?.profile_id ?? "");
  const [errorsOnly, setErrorsOnly] = useState(false);
  const [stageFilter, setStageFilter] = useState<Stage | "">("");
  const active = isActive(run.data);
  const events = useRunEvents(workspace.id, runId, errorsOnly, active);
  const cancel = useCancelRun(workspace.id, runId);

  if (run.isPending) return <Spinner />;
  if (run.isError) return <ErrorState error={run.error} onRetry={() => run.refetch()} />;
  const r = run.data;
  // A run that ended leaves no stage "running" or waiting: show where it stopped.
  const settle = (p: StageProgress): StageProgress => {
    if (active || r.state === "succeeded") return p;
    if (p.state === "running") return { ...p, state: r.state === "failed" ? "failed" : "skipped", note: p.note || t("run.interrupted") };
    if (p.state === "pending") return { ...p, state: "skipped", note: t("run.notReached") };
    return p;
  };

  return (
    <div className="space-y-6">
      <BackLink to="/research">{t("research.title")}</BackLink>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <PageTitle>{profile.data?.name ?? t("run.title")}</PageTitle>
          <div className="mt-2 flex flex-wrap items-center gap-3 text-sm text-ink-muted">
            <RunStateBadge state={r.state} />
            <span>{t(`run.trigger.${r.trigger}`)}</span>
            <Elapsed run={r} />
          </div>
          <dl className="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-xs text-ink-muted">
            <div className="flex gap-1">
              <dt>{t("run.queuedAt")}</dt>
              <dd className="tabular-nums text-ink">{formatDate(r.queued_at, true)}</dd>
            </div>
            {r.started_at && (
              <div className="flex gap-1">
                <dt>{t("run.startedAt")}</dt>
                <dd className="tabular-nums text-ink">{formatDate(r.started_at, true)}</dd>
              </div>
            )}
            {r.finished_at && (
              <div className="flex gap-1">
                <dt>{t("run.finishedAt")}</dt>
                <dd className="tabular-nums text-ink">{formatDate(r.finished_at, true)}</dd>
              </div>
            )}
          </dl>
        </div>
        {active && (
          <Button variant="danger" busy={cancel.isPending || r.cancel_requested} onClick={() => cancel.mutate()}>
            <Square aria-hidden className="size-4" />
            {r.cancel_requested ? t("run.cancelling") : t("run.cancel")}
          </Button>
        )}
      </div>
      {r.error && (
        <p role="alert" className="rounded-lg border border-danger bg-danger-soft px-4 py-3 text-sm text-danger">
          {r.error}
        </p>
      )}

      <RunSystem active={active} />

      {r.state === "succeeded" && <Summary run={r} />}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <div className="space-y-3">
          {stages.map((s, i) => (
            <StageCard key={s} stage={s} index={i + 1} progress={settle(r.stages[s])} runEnd={active ? null : r.finished_at} />
          ))}
        </div>
        <div className="space-y-2">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="font-semibold text-ink">
              {t("run.log")} <span className="text-sm font-normal text-ink-muted">({events.length})</span>
            </h2>
            <select
              aria-label={t("run.logStage")}
              value={stageFilter}
              onChange={(e) => setStageFilter(e.target.value as Stage | "")}
              className={`${inputClass} !w-auto !py-1 text-sm`}
            >
              <option value="">{t("run.logAllStages")}</option>
              {stages.filter((s) => s !== "result").map((s) => (
                <option key={s} value={s}>
                  {t(`run.stage.${s}`)}
                </option>
              ))}
            </select>
            <label className="inline-flex items-center gap-2 text-sm text-ink-muted">
              <input type="checkbox" checked={errorsOnly} onChange={(e) => setErrorsOnly(e.target.checked)} className="accent-[var(--color-ink)]" />
              {t("run.errorsOnly")}
            </label>
          </div>
          <RunLog events={events} stageFilter={stageFilter} />
        </div>
      </div>
    </div>
  );
}

function Summary({ run }: { run: ResearchRun }) {
  const { t } = useI18n();
  return (
    <section aria-label={t("run.summary")} className="rounded-card bg-sage p-4">
      <dl className="grid grid-cols-2 gap-4 sm:grid-cols-4 lg:grid-cols-7">
        {summaryKeys
          .filter((k) => run.summary[k] !== undefined)
          .map((k) => (
            <div key={k}>
              <dt className="text-xs text-ink-muted">{t(`run.sum.${k}`)}</dt>
              <dd className="text-xl font-semibold tabular-nums text-ink">{run.summary[k].toLocaleString()}</dd>
            </div>
          ))}
      </dl>
      <Link to={`/jobs?profile=${run.profile_id}&sort=score`} className={buttonClass("primary", "mt-4")}>
        {t("research.matches")}
        <ArrowRight aria-hidden className="size-4" />
      </Link>
    </section>
  );
}

function Elapsed({ run }: { run: ResearchRun }) {
  const { t } = useI18n();
  const [now, setNow] = useState(Date.now);
  const active = isActive(run);
  useEffect(() => {
    if (!active) return;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [active]);
  if (!run.started_at) return null;
  const end = run.finished_at ? Date.parse(run.finished_at) : now;
  const secs = Math.max(0, Math.round((end - Date.parse(run.started_at)) / 1000));
  return <span className="tabular-nums">{t("run.elapsed", { minutes: Math.floor(secs / 60), seconds: secs % 60 })}</span>;
}
