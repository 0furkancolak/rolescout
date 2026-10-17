import { ArrowRight, Play, Radar } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { Button, Card, InlineError, PageTitle, Spinner } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import { applicationStatuses, data, query, type WorkspaceJob } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useQuery } from "@tanstack/react-query";
import { useApplications } from "../applications/api";
import { isActive, useProfiles, useRuns, useStartRun } from "../research/api";
import { RunStateBadge } from "../research/RunStateBadge";
import { useWorkspace, wsPath } from "../workspaces/context";

export function DashboardPage() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const profiles = useProfiles(workspace.id);
  const runs = useRuns(workspace.id, undefined, 1);
  const profile = profiles.data?.[0];

  return (
    <div className="space-y-6">
      <PageTitle>{t("dash.title")}</PageTitle>
      <div className="grid gap-6 xl:grid-cols-3">
        <div className="space-y-6 xl:col-span-2">
          <LastRunCard profileID={profile?.id} loading={profiles.isPending || runs.isPending} lastRun={runs.data?.[0]} />
          {profile && <NewMatches profileID={profile.id} />}
        </div>
        <ApplicationCounts />
      </div>
    </div>
  );
}

function LastRunCard({
  profileID,
  loading,
  lastRun,
}: {
  profileID?: string;
  loading: boolean;
  lastRun?: import("../../lib/api").ResearchRun;
}) {
  const { t, formatDate } = useI18n();
  const { workspace } = useWorkspace();
  const start = useStartRun(workspace.id);
  const navigate = useNavigate();
  const run = () => profileID && start.mutate(profileID, { onSuccess: (r) => navigate(`/research/runs/${r.id}`) });

  if (loading) {
    return (
      <Card>
        <Spinner />
      </Card>
    );
  }
  if (!lastRun) {
    return (
      <Card className="flex flex-col items-start gap-3 bg-gradient-to-br from-blue-50 to-white">
        <Radar aria-hidden className="size-8 text-blue-700" />
        <h2 className="text-lg font-semibold text-slate-900">{t("dash.noRun")}</h2>
        <p className="text-sm text-slate-600">{t("dash.noRunHint")}</p>
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
    <Card>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium text-slate-500">{t("dash.lastRun")}</h2>
          <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-slate-600">
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
      <dl className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-4">
        {(["companies_known", "jobs_open", "matches", "new_matches"] as const).map((k) => (
          <div key={k} className="rounded-lg bg-slate-50 p-3">
            <dt className="text-xs text-slate-500">{t(`run.sum.${k}`)}</dt>
            <dd className="text-2xl font-semibold tabular-nums text-slate-900">{(s[k] ?? 0).toLocaleString()}</dd>
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
        <h2 className="font-semibold text-slate-900">{t("dash.newToday")}</h2>
        <Link to={`/jobs?profile=${profileID}&sort=score`} className="inline-flex items-center gap-1 text-sm text-blue-700 hover:underline">
          {t("dash.seeAll")}
          <ArrowRight aria-hidden className="size-4" />
        </Link>
      </div>
      {jobs.isPending ? (
        <Spinner />
      ) : !jobs.data?.length ? (
        <p className="mt-3 text-sm text-slate-500">{t("dash.noMatches")}</p>
      ) : (
        <ul className="mt-3 divide-y divide-slate-200">
          {jobs.data.map((j) => (
            <li key={j.id} className="flex items-center gap-3 py-2.5">
              <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-blue-50 text-sm font-semibold tabular-nums text-blue-800">
                {j.match?.score ?? "–"}
              </span>
              <div className="min-w-0 flex-1">
                <Link to={`/jobs?profile=${profileID}&sort=score&job=${j.id}`} className="block truncate font-medium text-slate-900 hover:text-blue-700">
                  {j.title}
                </Link>
                <p className="truncate text-sm text-slate-500">
                  {j.company_name}
                  {j.location ? ` · ${j.location}` : ""}
                </p>
              </div>
              {j.match?.new && (
                <span className="rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800">{t("browse.new")}</span>
              )}
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
        <h2 className="font-semibold text-slate-900">{t("dash.pipeline")}</h2>
        <Link to="/applications?view=board" className="text-sm text-blue-700 hover:underline">
          {t("dash.seeAll")}
        </Link>
      </div>
      {apps.isPending ? (
        <Spinner />
      ) : (
        <ul className="mt-3 space-y-1.5">
          {applicationStatuses.map((s) => (
            <li key={s} className="flex items-center justify-between rounded-lg px-2 py-1.5 text-sm hover:bg-slate-50">
              <Link to={`/applications?status=${s}`} className="text-slate-700">
                {t(`status.${s}`)}
              </Link>
              <span className="font-semibold tabular-nums text-slate-900">{counts.get(s) ?? 0}</span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
