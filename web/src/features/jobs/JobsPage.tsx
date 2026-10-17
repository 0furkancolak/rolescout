import { Search, SlidersHorizontal } from "lucide-react";
import { useMemo, useRef, useState, type FormEvent } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Button, EmptyState, ErrorState, PageTitle, Spinner } from "../../components/ui";
import { buttonClass, inputClass } from "../../components/styles";
import type { WorkspaceJob } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useProfiles } from "../research/api";
import { useWorkspace } from "../workspaces/context";
import { useFacets, useJob, useSaveFromBrowser, useWorkspaceJobs } from "./api";
import { FacetPanel } from "./FacetPanel";
import { JobDetailPane } from "./JobDetailPane";
import { JobList } from "./JobList";
import { effectiveSearch, readJobSearch, writeJobSearch, type JobSearch, type Sort } from "./searchParams";
import { useJobKeyboard } from "./useJobKeyboard";

// JobsPage is the three-column job browser: filters, results and the
// selected job. All state lives in the URL.
export function JobsPage() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const [params, setParams] = useSearchParams();
  const raw = readJobSearch(params);
  const profiles = useProfiles(workspace.id);
  const ready = !profiles.isPending;
  const search = effectiveSearch(raw, profiles.data?.[0]?.id);
  const jobs = useWorkspaceJobs(workspace.id, search, ready);
  const facets = useFacets(workspace.id, search, ready);
  const save = useSaveFromBrowser(workspace.id);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [text, setText] = useState(raw.q);
  const searchInput = useRef<HTMLInputElement>(null);

  const update = (changes: Partial<JobSearch>, replace = false) => {
    setParams(writeJobSearch({ ...raw, ...changes }), { replace });
  };
  const list = useMemo(() => jobs.data?.pages.flatMap((p) => p.data) ?? [], [jobs.data]);
  const fromList = list.find((j) => j.id === raw.job);
  // The detail query is shared with the pane; it also covers a selected job
  // that is not in the loaded list (such as a link from the dashboard).
  const detail = useJob(raw.job);
  const selected: WorkspaceJob | undefined =
    fromList ?? (detail.data && jobs.isSuccess ? { ...detail.data, saved: false, match: null } : undefined);
  const careerURL = detail.data?.id === raw.job ? detail.data.listings[0]?.url : undefined;

  const keyboard = useMemo(() => {
    const index = list.findIndex((j) => j.id === raw.job);
    const move = (d: number) => {
      const next = list[Math.min(list.length - 1, Math.max(0, index + d))];
      if (next) setParams(writeJobSearch({ ...raw, job: next.id }), { replace: true });
    };
    return {
      next: () => move(index < 0 ? 0 : 1),
      previous: () => move(-1),
      save: () => selected && save.mutate({ jobID: selected.id, saved: !selected.saved }),
      open: () => {
        if (careerURL) window.open(careerURL, "_blank", "noopener,noreferrer");
      },
      search: () => searchInput.current?.focus(),
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [list, raw.job, selected?.id, selected?.saved, careerURL, params]);
  useJobKeyboard(keyboard);

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    update({ q: text.trim(), job: "", sort: text.trim() && raw.sort === "relevance" ? "relevance" : raw.sort });
  };
  const hasFilters = !!(raw.posted || raw.arrangement || raw.seniority || raw.technologies.length || raw.countries.length || raw.companies.length || raw.closed);
  const sorts: Exclude<Sort, "">[] = search.profile ? ["score", "recent"] : ["recent"];
  if (search.q) sorts.push("relevance");

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <PageTitle>{t("browse.title")}</PageTitle>
        <p className="text-sm text-slate-500">{t("browse.keyboardHint")}</p>
      </div>

      <form role="search" onSubmit={onSubmit} className="flex flex-col gap-2 sm:flex-row">
        <label htmlFor="browse-q" className="sr-only">
          {t("browse.search")}
        </label>
        <input
          id="browse-q"
          ref={searchInput}
          type="search"
          value={text}
          maxLength={200}
          placeholder={t("browse.searchPlaceholder")}
          onChange={(e) => setText(e.target.value)}
          className={`${inputClass} flex-1`}
        />
        <Button type="submit">
          <Search aria-hidden className="size-4" />
          {t("jobs.search")}
        </Button>
      </form>

      <div className="flex flex-wrap items-center gap-2 text-sm">
        <label className="inline-flex items-center gap-2 text-slate-600">
          {t("browse.profile")}
          <select
            value={search.profile || "all"}
            onChange={(e) => update({ profile: e.target.value, sort: "", job: "" })}
            className="rounded-md border border-slate-300 bg-white px-2 py-1.5 text-slate-900"
          >
            {profiles.data?.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
            <option value="all">{t("browse.allJobs")}</option>
          </select>
        </label>
        <label className="inline-flex items-center gap-2 text-slate-600">
          {t("browse.sort")}
          <select
            value={search.sort}
            onChange={(e) => update({ sort: e.target.value as Sort, job: "" })}
            className="rounded-md border border-slate-300 bg-white px-2 py-1.5 text-slate-900"
          >
            {sorts.map((s) => (
              <option key={s} value={s}>
                {t(`browse.sort.${s}`)}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          onClick={() => setFiltersOpen((o) => !o)}
          aria-expanded={filtersOpen}
          className={buttonClass("secondary", "lg:hidden")}
        >
          <SlidersHorizontal aria-hidden className="size-4" />
          {t("browse.filters")}
        </button>
        {hasFilters && (
          <Button
            variant="ghost"
            onClick={() => update({ posted: "", arrangement: "", seniority: "", technologies: [], countries: [], companies: [], closed: false })}
          >
            {t("browse.clear")}
          </Button>
        )}
      </div>

      <div className="grid gap-4 lg:grid-cols-[15rem_minmax(0,1fr)_minmax(0,1.3fr)]">
        <aside aria-label={t("browse.filters")} className={`${filtersOpen ? "block" : "hidden"} lg:block`}>
          <div className="rounded-xl border border-slate-200 bg-white p-4 lg:sticky lg:top-6">
            <FacetPanel search={raw} facets={facets.data} onChange={(c) => update({ ...c, job: "" })} />
          </div>
        </aside>

        <div className={selected ? "hidden lg:block" : ""}>
          {!ready || jobs.isPending ? (
            <Spinner />
          ) : jobs.isError ? (
            <ErrorState error={jobs.error} onRetry={() => jobs.refetch()} />
          ) : list.length === 0 ? (
            <EmptyState title={t("browse.empty")} hint={t("browse.emptyResearch")}>
              <Link to="/research" className={buttonClass("secondary", "mt-2")}>
                {t("nav.research")}
              </Link>
            </EmptyState>
          ) : (
            <>
              <JobList jobs={list} selected={raw.job} onSelect={(id) => update({ job: id }, true)} />
              {jobs.hasNextPage && (
                <div className="mt-3 flex justify-center">
                  <Button variant="secondary" busy={jobs.isFetchingNextPage} onClick={() => jobs.fetchNextPage()}>
                    {t("jobs.loadMore")}
                  </Button>
                </div>
              )}
            </>
          )}
        </div>

        <div className={selected ? "" : "hidden lg:block"}>
          {selected ? (
            <div className="lg:sticky lg:top-6">
              <JobDetailPane job={selected} onBack={() => update({ job: "" }, true)} />
            </div>
          ) : (
            <p className="rounded-xl border border-dashed border-slate-300 p-8 text-center text-sm text-slate-500">
              {t("browse.selectJob")}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
