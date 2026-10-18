import { ArrowLeft, Bookmark, BookmarkCheck, ExternalLink, Send } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { CompanyTile } from "../../components/CompanyTile";
import { Pill } from "../../components/Pill";
import { ScoreRing } from "../../components/ScoreRing";
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

  const facts: Array<[string, string]> = [
    [t("browse.fact.location"), job.location ?? t("jobs.locationUnknown")],
    ...(job.work_arrangement !== "unknown" ? [[t("browse.arrangement"), enumLabel(t, "work", job.work_arrangement)] as [string, string]] : []),
    ...(job.seniority !== "unknown" ? [[t("browse.seniority"), enumLabel(t, "seniority", job.seniority)] as [string, string]] : []),
    ...(job.employment_type !== "unknown" ? [[t("browse.fact.type"), enumLabel(t, "employment", job.employment_type)] as [string, string]] : []),
    [t("browse.fact.posted"), formatDate(job.first_seen_at)],
  ];

  return (
    <section aria-label={job.title} className="rounded-hero bg-surface p-5 shadow-card sm:p-7">
      <button type="button" onClick={onBack} className="mb-3 inline-flex min-h-10 items-center gap-1 text-sm text-ink-muted hover:text-ink lg:hidden">
        <ArrowLeft aria-hidden className="size-4" />
        {t("browse.backToList")}
      </button>
      <div className="flex items-start gap-4">
        <CompanyTile name={job.company_name} size={56} />
        <div className="min-w-0 flex-1">
          <h2 className="text-xl font-bold tracking-tight text-ink">{job.title}</h2>
          <p className="mt-1 text-sm text-ink-muted">{job.company_name}</p>
          {job.closed_at && (
            <div className="mt-2">
              <Pill tone="amber">{t("browse.closedBadge")}</Pill>
            </div>
          )}
        </div>
        {job.match && (
          <div className="flex flex-col items-center gap-1">
            <ScoreRing value={job.match.score} size={88} />
            <span className="text-xs text-ink-muted">{t(job.match.score >= 80 ? "score.band.high" : job.match.score >= 60 ? "score.band.mid" : "score.band.low")}</span>
          </div>
        )}
      </div>
      <dl className="mt-5 grid grid-cols-2 gap-2 sm:grid-cols-3">
        {facts.map(([k, v]) => (
          <div key={k} className="rounded-control bg-canvas px-3 py-2">
            <dt className="text-xs text-ink-muted">{k}</dt>
            <dd className="break-words text-sm font-medium text-ink">{v}</dd>
          </div>
        ))}
      </dl>
      {job.technologies.length > 0 && (
        <ul className="mt-4 flex flex-wrap gap-1.5">
          {job.technologies.map((tech) => (
            <li key={tech} className="rounded-full bg-lavender px-2.5 py-0.5 text-xs text-ink">
              {tech}
            </li>
          ))}
        </ul>
      )}
      <div className="mt-5 flex flex-wrap gap-2">
        <Button variant="secondary" busy={save.isPending} onClick={() => save.mutate({ jobID: job.id, saved: !job.saved })}>
          {job.saved ? <BookmarkCheck aria-hidden className="size-4" /> : <Bookmark aria-hidden className="size-4" />}
          {job.saved ? t("jobs.unsave") : t("jobs.save")}
        </Button>
        <Button busy={apply.isPending} onClick={() => apply.mutate(job.id, { onSuccess: (a) => navigate(`/applications/${a.id}`) })}>
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
        <div className="mt-5 rounded-card bg-canvas p-4">
          <h3 className="mb-2 text-sm font-semibold text-ink">{t("browse.whyMatch")}</h3>
          <MatchReasons reasons={job.match.reasons} />
        </div>
      )}
      <h3 className="mt-6 text-sm font-semibold text-ink">{t("jobs.description")}</h3>
      <div className="mt-2 max-w-prose whitespace-pre-line text-sm leading-relaxed text-ink">
        {job.description || t("jobs.noDescription")}
      </div>
    </section>
  );
}
