import {
  ArrowLeft,
  Bookmark,
  BookmarkCheck,
  Check,
  ChevronDown,
  ExternalLink,
  Send,
  Sparkles,
  Undo2,
  WandSparkles,
  X,
} from "lucide-react";
import { useState, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { CompanyTile } from "../../components/CompanyTile";
import { Pill } from "../../components/Pill";
import { ScoreRing } from "../../components/ScoreRing";
import { Button, InlineError } from "../../components/ui";
import { buttonClass } from "../../components/styles";
import { isApiError, type TechFit, type WorkspaceJob } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { enumLabel } from "../../lib/labels";
import { useApplications, useDeleteApplication } from "../applications/api";
import { useWorkspace } from "../workspaces/context";
import { AutofillPanel } from "./AutofillPanel";
import {
  useAnalyzeJob,
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
  profileId,
  onBack,
}: {
  job: WorkspaceJob;
  /** The profile the list is scored against; empty when browsing without one. */
  profileId?: string;
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
  const analyze = useAnalyzeJob(workspace.id);
  const [fillOpen, setFillOpen] = useState(false);
  const url = detail.data?.listings[0]?.url;

  const generated = analyze.variables?.jobID === job.id ? analyze.data : undefined;
  const ai = job.ai ?? detail.data?.ai ?? generated?.ai ?? null;
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

  const streaming = summary.status === "streaming";
  const aiSummary = summary.text !== "" ? summary.text : null;
  const writing = summary.status === "loading" || streaming;
  const shownSummary = aiSummary ?? (summary.status === "unavailable" ? ai?.summary || null : null);
  const technologies = mergeTechnologies(job.technologies, ai?.extra_technologies ?? []);
  const fit = job.tech_fit ?? null;
  const reasons = job.match?.reasons ?? [];
  const explanation = job.match?.explanation ?? generated?.explanation ?? null;
  const canGenerate = !!job.match && !!profileId && !explanation;

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
          <CompanyTile name={job.company_name} size={40} />
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
        <div className="mt-3 flex items-center gap-2">
          {application ? (
            <>
              <Link
                to={`/applications/${application.id}`}
                className={buttonClass("secondary", "text-sage-deep")}
              >
                <Check aria-hidden className="size-4" />
                {t(`status.${application.status}`)}
              </Link>
              <IconButton
                label={t("browse.undoApplied")}
                busy={remove.isPending}
                onClick={() => remove.mutate(application.id)}
              >
                <Undo2 aria-hidden className="size-4" />
              </IconButton>
            </>
          ) : (
            <Button busy={apply.isPending} onClick={() => apply.mutate(job.id)}>
              <Send aria-hidden className="size-4" />
              {t("browse.applied")}
            </Button>
          )}
          <span aria-hidden className="mx-1 h-5 w-px bg-line" />
          <IconButton
            label={job.saved ? t("jobs.unsave") : t("jobs.save")}
            busy={save.isPending}
            pressed={job.saved}
            onClick={() => save.mutate({ jobID: job.id, saved: !job.saved })}
          >
            {job.saved ? (
              <BookmarkCheck aria-hidden className="size-4" />
            ) : (
              <Bookmark aria-hidden className="size-4" />
            )}
          </IconButton>
          <IconButton
            label={t("autofill.open")}
            expanded={fillOpen}
            onClick={() => setFillOpen((o) => !o)}
          >
            <WandSparkles aria-hidden className="size-4" />
          </IconButton>
          {url && (
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              aria-label={t("browse.careerPage")}
              title={t("browse.careerPage")}
              className={iconButtonClass}
            >
              <ExternalLink aria-hidden className="size-4" />
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

        <div className="grid gap-6 xl:grid-cols-2">
          <div className="min-w-0 xl:order-1">
            <div>
              <h3 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">
                {t("jobs.description")}
              </h3>
              {job.description ? (
                <div className="mt-2 whitespace-pre-line break-words text-sm leading-relaxed text-ink">{job.description}</div>
              ) : (
                <div className="mt-2 rounded-card border border-dashed border-line-strong p-4 text-sm text-ink-muted">
                  <p className="font-medium text-ink">{t("jobs.noDescription")}</p>
                  <p className="mt-1">{t("jobs.noDescriptionHint")}</p>
                  {url && (
                    <a href={url} target="_blank" rel="noopener noreferrer" className={buttonClass("secondary", "mt-3")}>
                      <ExternalLink aria-hidden className="size-4" />
                      {t("jobs.openSource")}
                    </a>
                  )}
                </div>
              )}
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
                  {ai && aiSummary && !streaming && !summary.facts && <AiBadge model={ai.model} />}
                </h3>
                {shownSummary ? (
                  <p
                    className="mt-1.5 text-sm leading-relaxed text-ink"
                    aria-live="polite"
                    aria-busy={streaming}
                  >
                    {shownSummary}
                    {streaming && (
                      <span
                        aria-hidden
                        className="ml-0.5 inline-block h-3.5 w-0.5 animate-pulse bg-ink-muted align-[-2px]"
                      />
                    )}
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

            {technologies.length > 0 && (
              <div>
                {fit && (
                  <p className="mb-1.5 text-xs text-ink-muted">
                    <span className="font-semibold uppercase tracking-wide">{t("tech.fitTitle")}</span>
                    {" · "}
                    {t("tech.fitCount", { matched: fit.matched.length, total: fit.matched.length + fit.missing.length })}
                    {" · "}
                    {fit.from_cv ? t("tech.fromCV") : t("tech.fromProfile")}
                  </p>
                )}
                <ul className="flex flex-wrap gap-1.5" aria-label={t("browse.technologies")}>
                  {technologies.map((tech) => {
                    const state = fitState(fit, tech.name);
                    if (state) {
                      const label = state === "match" ? t("tech.have") : state === "excluded" ? t("tech.excluded") : t("tech.lack");
                      return (
                        <li
                          key={tech.name}
                          title={label}
                          className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs ${
                            state === "match" ? "bg-sage text-sage-deep" : "bg-danger-soft text-danger"
                          } ${state === "excluded" ? "font-semibold line-through" : ""}`}
                        >
                          {state === "match" ? <Check aria-hidden className="size-3" /> : <X aria-hidden className="size-3" />}
                          {tech.name}
                          <span className="sr-only">{label}</span>
                        </li>
                      );
                    }
                    return (
                      <li
                        key={tech.name}
                        className={
                          tech.extra
                            ? "rounded-full border border-line px-2.5 py-0.5 text-xs text-ink-muted"
                            : "rounded-full bg-lavender px-2.5 py-0.5 text-xs text-ink"
                        }
                        title={tech.extra ? t("ai.extraTech") : undefined}
                      >
                        {tech.name}
                      </li>
                    );
                  })}
                </ul>
              </div>
            )}

            {job.match && (reasons.length > 0 || canGenerate) && (
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
                  {canGenerate && (
                    <div className="mt-3 space-y-2">
                      <Button
                        variant="secondary"
                        busy={analyze.isPending && analyze.variables?.jobID === job.id}
                        onClick={() => analyze.mutate({ jobID: job.id, profileID: profileId ?? "", lang: locale })}
                      >
                        <Sparkles aria-hidden className="size-4" />
                        {t("ai.generate")}
                      </Button>
                      {analyze.isError && analyze.variables?.jobID === job.id && (
                        <p role="alert" className="text-sm text-danger">
                          {analyzeError(t, analyze.error)}
                        </p>
                      )}
                    </div>
                  )}
                  {explanation && (
                    <p className="mt-3 flex items-start gap-1.5 text-sm text-ink">
                      {ai && <AiBadge model={ai.model} />}
                      <span>{explanation}</span>
                    </p>
                  )}
                </div>
              </details>
            )}

            {((aiSummary && !summary.facts) ||
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

function analyzeError(t: ReturnType<typeof useI18n>["t"], err: unknown): string {
  if (isApiError(err, "ai_disabled")) return t("ai.generateOff");
  if (isApiError(err, "ai_unavailable")) return t("ai.generateUnavailable");
  if (isApiError(err, "ai_failed")) return t("ai.generateFailed");
  if (isApiError(err, "rate_limited")) return t("ai.generateBusy");
  return t("common.error");
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

const techAlias: Record<string, string> = { golang: "go", nodejs: "node.js", "node js": "node.js", "google cloud platform": "gcp", k8s: "kubernetes", js: "javascript", ts: "typescript" };
const techKey = (name: string) => {
  const k = name.toLowerCase().trim();
  return techAlias[k] ?? k;
};

// fitState says whether the person has a technology (match), lacks it
// (missing) or ruled it out (excluded); null when nothing was compared.
function fitState(fit: TechFit | null, name: string): "match" | "missing" | "excluded" | null {
  if (!fit) return null;
  const key = techKey(name);
  const has = (list: string[]) => list.some((x) => techKey(x) === key);
  if (has(fit.excluded)) return "excluded";
  if (has(fit.missing)) return "missing";
  if (has(fit.matched)) return "match";
  return null;
}

// mergeTechnologies lists each technology once: the rules' names first, then
// what only the model found; spellings of one thing ("go", "golang") count once.
function mergeTechnologies(found: string[], extra: string[]): { name: string; extra: boolean }[] {
  const seen = new Set<string>();
  const out: { name: string; extra: boolean }[] = [];
  for (const [list, isExtra] of [[found, false], [extra, true]] as const) {
    for (const name of list) {
      const key = techKey(name);
      if (seen.has(key)) continue;
      seen.add(key);
      out.push({ name: techAlias[name.toLowerCase().trim()] ?? name, extra: isExtra });
    }
  }
  return out;
}

const iconButtonClass =
  "inline-flex size-10 shrink-0 items-center justify-center rounded-control text-ink-muted transition-colors hover:bg-canvas hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ink aria-pressed:text-ink aria-expanded:bg-canvas aria-expanded:text-ink disabled:opacity-50";

function IconButton({
  label,
  busy,
  pressed,
  expanded,
  onClick,
  children,
}: {
  label: string;
  busy?: boolean;
  pressed?: boolean;
  expanded?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      aria-pressed={pressed}
      aria-expanded={expanded}
      disabled={busy}
      onClick={onClick}
      className={iconButtonClass}
    >
      {children}
    </button>
  );
}
