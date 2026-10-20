import { AlertTriangle } from "lucide-react";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { NotFound } from "../../components/NotFound";
import { TabPanel, Tabs } from "../../components/Tabs";
import {
  Button,
  Card,
  ErrorState,
  InlineError,
  PageTitle,
  Spinner,
} from "../../components/ui";
import { inputClass } from "../../components/styles";
import { data, type AdapterReview, type Setting } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useCurrentUser } from "../auth/api";
import { LocalAiCard } from "./LocalAiCard";
import { SystemCard } from "./SystemCard";
import { CrawlPaceCard } from "./CrawlPaceCard";
import { UsersCard } from "./UsersCard";

const adaptersKey = ["admin", "adapters"] as const;
const tabs = ["sources", "people", "ai", "crawl", "system"] as const;
type SettingsTab = (typeof tabs)[number];

// SettingsPage is for the installation administrator: adapter-type reviews
// and the crawl settings in effect.
export function SettingsPage() {
  const { t } = useI18n();
  const me = useCurrentUser();
  const [params, setParams] = useSearchParams();
  const requested = params.get("tab");
  const tab: SettingsTab = tabs.find((x) => x === requested) ?? "sources";
  const isAdmin = !!me.data?.is_admin;
  const adapters = useQuery({
    queryKey: adaptersKey,
    queryFn: ({ signal }) =>
      data<AdapterReview[]>("/admin/adapters", { signal }),
    enabled: isAdmin && tab === "sources",
  });
  const settings = useQuery({
    queryKey: ["admin", "settings"],
    queryFn: ({ signal }) => data<Setting[]>("/admin/settings", { signal }),
    enabled: isAdmin && tab === "crawl",
  });

  if (me.isPending) return <Spinner />;
  if (!isAdmin) return <NotFound />;

  return (
    <div className="space-y-6">
      <PageTitle>{t("settings.title")}</PageTitle>
      <Tabs
        label={t("settings.title")}
        idPrefix="settings"
        value={tab}
        items={tabs.map((x) => ({
          value: x,
          label: t(`settings.tab.${x}` as const),
        }))}
        onChange={(next) => setParams({ tab: next }, { replace: true })}
      />
      <TabPanel idPrefix="settings" value={tab}>
        {tab === "sources" && (
          <>
            <Card>
              <h2 className="text-lg font-semibold text-ink">
                {t("settings.adapters")}
              </h2>
              <p className="mt-1 text-sm text-ink-muted">
                {t("settings.adaptersHint")}
              </p>
              {adapters.isPending ? (
                <Spinner />
              ) : adapters.isError ? (
                <ErrorState
                  error={adapters.error}
                  onRetry={() => adapters.refetch()}
                />
              ) : (
                <div className="mt-4 space-y-3">
                  {adapters.data.map((a) => (
                    <AdapterRow key={a.adapter} review={a} />
                  ))}
                </div>
              )}
            </Card>
          </>
        )}
        {tab === "people" && <UsersCard />}
        {tab === "ai" && <LocalAiCard />}
        {tab === "system" && <SystemCard />}
        {tab === "crawl" && (
          <>
            {isAdmin && <CrawlPaceCard />}
            <Card>
              <h2 className="text-lg font-semibold text-ink">
                {t("settings.crawl")}
              </h2>
              <p className="mt-1 text-sm text-ink-muted">
                {t("settings.crawlHint")}
              </p>
              {settings.data && (
                <dl className="mt-4 grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-2">
                  {settings.data.map((s) => (
                    <div
                      key={s.name}
                      className="flex min-w-0 justify-between gap-3 border-b border-line py-1.5 text-sm"
                    >
                      <dt className="shrink-0 font-mono text-xs text-ink-muted">
                        {s.name}
                      </dt>
                      <dd className="min-w-0 truncate font-medium text-ink">
                        {s.value}
                      </dd>
                    </div>
                  ))}
                </dl>
              )}
            </Card>
          </>
        )}
      </TabPanel>
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
      data<AdapterReview>(`/admin/adapters/${review.adapter}`, {
        method: "PATCH",
        body: { enabled, note: note.trim() },
      }),
    onSuccess: () => client.invalidateQueries({ queryKey: adaptersKey }),
  });
  const id = `note-${review.adapter}`;

  return (
    <fieldset
      aria-label={review.adapter}
      className="rounded-lg border border-line p-3"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <span className="font-mono text-sm font-semibold text-ink">
            {review.adapter}
          </span>
          <span
            className={`ml-2 inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${review.enabled ? "bg-sage text-ink" : "bg-canvas text-ink"}`}
          >
            {review.enabled ? t("settings.enable") : t("settings.disable")}
          </span>
          <span className="ml-2 text-xs text-ink-muted">
            {review.reviewed_at
              ? t("settings.reviewed", { date: formatDate(review.reviewed_at) })
              : t("settings.notReviewed")}
          </span>
        </div>
        {review.enabled ? (
          <Button
            variant="secondary"
            busy={save.isPending}
            onClick={() => save.mutate(false)}
          >
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
        <p className="mt-2 flex items-start gap-2 text-sm text-amber-deep">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          {t("settings.workdayWarning")}
        </p>
      )}
      {!review.enabled && (
        <div className="mt-2">
          <label
            htmlFor={id}
            className="block text-xs font-medium text-ink-muted"
          >
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
            <p role="alert" className="mt-1 text-xs text-danger">
              {t("settings.noteRequired")}
            </p>
          )}
        </div>
      )}
      {review.enabled && review.note && (
        <p className="mt-2 text-xs text-ink-muted">{review.note}</p>
      )}
      <InlineError error={save.error} />
    </fieldset>
  );
}
