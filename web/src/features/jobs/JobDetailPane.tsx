import {
  ArrowLeft,
  Bookmark,
  BookmarkCheck,
  Check,
  ChevronDown,
  ExternalLink,
  Send,
  Undo2,
  WandSparkles,
} from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { CompanyTile } from "../../components/CompanyTile";
import { Pill } from "../../components/Pill";
import { ScoreRing } from "../../components/ScoreRing";
import { Button, InlineError } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import type { WorkspaceJob } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { enumLabel } from "../../lib/labels";
import { useApplications, useDeleteApplication } from "../applications/api";
import { useWorkspace } from "../workspaces/context";
import { AutofillPanel } from "./AutofillPanel";
import {
  useApplyToJob,
  useJob,
  useJobSummary,
  useSaveFromBrowser,
} from "./api";
import { AiBadge } from "./AiBadge";
import { MatchReasons } from "./MatchReasons";

// JobDetailPane shows the selected job beside the list. It scrolls on its own
// with the title and actions pinned on top (full screen on small screens,
// with a way back).
export function JobDetailPane({
  job,
  onBack,
}: {
  job: WorkspaceJob;
  onBack: () => void;
}) {
  const { t, locale, formatDate } = useI18n();
  const { workspace } = useWorkspace();
  const detail = useJob(job.id);
  const save = useSaveFromBrowser(workspace.id);
  const apply = useApplyToJob(workspace.id);
  const remove = useDeleteApplication(workspace.id);
  const applications = useApplications(workspace.id);
  const application = applications.data?.find((a) => a.job_id === job.id);
  const summary = useJobSummary(workspace.id, job.id, locale);
  const [fillOpen, setFillOpen] = useState(false);
  const url = detail.data?.listings[0]?.url;

  const ai = job.ai ?? detail.data?.ai ?? null;
  const filled = new Set<string>(ai?.fields ?? []);
  const place = [
    job.company_name,
    job.location ?? t("jobs.locationUnknown"),
    formatDate(job.first_seen_at),
  ];

  // Short facts as pills; those the local model supplied are tinted.
  const facts: Array<{ label: string; fromAI: boolean }> = [
    ...(job.work_arrangement !== "unknown"
      ? [
          {
            label: enumLabel(t, "work", job.work_arrangement),
            fromAI: filled.has("work_arrangement"),
          },
        ]
      : []),
    ...(job.seniority !== "unknown"
      ? [
          {
            label: enumLabel(t, "seniority", job.seniority),
            fromAI: filled.has("seniority"),
          },
        ]
      : []),
    ...(job.employment_type !== "unknown"
      ? [
          {
            label: enumLabel(t, "employment", job.employment_type),
            fromAI: false,
          },
        ]
      : []),
    ...(ai?.salary && (ai.salary.min != null || ai.salary.max != null)
      ? [{ label: salaryText(t, ai.salary), fromAI: true }]
      : []),
  ];

  const aiSummary = summary.data?.source === "ai" ? summary.data.summary : null;
  const writing = summary.isPending || summary.data?.source === "pending";
  const shownSummary =
    aiSummary ??
    (summary.data?.source === "unavailable" ? ai?.summary || null : null);
  const reasons = job.match?.reasons ?? [];
  const explanation = job.match?.explanation ?? null;

  return (
    <section
      aria-label={job.title}
      className="overflow-y-auto rounded-hero bg-surface shadow-card lg:sticky lg:top-4 lg:max-h-[calc(100dvh-2rem)]"
    >
      <header className="sticky top-0 z-10 bg-surface/95 px-5 pb-3 pt-5 backdrop-blur sm:px-6">
        <button
          type="button"
          onClick={onBack}
          className="mb-3 inline-flex min-h-10 items-center gap-1 text-sm text-ink-muted hover:text-ink lg:hidden"
        >
          <ArrowLeft aria-hidden className="size-4" />
          {t("browse.backToList")}
        </button>
        <div className="flex items-start gap-3">
          <CompanyTile name={job.company_name} size={44} />
          <div className="min-w-0 flex-1">
            <h2 className="text-lg font-bold leading-snug tracking-tight text-ink">
              {job.title}
            </h2>
            <p className="mt-0.5 text-sm text-ink-muted">{place.join(" · ")}</p>
            {job.closed_at && (
              <div className="mt-1.5">
                <Pill tone="amber">{t("browse.closedBadge")}</Pill>
              </div>
            )}
          </div>
          {job.match && (
            <div
              className="shrink-0"
              title={t(
                job.match.score >= 80
                  ? "score.band.high"
                  : job.match.score >= 60
                    ? "score.band.mid"
                    : "score.band.low",
              )}
            >
              <ScoreRing value={job.match.score} size={52} />
            </div>
          )}
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {application ? (
            <>
              <Link
                to={`/applications/${application.id}`}
                className={buttonClass("secondary", "text-sage-deep")}
              >
                <Check aria-hidden className="size-4" />
                {t(`status.${application.status}`)}
              </Link>
              <Button
                variant="ghost"
                busy={remove.isPending}
                onClick={() => remove.mutate(application.id)}
              >
                <Undo2 aria-hidden className="size-4" />
                {t("browse.undoApplied")}
              </Button>
            </>
          ) : (
            <Button busy={apply.isPending} onClick={() => apply.mutate(job.id)}>
              <Send aria-hidden className="size-4" />
              {t("browse.applied")}
            </Button>
          )}
          <Button
            variant="secondary"
            busy={save.isPending}
            onClick={() => save.mutate({ jobID: job.id, saved: !job.saved })}
          >
            {job.saved ? (
              <BookmarkCheck aria-hidden className="size-4" />
            ) : (
              <Bookmark aria-hidden className="size-4" />
            )}
            {job.saved ? t("jobs.unsave") : t("jobs.save")}
          </Button>
          <Button
            variant="ghost"
            aria-expanded={fillOpen}
            onClick={() => setFillOpen((o) => !o)}
          >
            <WandSparkles aria-hidden className="size-4" />
            {t("autofill.open")}
          </Button>
          {url && (
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              className={buttonClass("ghost")}
            >
              <ExternalLink aria-hidden className="size-4" />
              {t("browse.careerPage")}
            </a>
          )}
        </div>
        <InlineError error={save.error ?? apply.error ?? remove.error} />
      </header>

      <div className="space-y-5 px-5 pb-6 pt-2 sm:px-6">
        {fillOpen && <AutofillPanel />}
        {facts.length > 0 && (
          <ul className="flex flex-wrap gap-1.5">
            {facts.map((f) => (
              <li key={f.label}>
                <Pill tone={f.fromAI ? "lavender" : "neutral"}>{f.label}</Pill>
              </li>
            ))}
          </ul>
        )}

        {job.technologies.length > 0 && (
          <ul
            className="flex flex-wrap gap-1.5"
            aria-label={t("browse.technologies")}
          >
            {job.technologies.map((tech) => (
              <li
                key={tech}
                className="rounded-full bg-lavender px-2.5 py-0.5 text-xs text-ink"
              >
                {tech}
              </li>
            ))}
            {ai?.extra_technologies.map((tech) => (
              <li
                key={tech}
                className="rounded-full border border-line px-2.5 py-0.5 text-xs text-ink-muted"
                title={t("ai.extraTech")}
              >
                {tech}
              </li>
            ))}
          </ul>
        )}

        <div className="grid gap-6 xl:grid-cols-2">
          <div className="min-w-0 xl:order-1">
            <div>
              <h3 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">
                {t("jobs.description")}
              </h3>
              <div className="mt-2 whitespace-pre-line text-sm leading-relaxed text-ink">
                {job.description || t("jobs.noDescription")}
              </div>
            </div>
          </div>
          <aside
            aria-label={t("ai.panel")}
            className="min-w-0 space-y-4 xl:order-2"
          >
            {ai?.remote_policy && (
              <p className="flex items-start gap-1.5 text-sm text-ink">
                <AiBadge model={ai.model} />
                <span>{ai.remote_policy}</span>
              </p>
            )}

            {(shownSummary || writing) && (
              <div className="rounded-card bg-canvas p-4">
                <h3 className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-ink-muted">
                  {t("ai.summary")}
                  {ai && aiSummary && <AiBadge model={ai.model} />}
                </h3>
                {shownSummary ? (
                  <p className="mt-1.5 text-sm leading-relaxed text-ink">
                    {shownSummary}
                  </p>
                ) : (
                  <p
                    role="status"
                    className="mt-1.5 animate-pulse text-sm text-ink-muted"
                  >
                    {t("ai.summarising")}
                  </p>
                )}
              </div>
            )}

            {reasons.length > 0 && (
              <details className="group rounded-card bg-canvas px-4 py-3">
                <summary className="flex cursor-pointer list-none items-center justify-between text-sm font-semibold text-ink">
                  {t("browse.whyMatch")}
                  <ChevronDown
                    aria-hidden
                    className="size-4 text-ink-muted transition-transform group-open:rotate-180"
                  />
                </summary>
                <div className="mt-3">
                  <MatchReasons reasons={reasons} />
                  {explanation && ai && (
                    <p className="mt-3 flex items-start gap-1.5 text-sm text-ink">
                      <AiBadge model={ai.model} />
                      <span>{explanation}</span>
                    </p>
                  )}
                </div>
              </details>
            )}

            {(aiSummary ||
              ai?.remote_policy ||
              explanation ||
              ai?.salary ||
              filled.size > 0) && (
              <p className="text-xs text-ink-muted">{t("ai.caption")}</p>
            )}
          </aside>
        </div>
      </div>
    </section>
  );
}

function salaryText(
  t: ReturnType<typeof useI18n>["t"],
  s: NonNullable<NonNullable<WorkspaceJob["ai"]>["salary"]>,
): string {
  const n = (v: number) => v.toLocaleString("en-US");
  const range =
    s.min != null && s.max != null && s.min !== s.max
      ? `${n(s.min)}–${n(s.max)}`
      : n((s.min ?? s.max) as number);
  return t("ai.salaryRange", {
    range,
    currency: s.currency,
    period: t(`ai.period.${s.period}`),
  });
}
