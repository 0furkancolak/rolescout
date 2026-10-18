import { Link } from "react-router-dom";
import { Button, EmptyState, ErrorState, InlineError, PageTitle, Spinner } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import { useI18n } from "../../lib/i18n";
import { useSavedJobs, useToggleSaved } from "../jobs/api";
import { JobCard } from "../jobs/JobCard";
import { useWorkspace } from "../workspaces/context";

export function SavedJobsPage() {
  const { t, formatDate } = useI18n();
  const { workspace } = useWorkspace();
  const saved = useSavedJobs(workspace.id);
  const toggle = useToggleSaved(workspace.id);

  return (
    <div className="space-y-6">
      <PageTitle>{t("saved.title")}</PageTitle>
      <InlineError error={toggle.error} />
      {saved.isPending ? (
        <Spinner />
      ) : saved.isError ? (
        <ErrorState error={saved.error} onRetry={() => void saved.refetch()} />
      ) : saved.data.length === 0 ? (
        <EmptyState title={t("saved.empty")} hint={t("saved.emptyHint")}>
          <Link to="/jobs" className={buttonClass("secondary", "mt-2")}>
            {t("common.goHome")}
          </Link>
        </EmptyState>
      ) : (
        <ul className="grid gap-3 md:grid-cols-2">
          {saved.data.map((s) => (
            <li key={s.job.id}>
              <JobCard
                job={s.job}
                footer={
                  <div className="mt-3 flex items-center justify-between gap-2">
                    <p className="text-xs text-ink-muted">{t("saved.savedOn", { date: formatDate(s.saved_at) })}</p>
                    <Button
                      variant="ghost"
                      disabled={toggle.isPending && toggle.variables?.jobID === s.job.id}
                      onClick={() => toggle.mutate({ jobID: s.job.id, saved: false })}
                    >
                      {t("jobs.unsave")}
                    </Button>
                  </div>
                }
              />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
