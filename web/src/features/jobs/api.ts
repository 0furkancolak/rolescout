import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { data, query, request, type JobDetail, type Page, type Job, type SavedJob } from "../../lib/api";
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
