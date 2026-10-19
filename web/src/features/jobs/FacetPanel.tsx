import { Search } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
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
  focusCountry = false,
}: {
  search: JobSearch;
  facets?: Facets;
  onChange: (changes: Partial<JobSearch>) => void;
  focusCountry?: boolean;
}) {
  const { t, locale } = useI18n();
  const [place, setPlace] = useState(search.location);
  useEffect(() => setPlace(search.location), [search.location]);
  const toggle = (list: string[], v: string) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v]);
  const countries = Object.entries(facets?.country ?? {}).map(([code, n]) => ({ value: code, label: countryName(code, locale), count: n }));
  // A selected value stays listed even when the other filters leave it no jobs.
  for (const code of search.countries) {
    if (!countries.some((c) => c.value === code)) countries.push({ value: code, label: countryName(code, locale), count: 0 });
  }

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
      {countries.length > 0 && (
        <FilterList
          title={t("browse.country")}
          items={countries}
          selected={search.countries}
          locale={locale}
          autoFocus={focusCountry}
          onToggle={(v) => onChange({ countries: toggle(search.countries, v) })}
        />
      )}
      {(facets?.technologies.length ?? 0) > 0 && (
        <FilterList
          title={t("browse.technology")}
          items={facets!.technologies.map((f) => ({ value: f.value, label: f.label, count: f.count }))}
          selected={search.technologies}
          locale={locale}
          onToggle={(v) => onChange({ technologies: toggle(search.technologies, v) })}
        />
      )}
      {(facets?.companies.length ?? 0) > 0 && (
        <FilterList
          title={t("browse.company")}
          items={facets!.companies.map((f) => ({ value: f.value, label: f.label, count: f.count }))}
          selected={search.companies}
          locale={locale}
          onToggle={(v) => onChange({ companies: toggle(search.companies, v) })}
        />
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
      <div className="space-y-1.5">{children}</div>
    </fieldset>
  );
}

const optionClass = "flex min-h-8 cursor-pointer items-center gap-2 rounded-control px-2 text-sm text-ink hover:bg-canvas";

interface Item {
  value: string;
  label: string;
  count: number;
}

// FilterList is a checkbox list with its own search box: selected values come
// first, the rest follow by count, and typing narrows the list.
function FilterList({
  title,
  items,
  selected,
  locale,
  autoFocus,
  onToggle,
}: {
  title: string;
  items: Item[];
  selected: string[];
  locale: string;
  autoFocus?: boolean;
  onToggle: (value: string) => void;
}) {
  const { t } = useI18n();
  const [text, setText] = useState("");
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (autoFocus) input.current?.focus();
  }, [autoFocus]);
  const needle = text.trim().toLocaleLowerCase(locale);
  const shown = items
    .filter((i) => !needle || i.label.toLocaleLowerCase(locale).includes(needle) || i.value.toLowerCase() === needle)
    .sort((a, b) => Number(selected.includes(b.value)) - Number(selected.includes(a.value)) || b.count - a.count || a.label.localeCompare(b.label, locale));
  return (
    <Group title={title}>
      <div className="relative">
        <Search aria-hidden className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-ink-muted" />
        <input
          ref={input}
          type="search"
          value={text}
          onChange={(e) => setText(e.target.value)}
          aria-label={t("browse.searchIn", { name: title })}
          placeholder={t("browse.searchIn", { name: title })}
          className="min-h-9 w-full rounded-control border border-line-strong bg-surface pl-9 pr-2 text-sm text-ink placeholder:text-ink-muted"
        />
      </div>
      <div className="max-h-56 space-y-0.5 overflow-y-auto pr-1">
        {shown.length === 0 && <p className="px-2 py-1 text-sm text-ink-muted">{t("browse.noMatch")}</p>}
        {shown.map((i) => (
          <label key={i.value} className={optionClass}>
            <input type="checkbox" checked={selected.includes(i.value)} onChange={() => onToggle(i.value)} className="accent-[var(--color-ink)]" />
            <span className="truncate">{i.label}</span>
            <span className="ml-auto text-xs tabular-nums text-ink-muted">{i.count.toLocaleString()}</span>
          </label>
        ))}
      </div>
    </Group>
  );
}
