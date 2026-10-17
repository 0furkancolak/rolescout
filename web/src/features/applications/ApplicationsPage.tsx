import { ChevronRight } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { EmptyState, ErrorState, PageTitle, Spinner, StatusBadge } from "../../components/ui";
import { applicationStatuses, type Application, type ApplicationStatus } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useJob } from "../jobs/api";
import { useWorkspace } from "../workspaces/context";
import { useApplications } from "./api";

export function ApplicationsPage() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const [params, setParams] = useSearchParams();
  const raw = params.get("status");
  const status = applicationStatuses.includes(raw as ApplicationStatus) ? (raw as ApplicationStatus) : undefined;
  const apps = useApplications(workspace.id, status);

  const tabs: (ApplicationStatus | undefined)[] = [undefined, ...applicationStatuses];

  return (
    <div className="space-y-6">
      <PageTitle>{t("apps.title")}</PageTitle>
      <nav aria-label={t("apps.filterLabel")} className="-mx-4 overflow-x-auto px-4">
        <ul className="flex gap-1">
          {tabs.map((s) => (
            <li key={s ?? "all"}>
              <button
                type="button"
                aria-current={s === status ? "true" : undefined}
                onClick={() => setParams(s ? { status: s } : {})}
                className={`min-h-10 whitespace-nowrap rounded-full px-3 py-1.5 text-sm font-medium ${
                  s === status ? "bg-slate-900 text-white" : "bg-white text-slate-600 ring-1 ring-slate-200 hover:bg-slate-50"
                }`}
              >
                {s ? t(`status.${s}`) : t("apps.all")}
              </button>
            </li>
          ))}
        </ul>
      </nav>

      {apps.isPending ? (
        <Spinner />
      ) : apps.isError ? (
        <ErrorState error={apps.error} onRetry={() => void apps.refetch()} />
      ) : apps.data.length === 0 ? (
        <EmptyState
          title={status ? t("apps.emptyFiltered") : t("apps.empty")}
          hint={status ? undefined : t("apps.emptyHint")}
        />
      ) : (
        <ul className="divide-y divide-slate-200 overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm">
          {apps.data.map((a) => (
            <ApplicationRow key={a.id} application={a} />
          ))}
        </ul>
      )}
    </div>
  );
}

function ApplicationRow({ application }: { application: Application }) {
  const { t, formatDate } = useI18n();
  // Job details are cached and shared with the job pages.
  const job = useJob(application.job_id);
  return (
    <li>
      <Link
        to={`/applications/${application.id}`}
        className="flex items-center gap-3 px-4 py-3 hover:bg-slate-50 focus-visible:bg-slate-50"
      >
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium text-slate-900">{job.data?.title ?? "…"}</p>
          <p className="truncate text-sm text-slate-600">{job.data?.company_name}</p>
        </div>
        <div className="flex flex-col items-end gap-1">
          <StatusBadge status={application.status} />
          <span className="text-xs text-slate-500">{t("apps.updated", { date: formatDate(application.updated_at) })}</span>
        </div>
        <ChevronRight aria-hidden className="size-4 shrink-0 text-slate-400" />
      </Link>
    </li>
  );
}
