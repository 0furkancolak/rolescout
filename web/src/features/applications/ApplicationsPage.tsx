import { ChevronRight, Columns3, List } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { EmptyState, ErrorState, PageTitle, Spinner, StatusBadge } from "../../components/ui";
import { applicationStatuses, type Application, type ApplicationStatus } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useJob } from "../jobs/api";
import { useWorkspace } from "../workspaces/context";
import { useApplications } from "./api";
import { ApplicationsBoard } from "./ApplicationsBoard";

export function ApplicationsPage() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const [params, setParams] = useSearchParams();
  const view = params.get("view") === "board" ? "board" : "list";
  const raw = params.get("status");
  const status = applicationStatuses.includes(raw as ApplicationStatus) ? (raw as ApplicationStatus) : undefined;
  const apps = useApplications(workspace.id, view === "board" ? undefined : status);

  const viewButton = (v: "list" | "board", label: string, Icon: typeof List) => (
    <button
      type="button"
      aria-pressed={view === v}
      onClick={() => setParams(v === "board" ? { view: "board" } : {})}
      className={`inline-flex min-h-9 items-center gap-1.5 rounded-md px-3 text-sm font-medium ${
        view === v ? "bg-white text-slate-900 shadow-sm" : "text-slate-600 hover:text-slate-900"
      }`}
    >
      <Icon aria-hidden className="size-4" />
      {label}
    </button>
  );

  let content;
  if (apps.isPending) content = <Spinner />;
  else if (apps.isError) content = <ErrorState error={apps.error} onRetry={() => void apps.refetch()} />;
  else if (view === "board") content = <ApplicationsBoard applications={apps.data} />;
  else content = <ApplicationList applications={apps.data} status={status} onStatus={(s) => setParams(s ? { status: s } : {})} />;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <PageTitle>{t("apps.title")}</PageTitle>
        <div className="inline-flex rounded-lg bg-slate-100 p-1">
          {viewButton("list", t("board.list"), List)}
          {viewButton("board", t("board.board"), Columns3)}
        </div>
      </div>
      {content}
    </div>
  );
}

function ApplicationList({
  applications,
  status,
  onStatus,
}: {
  applications: Application[];
  status?: ApplicationStatus;
  onStatus: (s?: ApplicationStatus) => void;
}) {
  const { t } = useI18n();
  const tabs: (ApplicationStatus | undefined)[] = [undefined, ...applicationStatuses];
  return (
    <>
      <nav aria-label={t("apps.filterLabel")} className="-mx-4 overflow-x-auto px-4">
        <ul className="flex gap-1">
          {tabs.map((s) => (
            <li key={s ?? "all"}>
              <button
                type="button"
                aria-current={s === status ? "true" : undefined}
                onClick={() => onStatus(s)}
                className={`min-h-10 whitespace-nowrap rounded-full px-3 py-1.5 text-sm font-medium ${
                  s === status ? "bg-blue-700 text-white" : "bg-white text-slate-600 ring-1 ring-slate-200 hover:bg-slate-50"
                }`}
              >
                {s ? t(`status.${s}`) : t("apps.all")}
              </button>
            </li>
          ))}
        </ul>
      </nav>
      {applications.length === 0 ? (
        <EmptyState title={status ? t("apps.emptyFiltered") : t("apps.empty")} hint={status ? undefined : t("apps.emptyHint")} />
      ) : (
        <ul className="divide-y divide-slate-200 overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm">
          {applications.map((a) => (
            <ApplicationRow key={a.id} application={a} />
          ))}
        </ul>
      )}
    </>
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
