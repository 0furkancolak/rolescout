import { useI18n } from "../lib/i18n";

const bandColor = (v: number) => (v >= 80 ? "var(--color-sage-deep)" : v >= 60 ? "var(--color-amber-deep)" : "var(--color-lavender-deep)");

// A match score as a progress ring. The number is always shown, so colour is
// never the only signal.
export function ScoreRing({ value, size = 40 }: { value: number; size?: number }) {
  const { t } = useI18n();
  const v = Math.round(Math.min(100, Math.max(0, Number.isFinite(value) ? value : 0)));
  const stroke = Math.max(3, size / 10);
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  return (
    <span className="relative inline-flex shrink-0 items-center justify-center" style={{ width: size, height: size }}>
      <svg role="img" aria-label={t("score.label", { n: v })} width={size} height={size} viewBox={`0 0 ${size} ${size}`} className="-rotate-90">
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--color-line)" strokeWidth={stroke} />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke={bandColor(v)}
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={c}
          strokeDashoffset={c * (1 - v / 100)}
          className="motion-safe:transition-[stroke-dashoffset] motion-safe:duration-500"
        />
      </svg>
      <span aria-hidden="true" className="absolute font-bold tabular-nums text-ink" style={{ fontSize: size * 0.32 }}>
        {v}
      </span>
    </span>
  );
}
