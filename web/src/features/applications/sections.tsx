import { useState, type FormEvent, type ReactNode } from "react";
import { ExternalLink, X } from "lucide-react";
import { Button, Card, ErrorState, Field, InlineError } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { statusTransitions, type Application, type ApplicationStatus, type JobDetail } from "../../lib/api";
import { eventLabel, todayInput, toISODate } from "./format";
import { useI18n } from "../../lib/i18n";
import {
  useAddAttempt,
  useAddEvent,
  useAddNote,
  useApplicationTags,
  useAttempts,
  useChangeStatus,
  useCreateTag,
  useEvents,
  useNotes,
  useSetTag,
  useTags,
  useUpdateNote,
} from "./api";
import { SkeletonRows } from "../../components/Skeleton";

interface Props {
  ws: string;
  application: Application;
}

function SectionTitle({ children }: { children: string }) {
  return <h2 className="mb-3 text-lg font-semibold">{children}</h2>;
}

export function StatusControl({ ws, application }: Props) {
  const { t } = useI18n();
  const change = useChangeStatus(ws, application.id);
  const options = statusTransitions[application.status];
  const [next, setNext] = useState<ApplicationStatus | "">("");

  if (options.length === 0) return <p className="text-sm text-ink-muted">{t("apps.noTransitions")}</p>;

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (next) change.mutate(next, { onSuccess: () => setNext("") });
  };

  return (
    <form onSubmit={onSubmit} className="space-y-2">
      <div className="flex flex-wrap items-end gap-2">
        <Field id="status" label={t("apps.changeStatus")}>
          <select
            id="status"
            value={next}
            onChange={(e) => setNext(e.target.value as ApplicationStatus)}
            className={inputClass}
          >
            <option value="" disabled>
              —
            </option>
            {options.map((s) => (
              <option key={s} value={s}>
                {t(`status.${s}`)}
              </option>
            ))}
          </select>
        </Field>
        <Button type="submit" disabled={!next} busy={change.isPending}>
          {t("common.save")}
        </Button>
      </div>
      <InlineError error={change.error} />
    </form>
  );
}

export function HistorySection({ ws, application }: Props) {
  const { t, formatDate } = useI18n();
  const events = useEvents(ws, application.id);
  return (
    <Card>
      <SectionTitle>{t("apps.history")}</SectionTitle>
      {events.isPending ? (
        <SkeletonRows rows={2} />
      ) : events.isError ? (
        <ErrorState error={events.error} onRetry={() => void events.refetch()} />
      ) : (
        <ol className="relative space-y-4 border-l border-line pl-5">
          {[...events.data].reverse().map((e) => (
            <li key={e.id} className="relative">
              <span aria-hidden className="absolute -left-[25px] top-1.5 size-2.5 rounded-full bg-ink ring-4 ring-surface" />
              <p className="text-sm font-medium text-ink">{eventLabel(e, t)}</p>
              <time dateTime={e.occurred_at} className="text-xs text-ink-muted">
                {formatDate(e.occurred_at, true)}
              </time>
            </li>
          ))}
        </ol>
      )}
    </Card>
  );
}

