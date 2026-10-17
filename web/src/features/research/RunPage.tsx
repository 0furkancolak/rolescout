import { ArrowRight, Square } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { BackLink } from "../../components/BackLink";
import { Button, ErrorState, PageTitle, Spinner } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import { stages, type ResearchRun } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useWorkspace } from "../workspaces/context";
import { isActive, useCancelRun, useProfile, useRun, useRunEvents } from "./api";
import { RunLog } from "./RunLog";
import { StageCard } from "./StageCard";
import { RunStateBadge } from "./RunStateBadge";

const summaryKeys = ["companies_known", "companies_checked", "companies_failed", "jobs_collected", "jobs_open", "matches", "new_matches"] as const;

export function RunPage() {
  const { t } = useI18n();
  const { runId = "" } = useParams();
  const { workspace } = useWorkspace();
  const run = useRun(workspace.id, runId);
  const profile = useProfile(workspace.id, run.data?.profile_id ?? "");
  const [errorsOnly, setErrorsOnly] = useState(false);
  const active = isActive(run.data);
  const events = useRunEvents(workspace.id, runId, errorsOnly, active);
  const cancel = useCancelRun(workspace.id, runId);

  if (run.isPending) return <Spinner />;
  if (run.isError) return <ErrorState error={run.error} onRetry={() => run.refetch()} />;
  const r = run.data;

  return (
    <div className="space-y-6">
      <BackLink to="/research">{t("research.title")}</BackLink>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <PageTitle>{profile.data?.name ?? t("run.title")}</PageTitle>
          <div className="mt-2 flex flex-wrap items-center gap-3 text-sm text-slate-600">
            <RunStateBadge state={r.state} />
            <span>{t(`run.trigger.${r.trigger}`)}</span>
            <Elapsed run={r} />
          </div>
        </div>
        {active && (
          <Button variant="danger" busy={cancel.isPending || r.cancel_requested} onClick={() => cancel.mutate()}>
            <Square aria-hidden className="size-4" />
            {r.cancel_requested ? t("run.cancelling") : t("run.cancel")}
          </Button>
        )}
      </div>
      {r.error && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
          {r.error}
        </p>
      )}

      {r.state === "succeeded" && <Summary run={r} />}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <div className="space-y-3">
          {stages.map((s, i) => (
            <StageCard key={s} stage={s} index={i + 1} progress={r.stages[s]} />
          ))}
        </div>
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <h2 className="font-semibold text-slate-900">{t("run.log")}</h2>
            <label className="inline-flex items-center gap-2 text-sm text-slate-600">
              <input type="checkbox" checked={errorsOnly} onChange={(e) => setErrorsOnly(e.target.checked)} className="accent-blue-700" />
              {t("run.errorsOnly")}
            </label>
          </div>
          <RunLog events={events} />
        </div>
      </div>
    </div>
  );
}

function Summary({ run }: { run: ResearchRun }) {
  const { t } = useI18n();
  return (
    <section aria-label={t("run.summary")} className="rounded-xl border border-green-200 bg-green-50 p-4">
      <dl className="grid grid-cols-2 gap-4 sm:grid-cols-4 lg:grid-cols-7">
        {summaryKeys
          .filter((k) => run.summary[k] !== undefined)
          .map((k) => (
            <div key={k}>
              <dt className="text-xs text-slate-600">{t(`run.sum.${k}`)}</dt>
              <dd className="text-xl font-semibold tabular-nums text-slate-900">{run.summary[k].toLocaleString()}</dd>
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
