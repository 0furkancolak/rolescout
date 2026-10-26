import { Cpu } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { Card, ErrorState } from "../../components/ui";
import { data, type SystemReport, type SystemService } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { SkeletonRows } from "../../components/Skeleton";

// formatBytes: 512 MB, 1.5 GB.
export function formatBytes(n: number): string {
  const mb = n / (1 << 20);
  if (mb < 1000) return `${mb >= 100 ? Math.round(mb) : mb.toFixed(1).replace(/\.0$/, "")} MB`;
  return `${(n / (1 << 30)).toFixed(1).replace(/\.0$/, "")} GB`;
}

// SystemCard shows the RAM each service uses now and the most it may use, so
// a self-hosted installation can be sized for its machine.
export function SystemCard() {
  const { t } = useI18n();
  const report = useQuery({
    queryKey: ["admin", "system"],
    queryFn: ({ signal }) => data<SystemReport>("/admin/system", { signal }),
    refetchInterval: 10_000,
  });
  return (
    <Card aria-labelledby="system-title">
      <h2 id="system-title" className="flex items-center gap-2 text-lg font-semibold text-ink">
        <Cpu aria-hidden className="size-5" />
        {t("system.title")}
      </h2>
      <p className="mt-1 text-sm text-ink-muted">{t("system.hint")}</p>
      {report.isPending ? (
        <SkeletonRows rows={4} />
      ) : report.isError ? (
        <ErrorState error={report.error} onRetry={() => report.refetch()} />
      ) : (
        <Body r={report.data} />
      )}
    </Card>
  );
}

function Body({ r }: { r: SystemReport }) {
  const { t } = useI18n();
  const used = r.services.reduce((n, s) => n + (s.used_bytes ?? 0), 0);
  const allowed = r.services.reduce((n, s) => n + (s.limit_bytes ?? 0), 0);
  const unknownLimit = r.services.some((s) => s.limit_bytes === null);
  return (
    <div className="mt-4 space-y-4">
      <dl className="grid gap-3 sm:grid-cols-3">
        <Stat label={t("system.measured")} value={formatBytes(used)} />
        <Stat label={t("system.maxAllowed")} value={`${unknownLimit ? "≥ " : ""}${formatBytes(allowed)}`} />
        <Stat label={t("system.host")} value={r.host_total_bytes > 0 ? formatBytes(r.host_total_bytes) : "—"} />
      </dl>
      <ul className="space-y-3" aria-label={t("system.title")}>
        {r.services.map((s) => (
          <ServiceRow key={s.name} s={s} />
        ))}
      </ul>
      <p className="text-xs text-ink-muted">
        {t("system.postgresInfo", {
          shared: formatBytes(r.postgres.shared_buffers_bytes),
          work: formatBytes(r.postgres.work_mem_bytes),
          conns: r.postgres.connections,
          max: r.postgres.max_connections,
          size: formatBytes(r.postgres.database_bytes),
        })}
      </p>
      <p className="text-xs text-ink-muted">{t("system.howToChange")}</p>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-card bg-canvas p-3">
      <dt className="text-xs text-ink-muted">{label}</dt>
      <dd className="text-xl font-semibold tabular-nums text-ink">{value}</dd>
    </div>
  );
}

function ServiceRow({ s }: { s: SystemService }) {
  const { t } = useI18n();
  const pct = s.used_bytes !== null && s.limit_bytes ? Math.min(100, Math.round((s.used_bytes / s.limit_bytes) * 100)) : null;
  const hot = pct !== null && pct >= 85;
  return (
    <li>
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 text-sm">
        <span className="font-medium text-ink">{t(`system.svc.${s.name}`)}</span>
        <span className="tabular-nums text-ink-muted">
          {s.used_bytes !== null ? (
            <>
              <span className="font-semibold text-ink">{formatBytes(s.used_bytes)}</span>
              {s.limit_bytes ? ` / ${formatBytes(s.limit_bytes)}` : ` · ${t("system.noLimit")}`}
            </>
          ) : s.limit_bytes ? (
            <>
              {t("system.upTo")} <span className="font-semibold text-ink">{formatBytes(s.limit_bytes)}</span>
            </>
          ) : (
            t("system.unknown")
          )}
        </span>
      </div>
      {pct !== null && (
        <div
          role="progressbar"
          aria-label={t(`system.svc.${s.name}`)}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={pct}
          className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-canvas"
        >
          <div className={`h-full rounded-full ${hot ? "bg-danger" : "bg-ink"}`} style={{ width: `${pct}%` }} />
        </div>
      )}
      <p className="mt-1 text-xs text-ink-muted">
        {s.stale
          ? t("system.stale")
          : s.note === "ceiling"
            ? t("system.noteCeiling")
            : s.note === "container"
              ? t("system.noteContainer")
              : s.estimated
                ? t("system.noteLimitOnly")
                : s.heap_bytes !== undefined
                  ? t("system.heap", { heap: formatBytes(s.heap_bytes) })
                  : ""}
      </p>
    </li>
  );
}
