import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, ErrorState, Field } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { ApiError, data, type CrawlPace, type CrawlPaceUpdate, type CrawlPaceValues } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import type { MessageKey } from "../../lib/messages";
import { SkeletonRows } from "../../components/Skeleton";

const paceKey = ["admin", "crawl", "config"] as const;

const FIELDS: { name: keyof CrawlPaceValues; label: MessageKey; hint: MessageKey; min: number; max: number }[] = [
  { name: "concurrency", label: "pace.concurrency", hint: "pace.concurrencyHint", min: 1, max: 1024 },
  { name: "host_interval_ms", label: "pace.hostInterval", hint: "pace.hostIntervalHint", min: 100, max: 60000 },
  { name: "vendor_concurrency", label: "pace.vendorConcurrency", hint: "pace.vendorConcurrencyHint", min: 1, max: 128 },
  { name: "vendor_interval_ms", label: "pace.vendorInterval", hint: "pace.vendorIntervalHint", min: 10, max: 10000 },
];

// CrawlPaceCard lets the administrator speed the research up or slow it down.
// Saved values live in the database and the worker follows within seconds.
export function CrawlPaceCard() {
  const { t } = useI18n();
  const pace = useQuery({
    queryKey: paceKey,
    queryFn: ({ signal }) => data<CrawlPace>("/admin/crawl/config", { signal }),
  });
  return (
    <Card>
      <h2 className="text-lg font-semibold text-ink">{t("pace.title")}</h2>
      <p className="mt-1 text-sm text-ink-muted">{t("pace.hint")}</p>
      {pace.isPending ? <SkeletonRows rows={3} /> : pace.isError ? <ErrorState error={pace.error} onRetry={() => pace.refetch()} /> : <Form pace={pace.data} />}
    </Card>
  );
}

function Form({ pace }: { pace: CrawlPace }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [values, setValues] = useState<Record<keyof CrawlPaceValues, string>>(() => toText(pace));
  const save = useMutation({
    mutationFn: (body: CrawlPaceUpdate) => data<CrawlPace>("/admin/crawl/config", { method: "PUT", body }),
    onSuccess: (next) => {
      client.setQueryData(paceKey, next);
      setValues(toText(next));
    },
  });
  const fieldError = (name: string) => (save.error instanceof ApiError ? save.error.fields.find((f) => f.field === name)?.message : undefined);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const body: CrawlPaceUpdate = {};
    for (const f of FIELDS) body[f.name] = Number.parseInt(values[f.name], 10) || 0;
    save.mutate(body);
  }

  return (
    <form onSubmit={submit} className="mt-4 space-y-4" aria-label={t("pace.title")}>
      <div className="grid gap-4 sm:grid-cols-2">
        {FIELDS.map((f) => (
          <Field key={f.name} id={`pace_${f.name}`} label={t(f.label)} hint={t(f.hint, { value: String(pace.defaults[f.name]) })} error={fieldError(f.name)}>
            <input
              id={`pace_${f.name}`}
              type="number"
              inputMode="numeric"
              min={f.min}
              max={f.max}
              className={inputClass}
              value={values[f.name]}
              onChange={(e) => setValues({ ...values, [f.name]: e.target.value })}
            />
          </Field>
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" busy={save.isPending}>
          {t("common.save")}
        </Button>
        <Button
          type="button"
          variant="ghost"
          disabled={save.isPending}
          onClick={() => save.mutate({ concurrency: 0, host_interval_ms: 0, vendor_concurrency: 0, vendor_interval_ms: 0 })}
        >
          {t("pace.reset")}
        </Button>
        <span role="status" className="text-sm text-ink">
          {save.isSuccess ? t("pace.saved") : null}
        </span>
        {save.isError && !(save.error instanceof ApiError && save.error.fields.length > 0) && (
          <p role="alert" className="text-sm text-danger">
            {t("common.error")}
          </p>
        )}
      </div>
    </form>
  );
}

function toText(p: CrawlPaceValues): Record<keyof CrawlPaceValues, string> {
  return {
    concurrency: String(p.concurrency),
    host_interval_ms: String(p.host_interval_ms),
    vendor_concurrency: String(p.vendor_concurrency),
    vendor_interval_ms: String(p.vendor_interval_ms),
  };
}
