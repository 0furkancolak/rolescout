import { useEffect, useState } from "react";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  API_BASE,
  data,
  query,
  request,
  type Analysis,
  type Application,
  type Facets,
  type Job,
  type JobDetail,
  type Page,
  type SavedJob,
  type WorkspaceJob,
} from "../../lib/api";
import type { JobSearch } from "./searchParams";
import { wsPath } from "../workspaces/context";

export interface JobFilters {
  q?: string;
  work_arrangement?: string;
  employment_type?: string;
  seniority?: string;
  technology?: string;
}

export const PAGE_SIZE = 20;

export function useJobSearch(filters: JobFilters) {
  return useInfiniteQuery({
    queryKey: ["jobs", "search", filters],
    queryFn: ({ pageParam, signal }) =>
      request<Page<Job>>(
        "/jobs" + query({ ...filters, limit: String(PAGE_SIZE), cursor: pageParam ?? undefined }),
        { signal },
      ),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.next_cursor,
  });
}

export function useJob(jobID: string) {
  return useQuery({
    queryKey: ["jobs", "detail", jobID],
    queryFn: ({ signal }) => data<JobDetail>(`/jobs/${jobID}`, { signal }),
    enabled: jobID !== "",
    staleTime: 60_000,
  });
}

export function savedJobsKey(workspaceID: string) {
  return ["workspaces", workspaceID, "saved-jobs"] as const;
}

export function useSavedJobs(workspaceID: string) {
  return useQuery({
    queryKey: savedJobsKey(workspaceID),
    queryFn: ({ signal }) => data<SavedJob[]>(wsPath(workspaceID, "/saved-jobs"), { signal }),
  });
}

export function useToggleSaved(workspaceID: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ jobID, saved }: { jobID: string; saved: boolean }) =>
      request<void>(wsPath(workspaceID, `/saved-jobs/${jobID}`), { method: saved ? "PUT" : "DELETE" }),
    onSettled: () => client.invalidateQueries({ queryKey: savedJobsKey(workspaceID) }),
  });
}

// Workspace job browsing (the job browser).

export const BROWSE_PAGE_SIZE = 25;

function browseParams(s: JobSearch, extra: Record<string, string | undefined> = {}) {
  return query({
    q: s.q || undefined,
    profile: s.profile || undefined,
    sort: s.sort || undefined,
    posted: s.posted || undefined,
    arrangement: s.arrangement || undefined,
    seniority: s.seniority || undefined,
    technology: s.technologies,
    country: s.countries,
    location: s.location || undefined,
    region: s.region || undefined,
    ats: s.ats || undefined,
    company: s.companies,
    closed: s.closed ? "true" : undefined,
    ...extra,
  });
}

export function workspaceJobsKey(ws: string) {
  return ["workspaces", ws, "jobs"] as const;
}

// scoreProfile is the profile that rates the jobs when the list is not limited
// to one profile's matches (all jobs): every job still shows how it fits.
export function useWorkspaceJobs(ws: string, s: JobSearch, enabled = true, scoreProfile = "") {
  const { job: _selected, ...key } = s;
  return useInfiniteQuery({
    queryKey: [...workspaceJobsKey(ws), "browse", key, scoreProfile],
    queryFn: ({ pageParam, signal }) =>
      request<Page<WorkspaceJob>>(
        wsPath(ws, "/jobs") +
          browseParams(s, {
            limit: String(BROWSE_PAGE_SIZE),
            cursor: pageParam ?? undefined,
            score_profile: s.profile ? undefined : scoreProfile || undefined,
          }),
        { signal },
      ),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.next_cursor,
    enabled,
  });
}

export function useFacets(ws: string, s: JobSearch, enabled = true) {
  const { job: _selected, sort: _sort, ...key } = s;
  return useQuery({
    queryKey: [...workspaceJobsKey(ws), "facets", key],
    queryFn: ({ signal }) => data<Facets>(wsPath(ws, "/jobs/facets") + browseParams({ ...s, sort: "" }), { signal }),
    enabled,
    placeholderData: (prev) => prev,
  });
}

// useSaveFromBrowser saves or unsaves a job and refreshes the job lists.
export function useSaveFromBrowser(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ jobID, saved }: { jobID: string; saved: boolean }) =>
      request<void>(wsPath(ws, `/saved-jobs/${jobID}`), { method: saved ? "PUT" : "DELETE" }),
    onSettled: () => {
      void client.invalidateQueries({ queryKey: savedJobsKey(ws) });
      void client.invalidateQueries({ queryKey: workspaceJobsKey(ws) });
    },
  });
}

