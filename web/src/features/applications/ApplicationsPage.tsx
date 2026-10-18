import { ChevronRight } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { CompanyTile } from "../../components/CompanyTile";
import { Segmented } from "../../components/Segmented";
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

  let content;
  if (apps.isPending) content = <Spinner />;
  else if (apps.isError) content = <ErrorState error={apps.error} onRetry={() => void apps.refetch()} />;
  else if (view === "board") content = <ApplicationsBoard applications={apps.data} />;
  else content = <ApplicationList applications={apps.data} status={status} onStatus={(s) => setParams(s ? { status: s } : {})} />;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <PageTitle>{t("apps.title")}</PageTitle>
        <Segmented
          label={t("apps.view")}
          value={view}
          options={[
            { value: "list", label: t("board.list") },
            { value: "board", label: t("board.board") },
          ]}
          onChange={(v) => setParams(v === "board" ? { view: "board" } : {})}
        />
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
                  s === status ? "bg-ink text-ink-inverse" : "bg-surface text-ink-muted ring-1 ring-line hover:bg-canvas"
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
        <ul className="divide-y divide-line overflow-hidden rounded-card bg-surface shadow-card">
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
        className="flex items-center gap-3 px-4 py-3 hover:bg-canvas focus-visible:bg-canvas"
      >
        <CompanyTile name={job.data?.company_name ?? ""} />
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium text-ink">{job.data?.title ?? "…"}</p>
          <p className="truncate text-sm text-ink-muted">{job.data?.company_name}</p>
        </div>
        <div className="flex flex-col items-end gap-1">
          <StatusBadge status={application.status} />
          <span className="text-xs text-ink-muted">{t("apps.updated", { date: formatDate(application.updated_at) })}</span>
        </div>
        <ChevronRight aria-hidden className="size-4 shrink-0 text-ink-muted" />
      </Link>
    </li>
  );
}
