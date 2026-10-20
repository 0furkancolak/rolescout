import { useRef, type KeyboardEvent, type ReactNode } from "react";

export interface TabItem<T extends string> {
  value: T;
  label: string;
}

// Tabs is a WAI-ARIA tab list. Arrow keys, Home and End move between tabs;
// only the selected tab is in the tab order. Render the panel with TabPanel.
export function Tabs<T extends string>({
  label,
  idPrefix,
  value,
  items,
  onChange,
}: {
  label: string;
  idPrefix: string;
  value: T;
  items: TabItem<T>[];
  onChange: (v: T) => void;
}) {
  const refs = useRef<Array<HTMLButtonElement | null>>([]);

  function onKeyDown(e: KeyboardEvent) {
    let to = -1;
    const at = items.findIndex((i) => i.value === value);
    if (e.key === "Home") to = 0;
    else if (e.key === "End") to = items.length - 1;
    else if (e.key === "ArrowRight") to = (at + 1) % items.length;
    else if (e.key === "ArrowLeft") to = (at - 1 + items.length) % items.length;
    if (to < 0) return;
    e.preventDefault();
    onChange(items[to].value);
    refs.current[to]?.focus();
  }

  return (
    <div role="tablist" aria-label={label} onKeyDown={onKeyDown} className="flex gap-1 overflow-x-auto border-b border-line">
      {items.map((i, n) => (
        <button
          key={i.value}
          ref={(el) => {
            refs.current[n] = el;
          }}
          id={`${idPrefix}-tab-${i.value}`}
          type="button"
          role="tab"
          aria-selected={i.value === value}
          aria-controls={`${idPrefix}-panel-${i.value}`}
          tabIndex={i.value === value ? 0 : -1}
          onClick={() => onChange(i.value)}
          className={`-mb-px min-h-11 shrink-0 border-b-2 px-4 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus ${
            i.value === value ? "border-ink text-ink" : "border-transparent text-ink-muted hover:text-ink"
          }`}
        >
          {i.label}
        </button>
      ))}
    </div>
  );
}

export function TabPanel({ idPrefix, value, children }: { idPrefix: string; value: string; children: ReactNode }) {
  return (
    <div role="tabpanel" id={`${idPrefix}-panel-${value}`} aria-labelledby={`${idPrefix}-tab-${value}`} tabIndex={0} className="space-y-6">
      {children}
    </div>
  );
}
