import { useState, type DragEvent } from "react";
import { Link } from "react-router-dom";
import { InlineError } from "../../components/ui";
import { applicationStatuses, statusTransitions, type Application, type ApplicationStatus } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useJob } from "../jobs/api";
import { useWorkspace } from "../workspaces/context";
import { useChangeStatus } from "./api";

// ApplicationsBoard shows applications as a kanban board. Cards move by drag
// and drop, or with each card's "Move to…" menu (keyboard and touch).
export function ApplicationsBoard({ applications }: { applications: Application[] }) {
  const { t } = useI18n();
  const [dragging, setDragging] = useState<Application | null>(null);
  const [over, setOver] = useState<ApplicationStatus | null>(null);
  const [moveTo, setMoveTo] = useState<{ app: Application; to: ApplicationStatus } | null>(null);

  const allowed = (app: Application | null, to: ApplicationStatus) => !!app && statusTransitions[app.status].includes(to);

  return (
    <div className="-mx-4 overflow-x-auto px-4 pb-2">
      <div className="grid auto-cols-[minmax(15rem,1fr)] grid-flow-col gap-3">
        {applicationStatuses.map((status) => {
          const items = applications.filter((a) => a.status === status);
          const canDrop = allowed(dragging, status);
          return (
            <section
              key={status}
              aria-label={t(`status.${status}`)}
              onDragOver={(e: DragEvent) => {
                if (canDrop) {
                  e.preventDefault();
                  setOver(status);
                }
              }}
              onDragLeave={() => setOver((o) => (o === status ? null : o))}
              onDrop={(e: DragEvent) => {
                e.preventDefault();
                setOver(null);
                if (dragging && canDrop) setMoveTo({ app: dragging, to: status });
                setDragging(null);
              }}
              className={`flex min-h-48 flex-col rounded-xl border p-2 transition-colors ${
                over === status
                  ? "border-blue-700 bg-blue-50"
                  : dragging && !canDrop && dragging.status !== status
                    ? "border-slate-200 bg-slate-100 opacity-60"
                    : "border-slate-200 bg-slate-50"
              }`}
            >
              <h2 className="flex items-center justify-between px-1 pb-2 text-sm font-semibold text-slate-700">
                {t(`status.${status}`)}
                <span className="rounded-full bg-white px-2 text-xs tabular-nums text-slate-500">{items.length}</span>
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
              {items.length === 0 && <p className="px-1 py-4 text-center text-xs text-slate-400">{t("board.empty")}</p>}
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
  const { t } = useI18n();
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
        className="cursor-grab rounded-lg border border-slate-200 bg-white p-3 shadow-sm active:cursor-grabbing"
      >
        <Link to={`/applications/${application.id}`} className="block font-medium text-slate-900 hover:text-blue-700">
          {title}
        </Link>
        <p className="truncate text-sm text-slate-600">{job.data?.company_name}</p>
        <label className="mt-2 block">
          <span className="sr-only">{t("board.moveTo")}</span>
          <select
            aria-label={t("board.moveTo")}
            value=""
            disabled={change.isPending || statusTransitions[application.status].length === 0}
            onChange={(e) => e.target.value && change.mutate(e.target.value as ApplicationStatus)}
            className="w-full rounded-md border border-slate-300 bg-white px-2 py-1 text-xs text-slate-700"
          >
            <option value="">{t("board.moveTo")}</option>
            {statusTransitions[application.status].map((s) => (
              <option key={s} value={s}>
                {t(`status.${s}`)}
              </option>
            ))}
          </select>
        </label>
        <div className="mt-1">
          <InlineError error={change.error} />
        </div>
      </article>
    </li>
  );
}
