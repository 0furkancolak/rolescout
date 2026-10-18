import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { Sparkline } from "./Sparkline";

// A big number with a small muted label and an optional trend line.
export function StatCard({
  label,
  value,
  delta,
  series,
  icon,
  href,
  chart,
}: {
  label: string;
  value: string;
  delta?: string;
  series?: number[];
  icon?: ReactNode;
  href?: string;
  chart?: ReactNode;
}) {
  const body = (
    <>
      <div className="flex items-center justify-between gap-2 text-sm text-ink-muted">
        <span>{label}</span>
        {icon && <span aria-hidden="true">{icon}</span>}
      </div>
      <div className="mt-3 flex items-end justify-between gap-3">
        <div>
          <div className="text-4xl font-bold tabular-nums tracking-tight text-ink">{value}</div>
          {delta && <div className="mt-1 text-xs text-ink-muted">{delta}</div>}
        </div>
        {chart ?? (series && <Sparkline values={series} label={label} />)}
      </div>
    </>
  );
  const cls = "block rounded-card bg-surface p-5 shadow-card";
  return href ? (
    <Link to={href} className={`${cls} transition-shadow hover:shadow-raised focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus`}>
      {body}
    </Link>
  ) : (
    <div className={cls}>{body}</div>
  );
}
