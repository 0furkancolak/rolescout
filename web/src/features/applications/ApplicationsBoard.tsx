import { ChevronLeft, ChevronRight } from "lucide-react";
import { Link } from "react-router-dom";
import { CompanyTile } from "../../components/CompanyTile";
import { InlineError } from "../../components/ui";
import { type Application, type ApplicationStatus } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useJob } from "../jobs/api";
import { useWorkspace } from "../workspaces/context";
import { useChangeStatus } from "./api";
import { DeleteApplication, StatusSelect } from "./parts";
import { isClosed, openStatuses } from "./statuses";

// The board has one column per step that is still going and a last one for
// everything that ended (accepted, rejected, withdrawn). A card moves one step
// back or forward with the arrow buttons on it, or to any status with its menu
// (the only way to the closing ones). Both work with keyboard and touch.
type Column = ApplicationStatus | "closed";
const columns: Column[] = [...openStatuses, "closed"];

export function ApplicationsBoard({ applications }: { applications: Application[] }) {
  const { t } = useI18n();
  return (
    <div className="-mx-4 overflow-x-auto px-4 pb-2">
      <div className="grid auto-cols-[minmax(16rem,1fr)] grid-flow-col gap-3">
        {columns.map((col) => {
          const items = applications.filter((a) => (col === "closed" ? isClosed(a.status) : a.status === col));
          return (
            <section key={col} aria-label={col === "closed" ? t("apps.group.closed") : t(`status.${col}`)} className="flex min-h-48 flex-col rounded-card bg-canvas p-3">
              <h2 className="flex items-center justify-between px-1 pb-3 text-sm font-semibold text-ink">
                {col === "closed" ? t("apps.group.closed") : t(`status.${col}`)}
                <span className="rounded-full bg-surface px-2 py-0.5 text-xs font-normal tabular-nums text-ink-muted">{items.length}</span>
              </h2>
              <ul className="space-y-2">
                {items.map((a) => (
                  <BoardCard key={a.id} application={a} />
                ))}
              </ul>
              {items.length === 0 && <p className="px-1 py-4 text-center text-xs text-ink-muted">{t("board.empty")}</p>}
            </section>
          );
        })}
      </div>
    </div>
  );
}

const stepButton =
  "inline-flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-control border border-line text-ink-muted transition-colors hover:border-line-strong hover:bg-canvas hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:cursor-not-allowed disabled:opacity-40";

function BoardCard({ application }: { application: Application }) {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const job = useJob(application.job_id);
  const change = useChangeStatus(workspace.id, application.id);
  const title = job.data?.title ?? "…";
  const at = openStatuses.indexOf(application.status);
  const prev = at > 0 ? openStatuses[at - 1] : undefined;
  const next = at >= 0 && at < openStatuses.length - 1 ? openStatuses[at + 1] : undefined;

  return (
    <li>
      <article
        aria-label={title}
        className="rounded-card border border-transparent bg-surface p-3 shadow-card transition-shadow hover:border-line-strong hover:shadow-md focus-within:border-line-strong"
      >
        <div className="flex items-center gap-2.5">
          <CompanyTile name={job.data?.company_name ?? ""} size={32} />
          <div className="min-w-0 flex-1">
            <Link to={`/applications/${application.id}`} className="block truncate font-medium text-ink hover:text-lavender-deep focus-visible:outline-2 focus-visible:outline-focus">
              {title}
            </Link>
            <p className="truncate text-sm text-ink-muted">{job.data?.company_name}</p>
          </div>
        </div>
        <div className="mt-2 flex flex-wrap items-center gap-1.5">
          <button
            type="button"
            className={stepButton}
            disabled={!prev || change.isPending}
            aria-label={prev ? t("board.moveBack", { status: t(`status.${prev}`) }) : t("board.moveBack", { status: "—" })}
            title={prev ? t(`status.${prev}`) : undefined}
            onClick={() => prev && change.mutate(prev)}
          >
            <ChevronLeft aria-hidden className="size-4" />
          </button>
                    <StatusSelect application={application} className="order-first w-full basis-full text-xs" />
          <button
            type="button"
            className={stepButton}
            disabled={!next || change.isPending}
            aria-label={next ? t("board.moveForward", { status: t(`status.${next}`) }) : t("board.moveForward", { status: "—" })}
            title={next ? t(`status.${next}`) : undefined}
            onClick={() => next && change.mutate(next)}
          >
            <ChevronRight aria-hidden className="size-4" />
          </button>
          <span className="ml-auto">
            <DeleteApplication application={application} />
          </span>
        </div>
        <InlineError error={change.error} />
      </article>
    </li>
  );
}
