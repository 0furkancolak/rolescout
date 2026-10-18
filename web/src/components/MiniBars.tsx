import { seriesLabel } from "../lib/seriesLabel";

// Small vertical bars, one per value, scaled to the largest.
export function MiniBars({ values, label, width = 96, height = 32 }: { values: number[]; label: string; width?: number; height?: number }) {
  const max = Math.max(...values, 1);
  const gap = 3;
  const bw = values.length ? Math.max(2, (width - gap * (values.length - 1)) / values.length) : 0;
  return (
    <svg role="img" aria-label={seriesLabel(label, values)} width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="text-ink">
      <line x1={0} x2={width} y1={height - 0.5} y2={height - 0.5} stroke="var(--color-line)" />
      {values.map((v, i) => {
        const h = Math.max(v > 0 ? 3 : 0, (v / max) * (height - 2));
        return <rect key={i} x={i * (bw + gap)} y={height - 1 - h} width={bw} height={h} rx={Math.min(3, bw / 2)} fill="currentColor" />;
      })}
    </svg>
  );
}
