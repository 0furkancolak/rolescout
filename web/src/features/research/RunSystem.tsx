import { Cpu } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { data, type SystemReport } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useCurrentUser } from "../auth/api";
import { formatBytes } from "../settings/SystemCard";

// RunSystem is a one-line view of the RAM the installation uses while a run
// works (administrators only; the numbers come from /admin/system).
export function RunSystem({ active }: { active: boolean }) {
  const { t } = useI18n();
  const me = useCurrentUser();
  const isAdmin = !!me.data?.is_admin;
  const report = useQuery({
    queryKey: ["admin", "system"],
    queryFn: ({ signal }) => data<SystemReport>("/admin/system", { signal }),
    enabled: isAdmin,
    refetchInterval: active ? 5_000 : false,
  });
  if (!isAdmin || !report.data) return null;
  return (
    <section aria-label={t("run.system")} className="rounded-card bg-surface p-4 shadow-card">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
          <Cpu aria-hidden className="size-4" />
          {t("run.system")}
        </h2>
        <Link to="/settings?tab=system" className="text-xs text-lavender-deep hover:underline">
          {t("run.systemMore")}
        </Link>
      </div>
      <ul className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {report.data.services.map((s) => {
          const pct = s.used_bytes !== null && s.limit_bytes ? Math.min(100, Math.round((s.used_bytes / s.limit_bytes) * 100)) : null;
          return (
            <li key={s.name}>
              <div className="flex items-baseline justify-between gap-2 text-sm">
                <span className="text-ink-muted">{t(`system.svc.${s.name}`)}</span>
                <span className="tabular-nums">
                  {s.used_bytes !== null ? (
                    <>
                      <span className="font-semibold text-ink">{formatBytes(s.used_bytes)}</span>
                      {s.limit_bytes ? <span className="text-ink-muted"> / {formatBytes(s.limit_bytes)}</span> : null}
                    </>
                  ) : s.limit_bytes ? (
                    <span className="text-ink-muted">
                      {t("system.upTo")} {formatBytes(s.limit_bytes)}
                    </span>
                  ) : (
                    "—"
                  )}
                </span>
              </div>
              {pct !== null && (
                <div role="progressbar" aria-label={t(`system.svc.${s.name}`)} aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct} className="mt-1 h-1 overflow-hidden rounded-full bg-canvas">
                  <div className={`h-full rounded-full ${pct >= 85 ? "bg-danger" : "bg-ink"}`} style={{ width: `${pct}%` }} />
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
