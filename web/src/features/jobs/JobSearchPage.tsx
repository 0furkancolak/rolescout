import { useState, type FormEvent } from "react";
import { Search } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import { Button, EmptyState, ErrorState, Field, InlineError, PageTitle, Spinner } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { employmentTypes, seniorityLevels, workArrangements } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { enumLabel } from "../../lib/labels";
import { useJobSearch, type JobFilters } from "./api";
import { JobCard } from "./JobCard";

const filterKeys = ["q", "work_arrangement", "employment_type", "seniority", "technology"] as const;

function filtersFrom(params: URLSearchParams): JobFilters {
  const f: JobFilters = {};
  for (const key of filterKeys) {
    const value = params.get(key)?.trim();
    if (value) f[key] = value;
  }
  return f;
}

export function JobSearchPage() {
  const { t } = useI18n();
  const [params, setParams] = useSearchParams();
  const filters = filtersFrom(params);
  const search = useJobSearch(filters);
  const [text, setText] = useState({ q: filters.q ?? "", technology: filters.technology ?? "" });

  const update = (changes: Partial<JobFilters>) => {
    const next = new URLSearchParams(params);
    for (const [key, value] of Object.entries(changes)) {
      if (value && value.trim()) next.set(key, value.trim());
      else next.delete(key);
    }
    setParams(next);
  };

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    update(text);
  };

  const selects = [
    { key: "work_arrangement", label: t("jobs.workArrangement"), values: workArrangements, group: "work" },
    { key: "employment_type", label: t("jobs.employmentType"), values: employmentTypes, group: "employment" },
    { key: "seniority", label: t("jobs.seniority"), values: seniorityLevels, group: "seniority" },
  ] as const;

  const jobs = search.data?.pages.flatMap((p) => p.data) ?? [];
  const hasFilters = Object.keys(filters).length > 0;

  return (
    <div className="space-y-6">
      <PageTitle>{t("jobs.title")}</PageTitle>
      <form role="search" onSubmit={onSubmit} className="space-y-4 rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
          <div className="flex-1">
            <Field id="q" label={t("jobs.searchLabel")}>
              <input
                id="q"
                type="search"
                value={text.q}
                maxLength={200}
                placeholder={t("jobs.searchPlaceholder")}
                onChange={(e) => setText({ ...text, q: e.target.value })}
                className={inputClass}
              />
            </Field>
          </div>
          <Button type="submit">
            <Search aria-hidden className="size-4" />
            {t("jobs.search")}
          </Button>
        </div>
        <fieldset className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <legend className="sr-only">{t("jobs.filters")}</legend>
          {selects.map((s) => (
            <Field key={s.key} id={s.key} label={s.label}>
              <select
                id={s.key}
                value={filters[s.key] ?? ""}
                onChange={(e) => update({ [s.key]: e.target.value })}
                className={inputClass}
              >
                <option value="">{t("common.any")}</option>
                {s.values
                  .filter((v) => v !== "unknown")
                  .map((v) => (
                    <option key={v} value={v}>
                      {enumLabel(t, s.group, v)}
                    </option>
                  ))}
              </select>
            </Field>
          ))}
          <Field id="technology" label={t("jobs.technology")}>
            <input
              id="technology"
              value={text.technology}
              placeholder={t("jobs.technologyPlaceholder")}
              onChange={(e) => setText({ ...text, technology: e.target.value })}
              onBlur={() => update({ technology: text.technology })}
              className={inputClass}
            />
          </Field>
        </fieldset>
        {hasFilters && (
          <Button
            variant="ghost"
            onClick={() => {
              setText({ q: "", technology: "" });
              setParams(new URLSearchParams());
            }}
          >
            {t("jobs.clearFilters")}
          </Button>
        )}
      </form>

      <div aria-live="polite">
        {search.isPending ? (
          <Spinner />
        ) : search.isError && !search.data ? (
          <ErrorState error={search.error} onRetry={() => void search.refetch()} />
        ) : jobs.length === 0 ? (
          <EmptyState title={t("jobs.empty")} hint={t("jobs.emptyHint")} />
        ) : (
          <div className="space-y-3">
            <ul className="grid gap-3 md:grid-cols-2">
              {jobs.map((job) => (
                <li key={job.id}>
                  <JobCard job={job} />
                </li>
              ))}
            </ul>
            {search.isFetchNextPageError && <InlineError error={search.error} />}
            {search.hasNextPage && (
              <div className="flex justify-center">
                <Button variant="secondary" busy={search.isFetchingNextPage} onClick={() => void search.fetchNextPage()}>
                  {t("jobs.loadMore")}
                </Button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
