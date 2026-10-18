import { MapPin } from "lucide-react";
import { useEffect, useRef } from "react";
import { CompanyTile } from "../../components/CompanyTile";
import { Pill } from "../../components/Pill";
import { ScoreRing } from "../../components/ScoreRing";
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
        className={`w-full rounded-card p-3 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus ${
          active ? "bg-raised shadow-raised outline outline-1 outline-line-strong" : "bg-surface shadow-card hover:bg-raised"
        }`}
      >
        <div className="flex items-start gap-3">
          <CompanyTile name={job.company_name} />
          <div className="min-w-0 flex-1">
            <span className="line-clamp-2 font-semibold text-ink">{job.title}</span>
            <span className="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-sm text-ink-muted">
              <span className="truncate">{job.company_name}</span>
              <span className="inline-flex items-center gap-1">
                <MapPin aria-hidden className="size-3.5" />
                {job.location ?? enumLabel(t, "work", job.work_arrangement)}
              </span>
            </span>
          </div>
          {job.match && <ScoreRing value={job.match.score} />}
        </div>
        <span className="mt-2 flex flex-wrap items-center gap-1.5 pl-[3.25rem] text-xs">
          {job.match?.new && <Pill tone="sage">{t("browse.new")}</Pill>}
          {job.closed_at && <Pill tone="amber">{t("browse.closedBadge")}</Pill>}
          {job.saved && <Pill tone="lavender">{t("jobs.saved")}</Pill>}
          {job.work_arrangement !== "unknown" && <Pill>{enumLabel(t, "work", job.work_arrangement)}</Pill>}
          <span className="text-ink-muted">{formatDate(job.first_seen_at)}</span>
        </span>
      </button>
    </li>
  );
}
