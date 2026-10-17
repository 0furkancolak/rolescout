import { Building2, MapPin } from "lucide-react";
import { useEffect, useRef } from "react";
import type { WorkspaceJob } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { enumLabel } from "../../lib/labels";

// JobList is the middle column: compact, selectable job rows.
export function JobList({
  jobs,
  selected,
  onSelect,
}: {
  jobs: WorkspaceJob[];
  selected: string;
  onSelect: (id: string) => void;
}) {
  return (
    <ul className="space-y-2">
      {jobs.map((j) => (
        <JobRow key={j.id} job={j} active={j.id === selected} onSelect={() => onSelect(j.id)} />
      ))}
    </ul>
  );
}

function JobRow({ job, active, onSelect }: { job: WorkspaceJob; active: boolean; onSelect: () => void }) {
  const { t, formatDate } = useI18n();
  const ref = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (active) ref.current?.scrollIntoView?.({ block: "nearest" });
  }, [active]);
  return (
    <li>
      <button
        ref={ref}
        type="button"
        onClick={onSelect}
        aria-current={active || undefined}
        className={`w-full rounded-xl border p-3 text-left transition-colors ${
          active ? "border-blue-700 bg-blue-50" : "border-slate-200 bg-white hover:border-slate-300"
        }`}
      >
        <div className="flex items-start gap-3">
          <div className="min-w-0 flex-1">
            <span className="block font-medium text-slate-900">{job.title}</span>
            <span className="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-sm text-slate-600">
              <span className="inline-flex items-center gap-1">
                <Building2 aria-hidden className="size-3.5" />
                {job.company_name}
              </span>
              <span className="inline-flex items-center gap-1">
                <MapPin aria-hidden className="size-3.5" />
                {job.location ?? enumLabel(t, "work", job.work_arrangement)}
              </span>
            </span>
          </div>
          {job.match && (
            <span
              aria-label={t("browse.score", { score: job.match.score })}
              className="grid size-10 shrink-0 place-items-center rounded-lg bg-blue-100 text-sm font-semibold tabular-nums text-blue-800"
            >
              {job.match.score}
            </span>
          )}
        </div>
        <span className="mt-2 flex flex-wrap items-center gap-1.5 text-xs">
          {job.match?.new && <span className="rounded-full bg-green-100 px-2 py-0.5 font-medium text-green-800">{t("browse.new")}</span>}
          {job.closed_at && <span className="rounded-full bg-amber-100 px-2 py-0.5 font-medium text-amber-900">{t("browse.closedBadge")}</span>}
          {job.saved && <span className="rounded-full bg-slate-100 px-2 py-0.5 text-slate-700">{t("jobs.saved")}</span>}
          <span className="text-slate-500">{formatDate(job.first_seen_at)}</span>
        </span>
      </button>
    </li>
  );
}
