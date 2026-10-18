import { seriesLabel } from "../lib/seriesLabel";

// A tiny line chart. Empty or one-point series draw a flat baseline.
export function Sparkline({ values, label, width = 96, height = 32 }: { values: number[]; label: string; width?: number; height?: number }) {
  const pad = 3;
  const max = Math.max(...values, 1);
  const pts = values.length > 1 ? values : [values[0] ?? 0, values[0] ?? 0];
  const step = (width - pad * 2) / (pts.length - 1);
  const y = (v: number) => height - pad - (v / max) * (height - pad * 2);
  const d = pts.map((v, i) => `${i ? "L" : "M"}${(pad + i * step).toFixed(1)} ${y(v).toFixed(1)}`).join(" ");
  return (
    <svg role="img" aria-label={seriesLabel(label, values)} width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="text-sage-deep">
      <path d={d} fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
