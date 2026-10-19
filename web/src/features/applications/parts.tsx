import { Trash2 } from "lucide-react";
import { useState } from "react";
import { InlineError } from "../../components/ui";
import { applicationStatuses, type Application, type ApplicationStatus } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useWorkspace } from "../workspaces/context";
import { useChangeStatus, useDeleteApplication } from "./api";

const dot: Record<ApplicationStatus, string> = {
  planned: "bg-line-strong",
  applied: "bg-lavender-deep",
  interviewing: "bg-amber-deep",
  offer: "bg-sage-deep",
  accepted: "bg-sage-deep",
  rejected: "bg-danger",
  withdrawn: "bg-line-strong",
};

export function StatusDot({ status }: { status: ApplicationStatus }) {
  return <span aria-hidden className={`inline-block size-2 shrink-0 rounded-full ${dot[status]}`} />;
}

// StatusSelect changes an application's status in place. Any status may be
// chosen, so a mistaken one is simply changed back.
export function StatusSelect({ application, className = "" }: { application: Application; className?: string }) {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const change = useChangeStatus(workspace.id, application.id);
  return (
    <>
      <select
        aria-label={t("apps.changeStatus")}
        value={application.status}
        disabled={change.isPending}
        onChange={(e) => change.mutate(e.target.value as ApplicationStatus)}
        className={`min-h-9 rounded-full border border-line bg-canvas py-1 pl-3 text-sm font-medium text-ink ${className}`}
      >
        {applicationStatuses.map((s) => (
          <option key={s} value={s}>
            {t(`status.${s}`)}
          </option>
        ))}
      </select>
      <InlineError error={change.error} />
    </>
  );
}

// DeleteApplication removes an application after a second click.
export function DeleteApplication({ application, onDeleted }: { application: Application; onDeleted?: () => void }) {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const remove = useDeleteApplication(workspace.id);
  const [asking, setAsking] = useState(false);
  if (!asking) {
    return (
      <button
        type="button"
        onClick={() => setAsking(true)}
        aria-label={t("apps.delete")}
        title={t("apps.delete")}
        className="inline-flex size-9 items-center justify-center rounded-control text-ink-muted hover:bg-danger-soft hover:text-danger"
      >
        <Trash2 aria-hidden className="size-4" />
      </button>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 text-sm">
      <button
        type="button"
        disabled={remove.isPending}
        onClick={() => remove.mutate(application.id, { onSuccess: onDeleted })}
        className="min-h-9 rounded-control bg-danger px-3 font-medium text-ink-inverse disabled:opacity-50"
      >
        {t("apps.deleteConfirm")}
      </button>
      <button type="button" onClick={() => setAsking(false)} className="min-h-9 rounded-control px-3 text-ink-muted hover:bg-canvas">
        {t("common.cancel")}
      </button>
      <InlineError error={remove.error} />
    </span>
  );
}
