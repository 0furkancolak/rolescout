import { useState, type DragEvent } from "react";
import { Link } from "react-router-dom";
import { CompanyTile } from "../../components/CompanyTile";
import { type Application, type ApplicationStatus } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useJob } from "../jobs/api";
import { useWorkspace } from "../workspaces/context";
import { useChangeStatus } from "./api";
import { DeleteApplication, StatusDot, StatusSelect } from "./parts";
import { isClosed, openStatuses } from "./statuses";

// The board has one column per step that is still going and a last one for
// everything that ended (accepted, rejected, withdrawn). Cards move by drag
// and drop between the open columns, or with the card's status menu (which
// also reaches the closing statuses, keyboard and touch included).
type Column = ApplicationStatus | "closed";
const columns: Column[] = [...openStatuses, "closed"];

export function ApplicationsBoard({ applications }: { applications: Application[] }) {
  const { t } = useI18n();
  const [dragging, setDragging] = useState<Application | null>(null);
  const [over, setOver] = useState<Column | null>(null);
  const [moveTo, setMoveTo] = useState<{ app: Application; to: ApplicationStatus } | null>(null);

  const canDrop = (col: Column) => !!dragging && col !== "closed" && dragging.status !== col;

  return (
    <div className="-mx-4 overflow-x-auto px-4 pb-2">
      <div className="grid auto-cols-[minmax(16rem,1fr)] grid-flow-col gap-3">
        {columns.map((col) => {
          const items = applications.filter((a) => (col === "closed" ? isClosed(a.status) : a.status === col));
          return (
            <section
              key={col}
              aria-label={col === "closed" ? t("apps.group.closed") : t(`status.${col}`)}
              onDragOver={(e: DragEvent) => {
                if (canDrop(col)) {
                  e.preventDefault();
                  setOver(col);
                }
              }}
              onDragLeave={() => setOver((o) => (o === col ? null : o))}
              onDrop={(e: DragEvent) => {
                e.preventDefault();
                setOver(null);
                if (dragging && canDrop(col)) setMoveTo({ app: dragging, to: col as ApplicationStatus });
                setDragging(null);
              }}
              className={`flex min-h-48 flex-col rounded-card p-3 transition-colors ${over === col ? "bg-lavender" : "bg-canvas"}`}
            >
              <h2 className="flex items-center justify-between px-1 pb-3 text-sm font-semibold text-ink">
                {col === "closed" ? t("apps.group.closed") : t(`status.${col}`)}
                <span className="text-xs font-normal tabular-nums text-ink-muted">{items.length}</span>
              </h2>
              <ul className="space-y-2">
                {items.map((a) => (
                  <BoardCard
                    key={a.id}
                    application={a}
                    pendingMove={moveTo?.app.id === a.id ? moveTo.to : undefined}
                    onMoved={() => setMoveTo(null)}
                    onDragStart={() => setDragging(a)}
                    onDragEnd={() => setDragging(null)}
                  />
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

function BoardCard({
  application,
  pendingMove,
  onMoved,
  onDragStart,
  onDragEnd,
}: {
  application: Application;
  pendingMove?: ApplicationStatus;
  onMoved: () => void;
  onDragStart: () => void;
  onDragEnd: () => void;
}) {
  const { workspace } = useWorkspace();
  const job = useJob(application.job_id);
  const change = useChangeStatus(workspace.id, application.id);
  const title = job.data?.title ?? "…";

  // A drop on another column asks this card to move.
  const [handled, setHandled] = useState<ApplicationStatus | undefined>();
  if (pendingMove && pendingMove !== handled) {
    setHandled(pendingMove);
    change.mutate(pendingMove, { onSettled: onMoved });
  }

  return (
    <li>
      <article
        aria-label={title}
        draggable
        onDragStart={(e) => {
          e.dataTransfer?.setData("text/plain", application.id);
          onDragStart();
        }}
        onDragEnd={onDragEnd}
        className="cursor-grab rounded-card bg-surface p-3 shadow-card active:cursor-grabbing"
      >
        <div className="flex items-center gap-2.5">
          <CompanyTile name={job.data?.company_name ?? ""} size={32} />
          <div className="min-w-0 flex-1">
            <Link to={`/applications/${application.id}`} className="block truncate font-medium text-ink hover:text-lavender-deep">
              {title}
            </Link>
            <p className="truncate text-sm text-ink-muted">{job.data?.company_name}</p>
          </div>
        </div>
        <div className="mt-2 flex items-center justify-between gap-2">
          {isClosed(application.status) && <StatusDot status={application.status} />}
          <StatusSelect application={application} className="min-w-0 flex-1 text-xs" />
          <DeleteApplication application={application} />
        </div>
      </article>
    </li>
  );
}

