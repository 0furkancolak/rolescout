import { Bookmark, BookmarkCheck, ExternalLink, ListPlus } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { BackLink } from "../../components/BackLink";
import { NotFound } from "../../components/NotFound";
import { Button, Card, ErrorState, InlineError, Spinner, StatusBadge } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import { isApiError } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { hostOf } from "../../lib/labels";
import { useApplications, useCreateApplication } from "../applications/api";
import { useWorkspace } from "../workspaces/context";
import { useJob, useSavedJobs, useToggleSaved } from "./api";
import { JobMeta } from "./JobCard";

export function JobDetailPage() {
  const { jobId = "" } = useParams();
  const { t, formatDate } = useI18n();
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const job = useJob(jobId);
  const saved = useSavedJobs(workspace.id);
  const toggleSaved = useToggleSaved(workspace.id);
  const applications = useApplications(workspace.id);
  const createApplication = useCreateApplication(workspace.id);

  if (job.isPending) return <Spinner />;
  if (isApiError(job.error, "not_found")) return <NotFound />;
  if (job.isError) return <ErrorState error={job.error} onRetry={() => void job.refetch()} />;

  const j = job.data;
  const isSaved = saved.data?.some((s) => s.job.id === j.id) ?? false;
  const application = applications.data?.find((a) => a.job_id === j.id);

  return (
    <div className="space-y-6">
      <BackLink to="/jobs">{t("nav.jobs")}</BackLink>
      <Card>
        <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
          <div className="space-y-2">
            <h1 className="text-2xl font-semibold tracking-tight">{j.title}</h1>
            <p className="text-slate-700">
              {j.company_name} · {j.location ?? t("jobs.locationUnknown")}
            </p>
            <JobMeta job={j} />
            <p className="text-xs text-slate-500">
              {t("jobs.firstSeen", { date: formatDate(j.first_seen_at) })} ·{" "}
              {t("jobs.lastSeen", { date: formatDate(j.last_seen_at) })}
            </p>
          </div>
          <div className="flex flex-wrap gap-2 md:flex-col md:items-stretch">
            <Button
              variant="secondary"
              aria-pressed={isSaved}
              disabled={saved.isPending}
              busy={toggleSaved.isPending}
              onClick={() => toggleSaved.mutate({ jobID: j.id, saved: !isSaved })}
            >
              {isSaved ? <BookmarkCheck aria-hidden className="size-4" /> : <Bookmark aria-hidden className="size-4" />}
              {isSaved ? t("jobs.unsave") : t("jobs.save")}
            </Button>
            {application ? (
              <Link to={`/applications/${application.id}`} className={buttonClass("primary")}>
                {t("jobs.viewApplication")} <StatusBadge status={application.status} />
              </Link>
            ) : (
              <Button
                disabled={applications.isPending}
                busy={createApplication.isPending}
                onClick={() =>
                  createApplication.mutate(j.id, { onSuccess: (a) => navigate(`/applications/${a.id}`) })
                }
              >
                <ListPlus aria-hidden className="size-4" />
                {t("jobs.track")}
              </Button>
            )}
          </div>
        </div>
        <div className="mt-2 space-y-1">
          <InlineError error={toggleSaved.error} />
          {isApiError(createApplication.error, "conflict") ? (
            <p role="alert" className="text-sm text-red-700">
              {t("apps.alreadyTracked")}
            </p>
          ) : (
            <InlineError error={createApplication.error} />
          )}
        </div>
      </Card>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <h2 className="mb-3 text-lg font-semibold">{t("jobs.description")}</h2>
          {j.description.trim() ? (
            <p className="whitespace-pre-line text-sm leading-6 text-slate-700">{j.description}</p>
          ) : (
            <p className="text-sm text-slate-500">{t("jobs.noDescription")}</p>
          )}
        </Card>
        <Card>
          <h2 className="mb-3 text-lg font-semibold">{t("jobs.sources")}</h2>
          <ul className="space-y-3">
            {j.listings.map((l) => (
              <li key={l.id} className="space-y-1 text-sm">
                <a
                  href={l.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 font-medium text-blue-700 hover:underline"
                >
                  {t("jobs.openListing", { host: hostOf(l.url) })}
                  <ExternalLink aria-hidden className="size-3.5" />
                </a>
                <p className="text-xs text-slate-500">
                  {l.source_posted_at && <>{t("jobs.posted", { date: formatDate(l.source_posted_at) })} · </>}
                  {l.source_updated_at && <>{t("jobs.updated", { date: formatDate(l.source_updated_at) })} · </>}
                  {t("jobs.ingested", { date: formatDate(l.last_ingested_at) })}
                </p>
              </li>
            ))}
          </ul>
        </Card>
      </div>
    </div>
  );
}
