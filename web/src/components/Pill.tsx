import type { ReactNode } from "react";

const base = "inline-flex min-h-6 shrink-0 items-center gap-1 whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium";

type Tone = "neutral" | "sage" | "amber" | "lavender" | "danger";

const tones: Record<Tone, string> = {
  neutral: "bg-canvas text-ink-muted",
  sage: "bg-sage text-ink",
  amber: "bg-amber text-ink",
  lavender: "bg-lavender text-ink",
  danger: "bg-danger-soft text-danger",
};

// A status pill, or a filter chip when onClick is given (then it is a toggle
// button with aria-pressed).
export function Pill({
  children,
  tone = "neutral",
  selected,
  onClick,
}: {
  children: ReactNode;
  tone?: Tone;
  selected?: boolean;
  onClick?: () => void;
}) {
  if (!onClick) return <span className={`${base} ${tones[tone]}`}>{children}</span>;
  return (
    <button
      type="button"
      aria-pressed={!!selected}
      onClick={onClick}
      className={`${base} min-h-9 cursor-pointer border px-3.5 text-sm transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus ${
        selected ? "border-ink bg-ink text-ink-inverse" : "border-line-strong bg-surface text-ink hover:bg-canvas"
      }`}
    >
      {children}
    </button>
  );
}
