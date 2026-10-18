import { AlertTriangle, Cpu } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, ErrorState, Spinner } from "../../components/ui";
import { data, isApiError, type LlmStatus } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

const statusKey = ["admin", "llm"] as const;
const DOCS = "https://github.com/0furkancolak/rolescout/blob/main/docs/local-ai.md";

// LocalAiCard shows whether the optional local model is on, reachable and
// working, with what to do next in each state.
export function LocalAiCard() {
  const { t } = useI18n();
  const client = useQueryClient();
  const status = useQuery({
    queryKey: statusKey,
    queryFn: ({ signal }) => data<LlmStatus>("/admin/llm", { signal }),
    refetchInterval: 30_000,
  });
  const check = useMutation({
    mutationFn: () => data<LlmStatus>("/admin/llm/check", { method: "POST" }),
    onSuccess: (s) => client.setQueryData(statusKey, s),
  });

  return (
    <Card aria-labelledby="local-ai-title">
      <h2 id="local-ai-title" className="flex items-center gap-2 text-lg font-semibold text-ink">
        <Cpu aria-hidden className="size-5" />
        {t("localAi.title")}
      </h2>
      <p className="mt-1 text-sm text-ink-muted">{t("localAi.hint")}</p>
      {status.isPending ? (
        <Spinner />
      ) : status.isError ? (
        <ErrorState error={status.error} onRetry={() => status.refetch()} />
      ) : (
        <Body s={status.data} probe={check.data} onCheck={() => check.mutate()} checking={check.isPending} error={check.error} />
      )}
    </Card>
  );
}

function Body({
  s,
  probe,
  onCheck,
  checking,
  error,
}: {
  s: LlmStatus;
  probe?: LlmStatus;
  onCheck: () => void;
  checking: boolean;
  error: unknown;
}) {
  const { t } = useI18n();
  if (!s.enabled) {
    return (
      <div className="mt-4 space-y-2">
        <p className="font-medium text-ink">{t("localAi.off")}</p>
        <p className="text-sm text-ink-muted">
          {t("localAi.offHint")}{" "}
          <a href={DOCS} target="_blank" rel="noopener noreferrer" className="font-mono text-lavender-deep underline">
            docs/local-ai.md
          </a>
        </p>
      </div>
    );
  }

  let state: { title: string; hint?: string; command?: string };
  if (!s.reachable) {
    state = { title: t("localAi.unreachable"), hint: t("localAi.unreachableHint", { host: s.base_host ?? "" }) };
  } else if (!s.model_present) {
    state = { title: t("localAi.missing"), hint: t("localAi.missingHint"), command: `ollama pull ${s.model ?? ""}` };
  } else if (s.degraded) {
    state = { title: t("localAi.degraded"), hint: t("localAi.degradedHint", { model: s.fallback_model ?? "" }) };
  } else {
    state = { title: t("localAi.working") };
  }
  const working = s.reachable && s.model_present;
  const seconds = s.avg_latency_ms > 0 ? `${(s.avg_latency_ms / 1000).toFixed(1)} s` : "–";

  return (
    <div className="mt-4 space-y-4">
      {!s.local && (
        <p role="alert" className="flex items-start gap-2 rounded-control bg-amber p-3 text-sm text-ink">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          {t("localAi.nonLocal", { host: s.base_host ?? "" })}
        </p>
      )}
      <div>
        <p className="font-medium text-ink">{state.title}</p>
        {state.hint && <p className="mt-1 text-sm text-ink-muted">{state.hint}</p>}
        {state.command && <code className="mt-2 block rounded-control bg-well px-3 py-2 font-mono text-xs text-well-ink">{state.command}</code>}
        <p className="mt-1 font-mono text-xs text-ink-muted">
          {s.model} · {s.base_host}
        </p>
      </div>
      {working && (
        <ul aria-label={t("localAi.progress")} className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Counter value={s.queue.toLocaleString()} label={t("localAi.waiting")} />
          <Counter value={s.enriched.toLocaleString()} label={t("localAi.read")} />
          <Counter value={s.failed.toLocaleString()} label={t("localAi.failed")} />
          <Counter value={seconds} label={t("localAi.perJob")} />
        </ul>
      )}
      {s.last_error && <p className="text-xs text-ink-muted">{t("localAi.lastError", { error: s.last_error })}</p>}
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="secondary" busy={checking} onClick={onCheck}>
          {t("localAi.test")}
        </Button>
        <span role="status" className="text-sm text-ink">
          {probe && !checking ? (probe.probe_ok ? t("localAi.probeOk", { ms: probe.probe_ms }) : t("localAi.probeFail")) : null}
        </span>
        {error != null && (
          <p role="alert" className="text-sm text-danger">
            {isApiError(error, "rate_limited") ? t("localAi.rateLimited") : t("common.error")}
          </p>
        )}
      </div>
    </div>
  );
}

function Counter({ value, label }: { value: string; label: string }) {
  return (
    <li className="rounded-card bg-canvas p-3">
      <div className="text-2xl font-bold tabular-nums text-ink">{value}</div>
      <div className="text-xs text-ink-muted">{label}</div>
    </li>
  );
}
