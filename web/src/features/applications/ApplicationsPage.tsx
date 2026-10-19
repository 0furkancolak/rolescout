import { Link, useSearchParams } from "react-router-dom";
import { CompanyTile } from "../../components/CompanyTile";
import { Segmented } from "../../components/Segmented";
import { EmptyState, ErrorState, PageTitle, Spinner } from "../../components/ui";
import type { Application } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useJob } from "../jobs/api";
import { useWorkspace } from "../workspaces/context";
import { useApplications } from "./api";
import { ApplicationsBoard } from "./ApplicationsBoard";
import { DeleteApplication, StatusSelect } from "./parts";
import { isClosed } from "./statuses";

type Group = "all" | "open" | "closed";
const groups: Group[] = ["all", "open", "closed"];

// ApplicationsPage lists the user's applications as a list or a board.
export function ApplicationsPage() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const [params, setParams] = useSearchParams();
  const view = params.get("view") === "board" ? "board" : "list";
  const raw = params.get("group");
  const group: Group = groups.includes(raw as Group) ? (raw as Group) : "all";
  const apps = useApplications(workspace.id);

  const set = (changes: Record<string, string>) => {
    const next = new URLSearchParams(params);
    for (const [k, v] of Object.entries(changes)) {
      if (v) next.set(k, v);
      else next.delete(k);
    }
    setParams(next);
  };

  let content;
  if (apps.isPending) content = <Spinner />;
  else if (apps.isError) content = <ErrorState error={apps.error} onRetry={() => void apps.refetch()} />;
  else if (view === "board") content = <ApplicationsBoard applications={apps.data} />;
  else {
    const shown = apps.data.filter((a) => group === "all" || (group === "closed") === isClosed(a.status));
    content = <ApplicationList applications={shown} filtered={group !== "all"} />;
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <PageTitle>{t("apps.title")}</PageTitle>
        <div className="flex flex-wrap items-center gap-3">
          {view === "list" && (
            <Segmented
              label={t("apps.filterLabel")}
              value={group}
              options={groups.map((g) => ({ value: g, label: t(`apps.group.${g}`) }))}
              onChange={(g) => set({ group: g === "all" ? "" : g })}
            />
          )}
          <Segmented
            label={t("apps.view")}
            value={view}
            options={[
              { value: "list", label: t("board.list") },
              { value: "board", label: t("board.board") },
            ]}
            onChange={(v) => set({ view: v === "board" ? "board" : "" })}
          />
        </div>
      </div>
      {content}
    </div>
  );
}

function ApplicationList({ applications, filtered }: { applications: Application[]; filtered: boolean }) {
  const { t } = useI18n();
  if (applications.length === 0) {
    return <EmptyState title={filtered ? t("apps.emptyFiltered") : t("apps.empty")} hint={filtered ? undefined : t("apps.emptyHint")} />;
  }
  return (
    <ul className="space-y-2">
      {applications.map((a) => (
        <ApplicationRow key={a.id} application={a} />
      ))}
    </ul>
  );
}

function ApplicationRow({ application }: { application: Application }) {
  const { t, formatDate } = useI18n();
  // Job details are cached and shared with the job pages.
  const job = useJob(application.job_id);
  return (
    <li className="flex flex-wrap items-center gap-3 rounded-card bg-surface p-3 shadow-card sm:flex-nowrap">
      <Link to={`/applications/${application.id}`} className="flex min-w-0 flex-1 items-center gap-3 rounded-control focus-visible:outline-2 focus-visible:outline-focus">
        <CompanyTile name={job.data?.company_name ?? ""} />
        <span className="min-w-0">
          <span className="block truncate font-medium text-ink">{job.data?.title ?? "…"}</span>
          <span className="block truncate text-sm text-ink-muted">
            {job.data?.company_name} · {t("apps.updated", { date: formatDate(application.updated_at) })}
          </span>
        </span>
      </Link>
      <StatusSelect application={application} />
      <DeleteApplication application={application} />
    </li>
  );
}
