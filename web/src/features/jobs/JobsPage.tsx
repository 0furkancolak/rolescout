import { Globe, Search, SlidersHorizontal } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Drawer } from "../../components/Drawer";
import { Segmented } from "../../components/Segmented";
import { Button, EmptyState, ErrorState, PageTitle, Spinner } from "../../components/ui";
import { SkeletonCards } from "../../components/Skeleton";
import { buttonClass, inputClass } from "../../components/styles";
import type { WorkspaceJob } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useProfiles } from "../research/api";
import { useUserState, usePutUserState } from "../state/api";
import { useWorkspace } from "../workspaces/context";
import { useFacets, useJob, useSaveFromBrowser, useWorkspaceJobs } from "./api";
import { FacetPanel } from "./FacetPanel";
import { FilterChips } from "./FilterChips";
import { JobDetailPane } from "./JobDetailPane";
import { JobList } from "./JobList";
import { effectiveSearch, readJobSearch, writeJobSearch, type JobSearch, type Sort } from "./searchParams";
import { useJobKeyboard } from "./useJobKeyboard";

// JobsPage is the job browser: results beside the selected job, filters in a
// drawer. All state lives in the URL, and the last view is also kept on the
// server so the next visit (on any device) continues where the user left off.
export function JobsPage() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const [params, setParams] = useSearchParams();
  const raw = readJobSearch(params);
  const profiles = useProfiles(workspace.id);
  const ready = !profiles.isPending;
  const search = effectiveSearch(raw, profiles.data?.[0]?.id);
  // Browsing all jobs: they are still rated against the first profile.
  const scoreProfile = search.profile ? "" : (profiles.data?.[0]?.id ?? "");
  const jobs = useWorkspaceJobs(workspace.id, search, ready, scoreProfile);
  const facets = useFacets(workspace.id, search, ready);
  const save = useSaveFromBrowser(workspace.id);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [focusCountry, setFocusCountry] = useState(false);
  useResumeView(workspace.id, params, setParams);
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

  // A restored selection may lie beyond the first page: load a few more.
  const loadedPages = jobs.data?.pages.length ?? 0;
  useEffect(() => {
    if (raw.job && !fromList && jobs.hasNextPage && !jobs.isFetchingNextPage && loadedPages < 6) void jobs.fetchNextPage();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [raw.job, fromList, jobs.hasNextPage, jobs.isFetchingNextPage, loadedPages]);

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
  const activeFilters =
    [raw.posted, raw.arrangement, raw.seniority, raw.location, raw.region, raw.ats].filter(Boolean).length +
    raw.technologies.length + raw.countries.length + raw.companies.length + (raw.closed ? 1 : 0);
  const hasFilters = !!(raw.posted || raw.arrangement || raw.seniority || raw.technologies.length || raw.countries.length || raw.location || raw.region || raw.ats || raw.companies.length || raw.closed);
  const sorts: Exclude<Sort, "">[] = search.profile ? ["score", "recent"] : ["recent"];
  if (search.q) sorts.push("relevance");

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <PageTitle>{t("browse.title")}</PageTitle>
        <p className="hidden text-sm text-ink-muted lg:block">{t("browse.keyboardHint")}</p>
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
          className={`${inputClass} flex-1 rounded-full! px-5!`}
        />
        <Button type="submit">
          <Search aria-hidden className="size-4" />
          {t("jobs.search")}
        </Button>
      </form>

      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-sm">
        <div className="flex w-full items-center gap-2 sm:w-auto sm:gap-3">
          <Segmented
            label={t("browse.sort")}
            value={search.sort}
            options={sorts.map((s) => ({ value: s, label: t(`browse.sort.${s}`) }))}
            onChange={(s) => update({ sort: s, job: "" })}
          />
          <label className="inline-flex min-w-0 flex-1 items-center gap-2 text-ink-muted sm:flex-none">
            <span className="hidden sm:inline">{t("browse.profile")}</span>
            <select
              aria-label={t("browse.profile")}
              value={search.profile || "all"}
              onChange={(e) => update({ profile: e.target.value, sort: "", job: "" })}
              className="min-h-10 w-full min-w-0 rounded-control border border-line-strong bg-surface px-2 py-1.5 text-ink sm:w-auto"
            >
              {profiles.data?.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
              <option value="all">{t("browse.allJobs")}</option>
            </select>
          </label>
        </div>
        <div className="flex w-full items-center gap-2 sm:w-auto">
          <button
            type="button"
            onClick={() => {
              setFocusCountry(true);
              setFiltersOpen(true);
            }}
            aria-haspopup="dialog"
            className={buttonClass("secondary", "flex-1 sm:flex-none")}
          >
            <Globe aria-hidden className="size-4" />
            {t("browse.country")}
            {raw.countries.length > 0 && (
              <span className="rounded-full bg-ink px-1.5 text-xs tabular-nums text-ink-inverse">{raw.countries.length}</span>
            )}
          </button>
          <button
            type="button"
            onClick={() => {
              setFocusCountry(false);
              setFiltersOpen(true);
            }}
            aria-haspopup="dialog"
            className={buttonClass("secondary", "flex-1 sm:flex-none")}
          >
            <SlidersHorizontal aria-hidden className="size-4" />
            {t("browse.filters")}
            {activeFilters > 0 && (
              <span className="rounded-full bg-ink px-1.5 text-xs tabular-nums text-ink-inverse">{activeFilters}</span>
            )}
          </button>
          {hasFilters && (
            <Button
              variant="ghost"
              onClick={() => update({ posted: "", arrangement: "", seniority: "", technologies: [], countries: [], location: "", region: "", ats: "", companies: [], closed: false })}
            >
              {t("browse.clear")}
            </Button>
          )}
        </div>
      </div>

      <FilterChips search={raw} facets={facets.data} onChange={(c) => update({ ...c, job: "" })} />

      <Drawer open={filtersOpen} title={t("browse.filters")} onClose={() => setFiltersOpen(false)}>
        <FacetPanel search={raw} facets={facets.data} focusCountry={focusCountry} onChange={(c) => update({ ...c, job: "" })} />
      </Drawer>

      <div className="grid grid-cols-[minmax(0,1fr)] gap-4 lg:grid-cols-[minmax(20rem,27rem)_minmax(0,1fr)]">
        <div className={selected ? "hidden lg:block" : ""}>
          {!ready || jobs.isPending ? (
            <SkeletonCards count={7} />
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
                <MoreSentinel busy={jobs.isFetchingNextPage} onVisible={() => !jobs.isFetchingNextPage && void jobs.fetchNextPage()} />
              )}
            </>
          )}
        </div>

        <div className={selected ? "" : "hidden lg:block"}>
          {selected ? (
            <JobDetailPane job={selected} profileId={search.profile || scoreProfile} onBack={() => update({ job: "" }, true)} />
          ) : (
            <p className="rounded-hero border border-dashed border-line-strong p-8 text-center text-sm text-ink-muted">
              {t("browse.selectJob")}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

// MoreSentinel loads the next page when it scrolls into view. Where
// IntersectionObserver is missing it falls back to a button.
function MoreSentinel({ busy, onVisible }: { busy: boolean; onVisible: () => void }) {
  const { t } = useI18n();
  const ref = useRef<HTMLDivElement>(null);
  const supported = typeof IntersectionObserver !== "undefined";
  useEffect(() => {
    const el = ref.current;
    if (!el || !supported) return;
    const io = new IntersectionObserver((entries) => entries.some((e) => e.isIntersecting) && onVisible(), { rootMargin: "600px" });
    io.observe(el);
    return () => io.disconnect();
  });
  return (
    <div ref={ref} className="mt-3 flex min-h-12 justify-center">
      {busy ? (
        <Spinner />
      ) : (
        !supported && (
          <Button variant="secondary" onClick={onVisible}>
            {t("jobs.loadMore")}
          </Button>
        )
      )}
    </div>
  );
}

interface ViewState {
  query: string;
}

// useResumeView keeps the job browser's URL (filters, sort, selected job) on
// the server. A visit to the bare page restores it; later changes are saved.
function useResumeView(ws: string, params: URLSearchParams, setParams: (p: URLSearchParams, o?: { replace: boolean }) => void) {
  const saved = useUserState<ViewState>(ws, "jobs_view");
  const put = usePutUserState<ViewState>(ws, "jobs_view");
  const ready = useRef(false);
  const query = params.toString();
  useEffect(() => {
    if (ready.current || saved.isPending) return;
    ready.current = true;
    if (!query && saved.data?.query) setParams(new URLSearchParams(saved.data.query), { replace: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [saved.isPending]);
  useEffect(() => {
    if (!ready.current || saved.isPending || (saved.data?.query ?? "") === query) return;
    const timer = setTimeout(() => put.mutate({ query }), 700);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, saved.isPending, saved.data?.query]);
}