export function AttemptsSection({ ws, application }: Props) {
  const { t, formatDate } = useI18n();
  const attempts = useAttempts(ws, application.id);
  const add = useAddAttempt(ws, application.id);
  const [appliedAt, setAppliedAt] = useState(todayInput);
  const [channel, setChannel] = useState("");

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    add.mutate(
      { applied_at: toISODate(appliedAt), channel: channel.trim() },
      { onSuccess: () => setChannel("") },
    );
  };

  return (
    <Card>
      <SectionTitle>{t("apps.attempts")}</SectionTitle>
      {attempts.isPending ? (
        <SkeletonRows rows={2} />
      ) : attempts.isError ? (
        <ErrorState error={attempts.error} onRetry={() => void attempts.refetch()} />
      ) : attempts.data.length === 0 ? (
        <p className="text-sm text-ink-muted">{t("apps.noAttempts")}</p>
      ) : (
        <ul className="space-y-2">
          {attempts.data.map((a) => (
            <li key={a.id} className="text-sm">
              <span className="font-medium">{t("apps.attemptNumber", { n: a.attempt_number })}</span>
              <span className="text-ink-muted">
                {" "}
                · {formatDate(a.applied_at)}
                {a.channel && ` · ${a.channel}`}
              </span>
            </li>
          ))}
        </ul>
      )}
      <form onSubmit={onSubmit} className="mt-4 space-y-3 border-t border-line pt-4">
        <Field id="applied_at" label={t("apps.appliedAt")}>
          <input
            id="applied_at"
            type="date"
            required
            max={todayInput()}
            value={appliedAt}
            onChange={(e) => setAppliedAt(e.target.value)}
            className={inputClass}
          />
        </Field>
        <Field id="channel" label={t("apps.channel")}>
          <input
            id="channel"
            maxLength={100}
            value={channel}
            placeholder={t("apps.channelPlaceholder")}
            onChange={(e) => setChannel(e.target.value)}
            className={inputClass}
          />
        </Field>
        <InlineError error={add.error} />
        <Button type="submit" variant="secondary" busy={add.isPending} disabled={!appliedAt}>
          {t("apps.addAttempt")}
        </Button>
      </form>
    </Card>
  );
}

const scheduleTypes = ["interview", "follow_up", "deadline"] as const;
type ScheduleType = (typeof scheduleTypes)[number];

export function ScheduleSection({ ws, application }: Props) {
  const { t, formatDate } = useI18n();
  const events = useEvents(ws, application.id);
  const add = useAddEvent(ws, application.id);
  const [type, setType] = useState<ScheduleType>("interview");
  const [when, setWhen] = useState("");

  const [now] = useState(Date.now);
  const upcoming = (events.data ?? [])
    .filter((e) => (scheduleTypes as readonly string[]).includes(e.type) && Date.parse(e.occurred_at) >= now)
    .sort((a, b) => Date.parse(a.occurred_at) - Date.parse(b.occurred_at));

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (!when) return;
    add.mutate({ type, occurred_at: new Date(when).toISOString() }, { onSuccess: () => setWhen("") });
  };

  return (
    <Card>
      <SectionTitle>{t("apps.schedule")}</SectionTitle>
      {upcoming.length > 0 && (
        <div className="mb-4">
          <h3 className="mb-2 text-sm font-medium text-ink">{t("apps.upcoming")}</h3>
          <ul className="space-y-1 text-sm">
            {upcoming.map((e) => (
              <li key={e.id}>
                <span className="font-medium">{t(`event.${e.type}`)}</span>{" "}
                <time dateTime={e.occurred_at} className="text-ink-muted">
                  {formatDate(e.occurred_at, true)}
                </time>
              </li>
            ))}
          </ul>
        </div>
      )}
      <form onSubmit={onSubmit} className="space-y-3">
        <Field id="event_type" label={t("apps.eventType")}>
          <select
            id="event_type"
            value={type}
            onChange={(e) => setType(e.target.value as ScheduleType)}
            className={inputClass}
          >
            {scheduleTypes.map((s) => (
              <option key={s} value={s}>
                {t(`event.${s}`)}
              </option>
            ))}
          </select>
        </Field>
        <Field id="event_date" label={t("apps.eventDate")}>
          <input
            id="event_date"
            type="datetime-local"
            required
            value={when}
            onChange={(e) => setWhen(e.target.value)}
            className={inputClass}
          />
        </Field>
        <InlineError error={add.error} />
        <Button type="submit" variant="secondary" busy={add.isPending} disabled={!when}>
          {t("apps.addEvent")}
        </Button>
      </form>
    </Card>
  );
}

