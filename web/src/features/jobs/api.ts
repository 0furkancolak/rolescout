import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  data,
  query,
  request,
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
    company: s.companies,
    closed: s.closed ? "true" : undefined,
    ...extra,
  });
}

export function workspaceJobsKey(ws: string) {
  return ["workspaces", ws, "jobs"] as const;
}

export function useWorkspaceJobs(ws: string, s: JobSearch, enabled = true) {
  const { job: _selected, ...key } = s;
  return useInfiniteQuery({
    queryKey: [...workspaceJobsKey(ws), "browse", key],
    queryFn: ({ pageParam, signal }) =>
      request<Page<WorkspaceJob>>(
        wsPath(ws, "/jobs") + browseParams(s, { limit: String(BROWSE_PAGE_SIZE), cursor: pageParam ?? undefined }),
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
