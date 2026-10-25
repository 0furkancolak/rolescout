import { Pill } from "../../components/Pill";
import type { Facets } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { countryName } from "../../lib/countries";
import { enumLabel } from "../../lib/labels";
import type { JobSearch, Posted } from "./searchParams";

const posted: Exclude<Posted, "">[] = ["24h", "7d", "30d"];

// FilterChips are the quick filters above the list: when it was posted, work
// arrangement and seniority. Each group allows one value; pressing the active
// chip clears it. They write the same URL params the facet panel used to.
export function FilterChips({
  search,
  facets,
  onChange,
}: {
  search: JobSearch;
  facets?: Facets;
  onChange: (changes: Partial<JobSearch>) => void;
}) {
  const { t, locale } = useI18n();
  const arrangements = Object.entries(facets?.arrangement ?? {}).filter(([k]) => k !== "unknown");
  const seniorities = Object.entries(facets?.seniority ?? {}).filter(([k]) => k !== "unknown");
  const label = (text: string, n?: number) => (n === undefined ? text : `${text} ${n.toLocaleString()}`);

  return (
    <div role="group" aria-label={t("browse.quickFilters")} className="-mx-4 flex items-center gap-2 overflow-x-auto px-4 pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden sm:mx-0 sm:flex-wrap sm:overflow-visible sm:px-0">
      {posted.map((p) => (
        <Pill key={p} selected={search.posted === p} onClick={() => onChange({ posted: search.posted === p ? "" : p })}>
          {label(t(`browse.posted.${p}`), facets?.posted[p])}
        </Pill>
      ))}
      <Pill selected={search.countries.length === 1 && search.countries[0] === "TR"} onClick={() => onChange({ countries: search.countries.includes("TR") ? [] : ["TR"] })}>
        {label(countryName("TR", locale), facets?.country?.TR)}
      </Pill>
      {arrangements.length > 0 && <span aria-hidden className="mx-1 h-5 w-px bg-line-strong" />}
      {arrangements.map(([k, n]) => (
        <Pill key={k} selected={search.arrangement === k} onClick={() => onChange({ arrangement: search.arrangement === k ? "" : k })}>
          {label(enumLabel(t, "work", k), n)}
        </Pill>
      ))}
      {seniorities.length > 0 && <span aria-hidden className="mx-1 h-5 w-px bg-line-strong" />}
      {seniorities.map(([k, n]) => (
        <Pill key={k} selected={search.seniority === k} onClick={() => onChange({ seniority: search.seniority === k ? "" : k })}>
          {label(enumLabel(t, "seniority", k), n)}
        </Pill>
      ))}
    </div>
  );
}
