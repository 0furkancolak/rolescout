import { ArrowLeft, Bookmark, BookmarkCheck, ExternalLink, Send } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { Button, InlineError } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import type { WorkspaceJob } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { enumLabel } from "../../lib/labels";
import { useWorkspace } from "../workspaces/context";
import { useApplyToJob, useJob, useSaveFromBrowser } from "./api";
import { MatchReasons } from "./MatchReasons";

// JobDetailPane shows the selected job beside the list (full screen on
// small screens, with a way back).
export function JobDetailPane({ job, onBack }: { job: WorkspaceJob; onBack: () => void }) {
  const { t, formatDate } = useI18n();
  const { workspace } = useWorkspace();
  const detail = useJob(job.id);
  const save = useSaveFromBrowser(workspace.id);
  const apply = useApplyToJob(workspace.id);
  const navigate = useNavigate();
  const url = detail.data?.listings[0]?.url;

  return (
    <section aria-label={job.title} className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <button type="button" onClick={onBack} className="mb-3 inline-flex items-center gap-1 text-sm text-slate-600 hover:text-slate-900 lg:hidden">
        <ArrowLeft aria-hidden className="size-4" />
        {t("browse.backToList")}
      </button>
      <div className="flex items-start gap-3">
        <div className="min-w-0 flex-1">
          <h2 className="text-xl font-semibold text-slate-900">{job.title}</h2>
          <p className="mt-1 text-sm text-slate-600">
            {job.company_name}
            {job.location ? ` · ${job.location}` : ""}
          </p>
        </div>
        {job.match && (
          <span className="rounded-lg bg-blue-100 px-2.5 py-1 text-sm font-semibold text-blue-800">
            {t("browse.score", { score: job.match.score })}
          </span>
        )}
      </div>
      <div className="mt-3 flex flex-wrap gap-1.5 text-xs">
        {job.closed_at && <span className="rounded-full bg-amber-100 px-2 py-0.5 font-medium text-amber-900">{t("browse.closedBadge")}</span>}
        {job.work_arrangement !== "unknown" && <Badge>{enumLabel(t, "work", job.work_arrangement)}</Badge>}
        {job.employment_type !== "unknown" && <Badge>{enumLabel(t, "employment", job.employment_type)}</Badge>}
        {job.seniority !== "unknown" && <Badge>{enumLabel(t, "seniority", job.seniority)}</Badge>}
        <Badge>{t("jobs.firstSeen", { date: formatDate(job.first_seen_at) })}</Badge>
      </div>
      {job.technologies.length > 0 && (
        <ul className="mt-3 flex flex-wrap gap-1.5">
          {job.technologies.map((tech) => (
            <li key={tech} className="rounded-md bg-blue-50 px-2 py-0.5 text-xs text-blue-800">
              {tech}
            </li>
          ))}
        </ul>
      )}
      <div className="mt-4 flex flex-wrap gap-2">
        <Button
          variant={job.saved ? "secondary" : "primary"}
          busy={save.isPending}
          onClick={() => save.mutate({ jobID: job.id, saved: !job.saved })}
        >
          {job.saved ? <BookmarkCheck aria-hidden className="size-4" /> : <Bookmark aria-hidden className="size-4" />}
          {job.saved ? t("jobs.unsave") : t("jobs.save")}
        </Button>
        <Button variant="secondary" busy={apply.isPending} onClick={() => apply.mutate(job.id, { onSuccess: (a) => navigate(`/applications/${a.id}`) })}>
          <Send aria-hidden className="size-4" />
          {t("browse.applied")}
        </Button>
        {url && (
          <a href={url} target="_blank" rel="noopener noreferrer" className={buttonClass("ghost")}>
            <ExternalLink aria-hidden className="size-4" />
            {t("browse.careerPage")}
          </a>
        )}
      </div>
      <InlineError error={save.error ?? apply.error} />
      {job.match && job.match.reasons.length > 0 && (
        <div className="mt-5 rounded-lg bg-slate-50 p-3">
          <h3 className="mb-2 text-sm font-semibold text-slate-800">{t("browse.whyMatch")}</h3>
          <MatchReasons reasons={job.match.reasons} />
        </div>
      )}
      <h3 className="mt-5 text-sm font-semibold text-slate-800">{t("jobs.description")}</h3>
      <div className="mt-2 max-w-prose whitespace-pre-line text-sm leading-relaxed text-slate-700">
        {job.description || t("jobs.noDescription")}
      </div>
    </section>
  );
}

function Badge({ children }: { children: string }) {
  return <span className="rounded-full bg-slate-100 px-2 py-0.5 text-slate-700">{children}</span>;
}