export interface JobSummary {
  /** The text so far (it grows while status is "streaming"). */
  text: string;
  /** loading: nothing yet; streaming: the model is writing; done: checked and stored; unavailable: none (model off or answer rejected). */
  status: "loading" | "streaming" | "done" | "unavailable";
  /** True when the text was built from the listing's own fields, not written by the model. */
  facts?: boolean;
}

const finished = new Map<string, { text: string; facts: boolean }>();

/** Forgets finished summaries (tests). */
export const resetSummaryCache = () => finished.clear();

// parseEvents reads server-sent events from a stream and calls on for each.
async function parseEvents(body: ReadableStream<Uint8Array>, on: (event: string, data: unknown) => void) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { done, value } = await reader.read();
    buffer += decoder.decode(value, { stream: !done });
    let end: number;
    while ((end = buffer.indexOf("\n\n")) >= 0) {
      const block = buffer.slice(0, end);
      buffer = buffer.slice(end + 2);
      let event = "message";
      let payload = "";
      for (const line of block.split("\n")) {
        if (line.startsWith("event:")) event = line.slice(6).trim();
        else if (line.startsWith("data:")) payload += line.slice(5).trim();
      }
      try {
        on(event, payload ? JSON.parse(payload) : {});
      } catch {
        /* a malformed event is skipped */
      }
    }
    if (done) return;
  }
}

// useJobSummary streams the model's short summary in the given language: the
// text grows as the model writes it. A summary written before arrives at once.
export function useJobSummary(ws: string, jobID: string, lang: string, enabled = true): JobSummary {
  const key = `${ws}/${jobID}/${lang}`;
  const [state, setState] = useState<{ key: string } & JobSummary>({ key: "", text: "", status: "loading" });
  useEffect(() => {
    if (!enabled || jobID === "" || finished.has(key)) return;
    const controller = new AbortController();
    const set = (text: string, status: JobSummary["status"], facts = false) => setState({ key, text, status, facts });
    let text = "";
    (async () => {
      try {
        const res = await fetch(`${API_BASE}${wsPath(ws, `/jobs/${jobID}/summary/stream`)}${query({ lang })}`, {
          headers: { Accept: "text/event-stream" },
          credentials: "same-origin",
          signal: controller.signal,
        });
        if (!res.ok || !res.body) return set("", "unavailable");
        await parseEvents(res.body, (event, payload) => {
          const p = payload as { text?: string; summary?: string; source?: string };
          if (event === "delta" && p.text) {
            text += p.text;
            set(text, "streaming");
          } else if (event === "done" && p.summary) {
            const facts = p.source === "facts";
            finished.set(key, { text: p.summary, facts });
            set(p.summary, "done", facts);
          } else if (event === "unavailable") {
            set("", "unavailable");
          }
        });
        // The stream ended without a verdict (connection dropped).
        setState((cur) => (cur.key === key && cur.status === "streaming" ? { key, text: "", status: "unavailable" } : cur));
      } catch (err) {
        if (!(err instanceof DOMException && err.name === "AbortError")) set("", "unavailable");
      }
    })();
    return () => controller.abort();
  }, [key, enabled, jobID, lang, ws]);

  const cached = finished.get(key);
  if (cached !== undefined) return { text: cached.text, status: "done", facts: cached.facts };
  if (state.key !== key) return { text: "", status: "loading" };
  return { text: state.text, status: state.status, facts: state.facts };
}

// useApplyToJob records an application with status "applied".
export function useApplyToJob(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (jobID: string) => {
      const app = await data<Application>(wsPath(ws, "/applications"), { method: "POST", body: { job_id: jobID } });
      return data<Application>(wsPath(ws, `/applications/${app.id}`), { method: "PATCH", body: { status: "applied" } });
    },
    onSuccess: () => client.invalidateQueries({ queryKey: ["workspaces", ws, "applications"] }),
  });
}

/** Asks the model to read one job and say why it matches the profile, at any score. */
export function useAnalyzeJob(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ jobID, profileID, lang }: { jobID: string; profileID: string; lang: string }) =>
      data<Analysis>(wsPath(ws, `/jobs/${jobID}/analyze`) + query({ profile: profileID, lang }), { method: "POST" }),
    onSuccess: () => client.invalidateQueries({ queryKey: workspaceJobsKey(ws) }),
  });
}