export function NotesSection({ ws, application }: Props) {
  const { t, formatDate } = useI18n();
  const notes = useNotes(ws, application.id);
  const add = useAddNote(ws, application.id);
  const update = useUpdateNote(ws, application.id);
  const [body, setBody] = useState("");
  const [editing, setEditing] = useState<{ id: string; body: string } | null>(null);

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (body.trim()) add.mutate(body.trim(), { onSuccess: () => setBody("") });
  };

  return (
    <Card>
      <SectionTitle>{t("apps.notes")}</SectionTitle>
      <form onSubmit={onSubmit} className="space-y-2">
        <label htmlFor="new_note" className="sr-only">
          {t("apps.noteLabel")}
        </label>
        <textarea
          id="new_note"
          rows={3}
          maxLength={10000}
          value={body}
          placeholder={t("apps.notePlaceholder")}
          onChange={(e) => setBody(e.target.value)}
          className={inputClass}
        />
        <InlineError error={add.error} />
        <Button type="submit" variant="secondary" busy={add.isPending} disabled={!body.trim()}>
          {t("apps.addNote")}
        </Button>
      </form>
      <div className="mt-4">
        {notes.isPending ? (
          <SkeletonRows rows={2} />
        ) : notes.isError ? (
          <ErrorState error={notes.error} onRetry={() => void notes.refetch()} />
        ) : notes.data.length === 0 ? (
          <p className="text-sm text-ink-muted">{t("apps.noNotes")}</p>
        ) : (
          <ul className="space-y-3">
            {[...notes.data].reverse().map((n) => (
              <li key={n.id} className="rounded-md bg-canvas p-3">
                {editing?.id === n.id ? (
                  <form
                    className="space-y-2"
                    onSubmit={(e) => {
                      e.preventDefault();
                      if (editing.body.trim())
                        update.mutate(
                          { noteID: n.id, body: editing.body.trim() },
                          { onSuccess: () => setEditing(null) },
                        );
                    }}
                  >
                    <label htmlFor={`note-${n.id}`} className="sr-only">
                      {t("apps.noteLabel")}
                    </label>
                    <textarea
                      id={`note-${n.id}`}
                      rows={3}
                      maxLength={10000}
                      value={editing.body}
                      onChange={(e) => setEditing({ id: n.id, body: e.target.value })}
                      className={inputClass}
                    />
                    <InlineError error={update.error} />
                    <div className="flex gap-2">
                      <Button type="submit" busy={update.isPending} disabled={!editing.body.trim()}>
                        {t("common.save")}
                      </Button>
                      <Button variant="ghost" onClick={() => setEditing(null)}>
                        {t("common.cancel")}
                      </Button>
                    </div>
                  </form>
                ) : (
                  <>
                    <p className="whitespace-pre-line text-sm text-ink">{n.body}</p>
                    <div className="mt-2 flex items-center justify-between gap-2">
                      <p className="text-xs text-ink-muted">
                        {formatDate(n.created_at, true)}
                        {n.updated_at !== n.created_at && ` · ${t("apps.edited")}`}
                      </p>
                      <Button variant="ghost" className="min-h-8 px-2 py-1" onClick={() => setEditing({ id: n.id, body: n.body })}>
                        {t("common.edit")}
                      </Button>
                    </div>
                  </>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </Card>
  );
}

export function TagsSection({ ws, application }: Props) {
  const { t } = useI18n();
  const attached = useApplicationTags(ws, application.id);
  const all = useTags(ws);
  const setTag = useSetTag(ws, application.id);
  const createTag = useCreateTag(ws);
  const [name, setName] = useState("");

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) return;
    // Reuse an existing tag with the same name (names are unique per workspace,
    // case-insensitively) instead of failing with a conflict.
    let tag = all.data?.find((tg) => tg.name.toLowerCase() === trimmed.toLowerCase());
    try {
      tag ??= await createTag.mutateAsync(trimmed);
      await setTag.mutateAsync({ tagID: tag.id, on: true });
      setName("");
    } catch {
      // The mutation's error state is rendered below.
    }
  };

  const attachedIDs = new Set(attached.data?.map((tg) => tg.id));
  const suggestions = all.data?.filter((tg) => !attachedIDs.has(tg.id)) ?? [];

  return (
    <Card>
      <SectionTitle>{t("apps.tags")}</SectionTitle>
      {attached.isPending ? (
        <SkeletonRows rows={2} />
      ) : attached.isError ? (
        <ErrorState error={attached.error} onRetry={() => void attached.refetch()} />
      ) : attached.data.length === 0 ? (
        <p className="text-sm text-ink-muted">{t("apps.noTags")}</p>
      ) : (
        <ul className="flex flex-wrap gap-2">
          {attached.data.map((tg) => (
            <li key={tg.id} className="inline-flex items-center gap-1 rounded-full bg-lavender py-0.5 pl-3 pr-1 text-sm text-ink">
              {tg.name}
              <button
                type="button"
                aria-label={t("apps.removeTag", { name: tg.name })}
                onClick={() => setTag.mutate({ tagID: tg.id, on: false })}
                className="rounded-full p-1 hover:bg-canvas"
              >
                <X aria-hidden className="size-3.5" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <form onSubmit={(e) => void onSubmit(e)} className="mt-4 flex items-end gap-2">
        <div className="flex-1">
          <Field id="tag_name" label={t("apps.tagName")}>
            <input
              id="tag_name"
              list="tag-suggestions"
              maxLength={50}
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={inputClass}
            />
          </Field>
          <datalist id="tag-suggestions">
            {suggestions.map((tg) => (
              <option key={tg.id} value={tg.name} />
            ))}
          </datalist>
        </div>
        <Button type="submit" variant="secondary" busy={createTag.isPending || setTag.isPending} disabled={!name.trim()}>
          {t("apps.addTag")}
        </Button>
      </form>
      <InlineError error={createTag.error ?? setTag.error} />
    </Card>
  );
}

function InfoRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5 py-2.5 sm:flex-row sm:items-baseline sm:gap-4">
      <dt className="shrink-0 text-sm text-ink-muted sm:w-40">{label}</dt>
      <dd className="min-w-0 text-sm font-medium text-ink">{children}</dd>
    </div>
  );
}

// InfoSection is what the application is about: the job and the first
// application, at a glance and with a way back to the original posting.
export function InfoSection({ ws, application, job }: Props & { job?: JobDetail }) {
  const { t, formatDate } = useI18n();
  const attempts = useAttempts(ws, application.id);
  const first = attempts.data?.[0];
  const url = job?.listings[0]?.url;
  let host = "";
  try {
    host = url ? new URL(url).host : "";
  } catch {
    host = "";
  }
  return (
    <Card>
      <SectionTitle>{t("apps.info")}</SectionTitle>
      <dl className="divide-y divide-line">
        <InfoRow label={t("apps.info.company")}>{job?.company_name ?? "…"}</InfoRow>
        <InfoRow label={t("apps.info.location")}>{job ? (job.location ?? t("jobs.locationUnknown")) : "…"}</InfoRow>
        {job && <InfoRow label={t("apps.info.arrangement")}>{t(`work.${job.work_arrangement}` as "work.unknown")}</InfoRow>}
        {job && <InfoRow label={t("apps.info.type")}>{t(`employment.${job.employment_type}` as "employment.unknown")}</InfoRow>}
        {job && <InfoRow label={t("apps.info.seniority")}>{t(`seniority.${job.seniority}` as "seniority.unknown")}</InfoRow>}
        <InfoRow label={t("apps.info.firstApplied")}>
          {first ? (
            `${formatDate(first.applied_at)}${first.channel ? ` · ${first.channel}` : ""}`
          ) : (
            <span className="font-normal text-ink-muted">{t("apps.info.notApplied")}</span>
          )}
        </InfoRow>
        <InfoRow label={t("apps.info.added")}>{formatDate(application.created_at, true)}</InfoRow>
        <InfoRow label={t("apps.info.updated")}>{formatDate(application.updated_at, true)}</InfoRow>
        {job && <InfoRow label={t("apps.info.posted")}>{formatDate(job.first_seen_at)}</InfoRow>}
      </dl>
      {job?.closed_at && <p className="mt-3 rounded-md bg-amber px-3 py-2 text-sm text-ink">{t("apps.info.closedPosting")}</p>}
      {url && (
        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          className="mt-4 inline-flex min-h-10 cursor-pointer items-center gap-2 rounded-control border border-line-strong px-3 text-sm font-medium text-ink transition-colors hover:bg-canvas focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
        >
          <ExternalLink aria-hidden className="size-4" />
          {t("apps.info.openPosting", { host })}
        </a>
      )}
    </Card>
  );
}
