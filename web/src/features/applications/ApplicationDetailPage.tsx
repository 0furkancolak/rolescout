import { Link, useParams } from "react-router-dom";
import { BackLink } from "../../components/BackLink";
import { NotFound } from "../../components/NotFound";
import { Card, ErrorState, Spinner, StatusBadge } from "../../components/ui";
import { isApiError } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useJob } from "../jobs/api";
import { useWorkspace } from "../workspaces/context";
import { useApplication } from "./api";
import { AttemptsSection, HistorySection, NotesSection, ScheduleSection, StatusControl, TagsSection } from "./sections";

export function ApplicationDetailPage() {
  const { applicationId = "" } = useParams();
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const app = useApplication(workspace.id, applicationId);
  const job = useJob(app.data?.job_id ?? "");

  if (app.isPending) return <Spinner />;
  if (isApiError(app.error, "not_found")) return <NotFound />;
  if (app.isError) return <ErrorState error={app.error} onRetry={() => void app.refetch()} />;

  const props = { ws: workspace.id, application: app.data };
  return (
    <div className="space-y-6">
      <BackLink to="/applications">{t("nav.applications")}</BackLink>
      <Card>
        <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div className="space-y-1">
            <h1 className="text-2xl font-semibold tracking-tight">
              {job.data ? (
                <Link to={`/jobs/${job.data.id}`} className="hover:text-lavender-deep hover:underline">
                  {job.data.title}
                </Link>
              ) : (
                "…"
              )}
            </h1>
            <p className="text-ink-muted">{job.data?.company_name}</p>
            <p className="flex items-center gap-2 text-sm text-ink-muted">
              {t("apps.status")}: <StatusBadge status={app.data.status} />
            </p>
          </div>
          <StatusControl {...props} />
        </div>
      </Card>
      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <NotesSection {...props} />
          <HistorySection {...props} />
        </div>
        <div className="space-y-6">
          <AttemptsSection {...props} />
          <ScheduleSection {...props} />
          <TagsSection {...props} />
        </div>
      </div>
    </div>
  );
}
