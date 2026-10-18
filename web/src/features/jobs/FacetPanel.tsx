import type { ReactNode } from "react";
import type { Facets } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import type { JobSearch } from "./searchParams";

// FacetPanel offers filter values with how many jobs each one has.
export function FacetPanel({
  search,
  facets,
  onChange,
}: {
  search: JobSearch;
  facets?: Facets;
  onChange: (changes: Partial<JobSearch>) => void;
}) {
  const { t } = useI18n();
  const toggle = (list: string[], v: string) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v]);
  const count = (n?: number) => <span className="ml-auto text-xs tabular-nums text-ink-muted">{(n ?? 0).toLocaleString()}</span>;

  const countries = Object.entries(facets?.country ?? {}).sort((a, b) => b[1] - a[1]).slice(0, 15);

  return (
    <div className="space-y-5">
      {(facets?.technologies.length ?? 0) > 0 && (
        <Group title={t("browse.technology")}>
          {facets!.technologies.map((f) => (
            <Check
              key={f.value}
              checked={search.technologies.includes(f.value)}
              onChange={() => onChange({ technologies: toggle(search.technologies, f.value) })}
            >
              {f.label}
              {count(f.count)}
            </Check>
          ))}
        </Group>
      )}
      {countries.length > 0 && (
        <Group title={t("browse.country")}>
          {countries.map(([k, n]) => (
            <Check key={k} checked={search.countries.includes(k)} onChange={() => onChange({ countries: toggle(search.countries, k) })}>
              {countryName(k)}
              {count(n)}
            </Check>
          ))}
        </Group>
      )}
      {(facets?.companies.length ?? 0) > 0 && (
        <Group title={t("browse.company")}>
          {facets!.companies.map((f) => (
            <Check
              key={f.value}
              checked={search.companies.includes(f.value)}
              onChange={() => onChange({ companies: toggle(search.companies, f.value) })}
            >
              <span className="truncate">{f.label}</span>
              {count(f.count)}
            </Check>
          ))}
        </Group>
      )}
      <label className="flex items-center gap-2 text-sm text-ink">
        <input type="checkbox" checked={search.closed} onChange={(e) => onChange({ closed: e.target.checked })} className="accent-[var(--color-ink)]" />
        {t("browse.closed")}
      </label>
    </div>
  );
}

function countryName(code: string): string {
  try {
    return new Intl.DisplayNames([document.documentElement.lang || "en"], { type: "region" }).of(code) ?? code;
  } catch {
    return code;
  }
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <fieldset>
      <legend className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-ink-muted">{title}</legend>
      <div className="space-y-0.5">{children}</div>
    </fieldset>
  );
}

const optionClass = "flex min-h-8 cursor-pointer items-center gap-2 rounded-control px-2 text-sm text-ink hover:bg-canvas";

function Check({ checked, onChange, children }: { checked: boolean; onChange: () => void; children: ReactNode }) {
  return (
    <label className={optionClass}>
      <input type="checkbox" checked={checked} onChange={onChange} className="accent-[var(--color-ink)]" />
      {children}
    </label>
  );
}
