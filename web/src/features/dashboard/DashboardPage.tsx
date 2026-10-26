import { ArrowRight, Play, Radar } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { CompanyTile } from "../../components/CompanyTile";
import { Pill } from "../../components/Pill";
import { ScoreRing } from "../../components/ScoreRing";
import { Button, Card, InlineError, PageTitle } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import { applicationStatuses, data, query, stages, type ResearchRun, type WorkspaceJob } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useQuery } from "@tanstack/react-query";
import { useApplications } from "../applications/api";
import { isActive, useProfiles, useRuns, useStartRun } from "../research/api";
import { RunStateBadge } from "../research/RunStateBadge";
import { useWorkspace, wsPath } from "../workspaces/context";
import { ActivityCards } from "./ActivityCards";
import { SkeletonRows } from "../../components/Skeleton";

export function DashboardPage() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const profiles = useProfiles(workspace.id);
  const runs = useRuns(workspace.id, undefined, 1);
  const profile = profiles.data?.[0];
  const lastRun = runs.data?.[0];

  return (
    <div className="space-y-6">
      <PageTitle>{t("dash.title")}</PageTitle>
      <ActivityCards profileID={profile?.id} lastRun={lastRun} />
      <div className="grid grid-cols-[minmax(0,1fr)] gap-6 xl:grid-cols-3">
        <div className="space-y-6 xl:col-span-2">
          <LastRunCard profileID={profile?.id} loading={profiles.isPending || runs.isPending} lastRun={lastRun} />
          {profile && <NewMatches profileID={profile.id} />}
        </div>
        <ApplicationCounts />
      </div>
    </div>
  );
}

const segmentColor: Record<string, string> = {
  done: "bg-sage-deep",
  running: "bg-amber-deep",
  failed: "bg-danger",
  skipped: "bg-line-strong",
  pending: "bg-line",
};

function StageStrip({ run }: { run: ResearchRun }) {
  const { t } = useI18n();
  return (
    <ol className="mt-5 grid grid-cols-[repeat(5,minmax(0,1fr))] gap-2">
      {stages.map((s) => {
        const state = run.stages[s]?.state ?? "pending";
        return (
          <li key={s} className="min-w-0">
            <div className={`h-1.5 rounded-full ${segmentColor[state] ?? "bg-line"}`} />
            <p className="mt-1.5 truncate text-xs text-ink-muted">{t(`run.stage.${s}`)}</p>
          </li>
        );
      })}
    </ol>
  );
}

function LastRunCard({ profileID, loading, lastRun }: { profileID?: string; loading: boolean; lastRun?: ResearchRun }) {
  const { t, formatDate } = useI18n();
  const { workspace } = useWorkspace();
  const start = useStartRun(workspace.id);
  const navigate = useNavigate();
  const run = () => profileID && start.mutate(profileID, { onSuccess: (r) => navigate(`/research/runs/${r.id}`) });

  if (loading) {
    return (
      <Card className="rounded-hero!">
        <SkeletonRows rows={3} />
      </Card>
    );
  }
  if (!lastRun) {
    return (
      <Card className="flex flex-col items-start gap-3 rounded-hero! bg-sage/40!">
        <Radar aria-hidden className="size-8 text-sage-deep" />
        <h2 className="text-xl font-bold text-ink">{t("dash.noRun")}</h2>
        <p className="text-sm text-ink-muted">{t("dash.noRunHint")}</p>
        <Button busy={start.isPending} disabled={!profileID} onClick={run}>
          <Play aria-hidden className="size-4" />
          {t("dash.runNow")}
        </Button>
        <InlineError error={start.error} />
      </Card>
    );
  }
  const s = lastRun.summary;
  return (
    <Card className="rounded-hero! sm:p-8!">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium text-ink-muted">{t("dash.lastRun")}</h2>
          <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-ink-muted">
            <RunStateBadge state={lastRun.state} />
            {lastRun.finished_at
              ? t("dash.finished", { date: formatDate(lastRun.finished_at, true) })
              : lastRun.started_at && t("dash.started", { date: formatDate(lastRun.started_at, true) })}
          </div>
        </div>
        <div className="flex gap-2">
          <Link to={`/research/runs/${lastRun.id}`} className={buttonClass("secondary")}>
            {t("dash.viewRun")}
          </Link>
          {!isActive(lastRun) && (
            <Button busy={start.isPending} disabled={!profileID} onClick={run}>
              <Play aria-hidden className="size-4" />
              {t("research.runNow")}
            </Button>
          )}
        </div>
      </div>
      <StageStrip run={lastRun} />
      <dl className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
        {(["companies_known", "jobs_open", "matches", "new_matches"] as const).map((k) => (
          <div key={k} className="rounded-card bg-canvas p-4">
            <dt className="text-xs text-ink-muted">{t(`run.sum.${k}`)}</dt>
            <dd className="mt-1 text-3xl font-bold tabular-nums tracking-tight text-ink">{(s[k] ?? 0).toLocaleString()}</dd>
          </div>
        ))}
      </dl>
      <InlineError error={start.error} />
    </Card>
  );
}

