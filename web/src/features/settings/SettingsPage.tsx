import { AlertTriangle } from "lucide-react";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { NotFound } from "../../components/NotFound";
import { Button, Card, ErrorState, InlineError, PageTitle, Spinner } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { data, type AdapterReview, type Setting } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useCurrentUser } from "../auth/api";

const adaptersKey = ["admin", "adapters"] as const;

// SettingsPage is for the installation administrator: adapter-type reviews
// and the crawl settings in effect.
export function SettingsPage() {
  const { t } = useI18n();
  const me = useCurrentUser();
  const isAdmin = !!me.data?.is_admin;
  const adapters = useQuery({
    queryKey: adaptersKey,
    queryFn: ({ signal }) => data<AdapterReview[]>("/admin/adapters", { signal }),
    enabled: isAdmin,
  });
  const settings = useQuery({
    queryKey: ["admin", "settings"],
    queryFn: ({ signal }) => data<Setting[]>("/admin/settings", { signal }),
    enabled: isAdmin,
  });

  if (me.isPending) return <Spinner />;
  if (!isAdmin) return <NotFound />;

  return (
    <div className="space-y-6">
      <PageTitle>{t("settings.title")}</PageTitle>
      <Card>
        <h2 className="text-lg font-semibold text-slate-900">{t("settings.adapters")}</h2>
        <p className="mt-1 text-sm text-slate-500">{t("settings.adaptersHint")}</p>
        {adapters.isPending ? (
          <Spinner />
        ) : adapters.isError ? (
          <ErrorState error={adapters.error} onRetry={() => adapters.refetch()} />
        ) : (
          <div className="mt-4 space-y-3">
            {adapters.data.map((a) => (
              <AdapterRow key={a.adapter} review={a} />
            ))}
          </div>
        )}
      </Card>
      <Card>
        <h2 className="text-lg font-semibold text-slate-900">{t("settings.crawl")}</h2>
        <p className="mt-1 text-sm text-slate-500">{t("settings.crawlHint")}</p>
        {settings.data && (
          <dl className="mt-4 grid gap-x-6 gap-y-2 sm:grid-cols-2">
            {settings.data.map((s) => (
              <div key={s.name} className="flex justify-between gap-3 border-b border-slate-100 py-1.5 text-sm">
                <dt className="font-mono text-xs text-slate-600">{s.name}</dt>
                <dd className="truncate font-medium text-slate-900">{s.value}</dd>
              </div>
            ))}
          </dl>
        )}
      </Card>
    </div>
  );
}

function AdapterRow({ review }: { review: AdapterReview }) {
  const { t, formatDate } = useI18n();
  const client = useQueryClient();
  const [note, setNote] = useState(review.note ?? "");
  const [missing, setMissing] = useState(false);
  const save = useMutation({
    mutationFn: (enabled: boolean) =>
      data<AdapterReview>(`/admin/adapters/${review.adapter}`, { method: "PATCH", body: { enabled, note: note.trim() } }),
    onSuccess: () => client.invalidateQueries({ queryKey: adaptersKey }),
  });
  const id = `note-${review.adapter}`;

  return (
    <fieldset aria-label={review.adapter} className="rounded-lg border border-slate-200 p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <span className="font-mono text-sm font-semibold text-slate-900">{review.adapter}</span>
          <span
            className={`ml-2 inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${review.enabled ? "bg-green-100 text-green-800" : "bg-slate-100 text-slate-700"}`}
          >
            {review.enabled ? t("settings.enable") : t("settings.disable")}
          </span>
          <span className="ml-2 text-xs text-slate-500">
            {review.reviewed_at ? t("settings.reviewed", { date: formatDate(review.reviewed_at) }) : t("settings.notReviewed")}
          </span>
        </div>
        {review.enabled ? (
          <Button variant="secondary" busy={save.isPending} onClick={() => save.mutate(false)}>
            {t("settings.disable")}
          </Button>
        ) : (
          <Button
            busy={save.isPending}
            onClick={() => {
              const empty = note.trim() === "";
              setMissing(empty);
              if (!empty) save.mutate(true);
            }}
          >
            {t("settings.enable")}
          </Button>
        )}
      </div>
      {review.adapter === "workday" && (
        <p className="mt-2 flex items-start gap-2 text-sm text-amber-800">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          {t("settings.workdayWarning")}
        </p>
      )}
      {!review.enabled && (
        <div className="mt-2">
          <label htmlFor={id} className="block text-xs font-medium text-slate-600">
            {t("settings.note")}
          </label>
          <textarea
            id={id}
            rows={2}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder={t("settings.notePlaceholder")}
            className={`${inputClass} mt-1`}
          />
          {missing && (
            <p role="alert" className="mt-1 text-xs text-red-700">
              {t("settings.noteRequired")}
            </p>
          )}
        </div>
      )}
      {review.enabled && review.note && <p className="mt-2 text-xs text-slate-500">{review.note}</p>}
      <InlineError error={save.error} />
    </fieldset>
  );
}
