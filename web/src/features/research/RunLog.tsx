import { useEffect, useRef } from "react";
import type { RunEvent } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

const levelClass = { info: "text-emerald-300", warn: "text-amber-300", error: "text-red-300" };

// RunLog is the live log beside the stage cards. It follows new lines unless
// the reader has scrolled up.
export function RunLog({ events }: { events: RunEvent[] }) {
  const { t } = useI18n();
  const ref = useRef<HTMLDivElement>(null);
  const stick = useRef(true);
  useEffect(() => {
    const el = ref.current;
    if (el && stick.current) el.scrollTop = el.scrollHeight;
  }, [events.length]);

  return (
    <div
      ref={ref}
      role="log"
      aria-live="polite"
      aria-label={t("run.log")}
      onScroll={(e) => {
        const el = e.currentTarget;
        stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
      }}
      className="h-[28rem] overflow-y-auto rounded-xl bg-[#0b1220] p-3 font-mono text-xs leading-relaxed text-slate-300 lg:h-[36rem]"
      style={{ color: "#cbd5e1" }}
    >
      {events.length === 0 ? (
        <p style={{ color: "#64748b" }}>{t("run.logEmpty")}</p>
      ) : (
        events.map((e) => (
          <div key={e.id} className="whitespace-pre-wrap break-words">
            <span style={{ color: "#64748b" }}>{new Date(e.at).toLocaleTimeString()} </span>
            {e.company && <span className="font-semibold" style={{ color: "#e2e8f0" }}>{e.company} · </span>}
            <span className={levelClass[e.level]}>{e.message}</span>
          </div>
        ))
      )}
    </div>
  );
}
