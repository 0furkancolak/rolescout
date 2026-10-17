import type { ReactNode } from "react";
import { Building2, MapPin } from "lucide-react";
import { Link } from "react-router-dom";
import { Chip } from "../../components/ui";
import type { Job } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { enumLabel } from "../../lib/labels";

export function JobMeta({ job }: { job: Job }) {
  const { t } = useI18n();
  return (
    <div className="flex flex-wrap gap-1.5">
      {job.work_arrangement !== "unknown" && <Chip>{enumLabel(t, "work", job.work_arrangement)}</Chip>}
      {job.employment_type !== "unknown" && <Chip>{enumLabel(t, "employment", job.employment_type)}</Chip>}
      {job.seniority !== "unknown" && <Chip>{enumLabel(t, "seniority", job.seniority)}</Chip>}
      {job.technologies.map((tech) => (
        <span key={tech} className="inline-flex rounded-md bg-blue-50 px-2 py-0.5 text-xs text-blue-800">
          {tech}
        </span>
      ))}
    </div>
  );
}

export function JobCard({ job, footer }: { job: Job; footer?: ReactNode }) {
  const { t } = useI18n();
  return (
    <article className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm transition-shadow hover:shadow">
      <h2 className="text-base font-semibold text-slate-900">
        <Link to={`/jobs?job=${job.id}`} className="hover:text-blue-700 hover:underline">
          {job.title}
        </Link>
      </h2>
      <p className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-slate-600">
        <span className="inline-flex items-center gap-1">
          <Building2 aria-hidden className="size-4" />
          {job.company_name}
        </span>
        <span className="inline-flex items-center gap-1">
          <MapPin aria-hidden className="size-4" />
          {job.location ?? t("jobs.locationUnknown")}
        </span>
      </p>
      <div className="mt-3">
        <JobMeta job={job} />
      </div>
      {footer ?? <JobSeen job={job} />}
    </article>
  );
}

function JobSeen({ job }: { job: Job }) {
  const { t, formatDate } = useI18n();
  return <p className="mt-3 text-xs text-slate-500">{t("jobs.lastSeen", { date: formatDate(job.last_seen_at) })}</p>;
}
