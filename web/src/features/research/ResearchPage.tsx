import { Pencil, Play, Plus } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { Button, Card, EmptyState, ErrorState, InlineError, PageTitle, Spinner } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import { isApiError, type ResearchProfile, type ResearchRun } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useWorkspace } from "../workspaces/context";
import { isActive, useProfiles, useRuns, useStartRun } from "./api";
import { RunStateBadge } from "./RunStateBadge";

export function ResearchPage() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const profiles = useProfiles(workspace.id);
  const runs = useRuns(workspace.id, undefined, 50);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <PageTitle>{t("research.title")}</PageTitle>
        <Link to="/research/profiles/new" className={buttonClass("secondary")}>
          <Plus aria-hidden className="size-4" />
          {t("research.newProfile")}
        </Link>
      </div>
      {profiles.isPending ? (
        <Spinner />
      ) : profiles.isError ? (
        <ErrorState error={profiles.error} onRetry={() => profiles.refetch()} />
      ) : profiles.data.length === 0 ? (
        <EmptyState title={t("research.profiles")} />
      ) : (
        <div className="grid gap-4 xl:grid-cols-2">
          {profiles.data.map((p) => (
            <ProfileCard key={p.id} profile={p} runs={(runs.data ?? []).filter((r) => r.profile_id === p.id)} />
          ))}
        </div>
      )}
    </div>
  );
}

function ProfileCard({ profile, runs }: { profile: ResearchProfile; runs: ResearchRun[] }) {
  const { t, formatDate } = useI18n();
  const { workspace } = useWorkspace();
  const start = useStartRun(workspace.id);
  const navigate = useNavigate();
  const active = runs.find((r) => isActive(r));
  const terms = [...profile.title_keywords, ...profile.technologies];

  return (
    <Card>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-lg font-semibold text-slate-900">{profile.name}</h2>
          <p className="text-sm text-slate-500">
            {profile.last_run_at ? t("research.lastRun", { date: formatDate(profile.last_run_at, true) }) : t("research.never")}
            {" · "}
            {t("research.every", { hours: profile.schedule_hours })}
          </p>
        </div>
        <div className="flex gap-2">
          <Link to={`/research/profiles/${profile.id}`} className={buttonClass("ghost")} aria-label={`${t("research.editProfile")}: ${profile.name}`}>
            <Pencil aria-hidden className="size-4" />
          </Link>
          {active ? (
            <Link to={`/research/runs/${active.id}`} className={buttonClass("secondary")}>
              {t("research.running")}
            </Link>
          ) : (
            <Button
              busy={start.isPending}
              onClick={() => start.mutate(profile.id, { onSuccess: (run) => navigate(`/research/runs/${run.id}`) })}
            >
              <Play aria-hidden className="size-4" />
              {t("research.runNow")}
            </Button>
          )}
        </div>
      </div>
      {terms.length > 0 && (
        <ul className="mt-3 flex flex-wrap gap-1.5">
          {terms.slice(0, 12).map((k) => (
            <li key={k} className="rounded-md bg-slate-100 px-2 py-0.5 text-xs text-slate-700">
              {k}
            </li>
          ))}
        </ul>
      )}
      {start.isError && (
        <div className="mt-3">
          {isApiError(start.error, "conflict") ? (
            <p role="alert" className="text-sm text-red-700">{t("research.alreadyRunning")}</p>
          ) : (
            <InlineError error={start.error} />
          )}
        </div>
      )}
      <h3 className="mt-5 text-sm font-semibold text-slate-700">{t("research.history")}</h3>
      {runs.length === 0 ? (
        <p className="mt-2 text-sm text-slate-500">{t("research.noRuns")}</p>
      ) : (
        <ul className="mt-2 divide-y divide-slate-200">
          {runs.slice(0, 6).map((r) => (
            <li key={r.id}>
              <Link to={`/research/runs/${r.id}`} className="flex flex-wrap items-center gap-x-3 gap-y-1 py-2 text-sm hover:text-blue-700">
                <RunStateBadge state={r.state} />
                <span className="text-slate-600">{formatDate(r.queued_at, true)}</span>
                {r.summary.matches !== undefined && (
                  <span className="ml-auto text-slate-700">
                    {t("run.sum.matches")}: <span className="font-semibold tabular-nums">{r.summary.matches}</span>
                  </span>
                )}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
