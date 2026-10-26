import { CalendarClock, Pencil, Play, Plus } from "lucide-react";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Button, Card, EmptyState, ErrorState, InlineError, PageTitle } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import { isApiError, type ResearchProfile, type ResearchRun } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useWorkspace } from "../workspaces/context";
import { isActive, useProfiles, useRuns, useStartRun } from "./api";
import { secondsBetween } from "./duration";
import { RunStateBadge } from "./RunStateBadge";
import { SkeletonCards } from "../../components/Skeleton";

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
        <SkeletonCards count={2} className="xl:grid-cols-2" />
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
  const [nowMs] = useState(Date.now);
  const terms = [...profile.title_keywords, ...profile.technologies];
  const nextRunLabel = (() => {
    if (!profile.schedule_enabled) return t("research.autoOff");
    if (active) return t("research.nextAfterRun", { hours: profile.schedule_hours });
    const newest = runs.reduce<string | null>((acc, r) => (acc === null || r.queued_at > acc ? r.queued_at : acc), null);
    const due = newest ? Date.parse(newest) + profile.schedule_hours * 3_600_000 : 0;
    return due <= nowMs ? t("research.nextSoon") : t("research.nextRun", { date: formatDate(new Date(due).toISOString(), true) });
  })();

  return (
    <Card className="transition-shadow hover:shadow-md">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-lg font-semibold text-ink">{profile.name}</h2>
          <p className="text-sm text-ink-muted">
            {profile.last_run_at ? t("research.lastRun", { date: formatDate(profile.last_run_at, true) }) : t("research.never")}
          </p>
          <p className="mt-1 inline-flex items-center gap-1.5 rounded-full bg-canvas px-2.5 py-0.5 text-xs text-ink-muted">
            <CalendarClock aria-hidden className="size-3.5" />
            {nextRunLabel}
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
            <li key={k} className="rounded-md bg-canvas px-2 py-0.5 text-xs text-ink">
              {k}
            </li>
          ))}
        </ul>
      )}
      {start.isError && (
        <div className="mt-3">
          {isApiError(start.error, "conflict") ? (
            <p role="alert" className="text-sm text-danger">{t("research.alreadyRunning")}</p>
          ) : (
            <InlineError error={start.error} />
          )}
        </div>
      )}
      <h3 className="mt-5 text-sm font-semibold text-ink">{t("research.history")}</h3>
      {runs.length === 0 ? (
        <p className="mt-2 text-sm text-ink-muted">{t("research.noRuns")}</p>
      ) : (
        <ul className="mt-2 space-y-1">
          {runs.slice(0, 10).map((r) => {
            const secs = secondsBetween(r.started_at, r.finished_at);
            const counts = (
              [
                ["companies_checked", "research.stat.checked"],
                ["jobs_collected", "research.stat.jobs"],
                ["matches", "research.stat.matches"],
                ["new_matches", "research.stat.new"],
              ] as const
            ).filter(([k]) => r.summary[k] !== undefined);
            return (
              <li key={r.id}>
                <Link
                  to={`/research/runs/${r.id}`}
                  className="block cursor-pointer rounded-control border border-transparent px-3 py-2.5 text-sm transition-colors hover:border-line hover:bg-canvas focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-focus"
                >
                  <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
                    <RunStateBadge state={r.state} />
                    <span className="text-ink">{formatDate(r.queued_at, true)}</span>
                    <span className="text-ink-muted">{t(`run.trigger.${r.trigger}`)}</span>
                    {secs !== null && (
                      <span className="ml-auto tabular-nums text-ink-muted">
                        {t("run.elapsed", { minutes: Math.floor(secs / 60), seconds: secs % 60 })}
                      </span>
                    )}
                  </span>
                  {counts.length > 0 && (
                    <span className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
                      {counts.map(([k, label]) => (
                        <span key={k} className="rounded-md bg-canvas px-2 py-1">
                          <span className="block text-[11px] text-ink-muted">{t(label)}</span>
                          <span className="block text-sm font-semibold tabular-nums text-ink">{r.summary[k].toLocaleString()}</span>
                        </span>
                      ))}
                    </span>
                  )}
                  {r.error && <span className="mt-1 block truncate text-xs text-danger">{r.error}</span>}
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