function NewMatches({ profileID }: { profileID: string }) {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const jobs = useQuery({
    queryKey: ["workspaces", workspace.id, "jobs", "top", profileID],
    queryFn: ({ signal }) =>
      data<WorkspaceJob[]>(wsPath(workspace.id, "/jobs") + query({ profile: profileID, sort: "score", limit: "6" }), { signal }),
  });
  return (
    <Card>
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-bold text-ink">{t("dash.newToday")}</h2>
        <Link to={`/jobs?profile=${profileID}&sort=score`} className="inline-flex items-center gap-1 text-sm font-medium text-ink hover:underline">
          {t("dash.seeAll")}
          <ArrowRight aria-hidden className="size-4" />
        </Link>
      </div>
      {jobs.isPending ? (
        <SkeletonRows rows={4} />
      ) : !jobs.data?.length ? (
        <p className="mt-3 text-sm text-ink-muted">{t("dash.noMatches")}</p>
      ) : (
        <ul className="mt-3 space-y-2">
          {jobs.data.map((j) => (
            <li key={j.id} className="flex items-center gap-3 rounded-card bg-canvas p-3">
              <CompanyTile name={j.company_name} />
              <div className="min-w-0 flex-1">
                <Link to={`/jobs?profile=${profileID}&sort=score&job=${j.id}`} className="block truncate font-medium text-ink hover:underline">
                  {j.title}
                </Link>
                <p className="truncate text-sm text-ink-muted">
                  {j.company_name}
                  {j.location ? ` · ${j.location}` : ""}
                </p>
              </div>
              {j.match?.new && <Pill tone="sage">{t("browse.new")}</Pill>}
              {j.match && <ScoreRing value={j.match.score} />}
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

function ApplicationCounts() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const apps = useApplications(workspace.id);
  const counts = new Map<string, number>();
  for (const a of apps.data ?? []) counts.set(a.status, (counts.get(a.status) ?? 0) + 1);
  return (
    <Card>
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-bold text-ink">{t("dash.pipeline")}</h2>
        <Link to="/applications?view=board" className="text-sm font-medium text-ink hover:underline">
          {t("dash.seeAll")}
        </Link>
      </div>
      {apps.isPending ? (
        <SkeletonRows rows={6} />
      ) : (
        <ul className="mt-3 space-y-1">
          {applicationStatuses.map((s) => (
            <li key={s} className="flex items-center justify-between rounded-control px-3 py-2 text-sm hover:bg-canvas">
              <Link to={`/applications?status=${s}`} className="text-ink-muted hover:text-ink">
                {t(`status.${s}`)}
              </Link>
              <span className="font-bold tabular-nums text-ink">{counts.get(s) ?? 0}</span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
