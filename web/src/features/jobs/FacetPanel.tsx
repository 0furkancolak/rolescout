import { useEffect, useState, type ReactNode } from "react";
import type { Facets } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { countryName } from "../../lib/countries";
import { inputClass } from "../../components/styles";
import { atsList, regions, type JobSearch } from "./searchParams";

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
  const { t, locale } = useI18n();
  const [place, setPlace] = useState(search.location);
  useEffect(() => setPlace(search.location), [search.location]);
  const toggle = (list: string[], v: string) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v]);
  const count = (n?: number) => <span className="ml-auto text-xs tabular-nums text-ink-muted">{(n ?? 0).toLocaleString()}</span>;

  const countries = Object.entries(facets?.country ?? {}).sort((a, b) => b[1] - a[1]).slice(0, 15);

  return (
    <div className="space-y-5">
      <Group title={t("browse.place")}>
        <form
          role="search"
          onSubmit={(e) => {
            e.preventDefault();
            onChange({ location: place.trim() });
          }}
        >
          <label htmlFor="browse-location" className="sr-only">
            {t("browse.location")}
          </label>
          <input
            id="browse-location"
            type="search"
            value={place}
            maxLength={80}
            onChange={(e) => setPlace(e.target.value)}
            onBlur={() => place.trim() !== search.location && onChange({ location: place.trim() })}
            placeholder={t("browse.locationPlaceholder")}
            className={inputClass}
          />
        </form>
        <select
          aria-label={t("browse.region")}
          value={search.region}
          onChange={(e) => onChange({ region: e.target.value })}
          className="min-h-10 w-full rounded-control border border-line-strong bg-surface px-2 text-sm text-ink"
        >
          <option value="">{t("browse.anyRegion")}</option>
          {regions.map((r) => (
            <option key={r} value={r}>
              {t(`region.${r}`)}
            </option>
          ))}
        </select>
        <select
          aria-label={t("browse.ats")}
          value={search.ats}
          onChange={(e) => onChange({ ats: e.target.value })}
          className="min-h-10 w-full rounded-control border border-line-strong bg-surface px-2 text-sm text-ink"
        >
          <option value="">{t("browse.anyAts")}</option>
          {atsList.map((a) => (
            <option key={a} value={a}>
              {t(`ats.${a}`)}
            </option>
          ))}
        </select>
      </Group>
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
              {countryName(k, locale)}
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
