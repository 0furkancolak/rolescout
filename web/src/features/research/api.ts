import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { data, query, request, type ResearchProfile, type ResearchRun, type RunEvent } from "../../lib/api";
import { wsPath } from "../workspaces/context";

// POLL_MS is how often an active run is refreshed.
export const POLL_MS = import.meta.env.MODE === "test" ? 40 : 2000;

export const isActive = (run?: ResearchRun) => run?.state === "queued" || run?.state === "running";

const profilesKey = (ws: string) => ["workspaces", ws, "research-profiles"] as const;
const runsKey = (ws: string) => ["workspaces", ws, "research-runs"] as const;

export type ProfileInput = Omit<ResearchProfile, "id" | "last_run_at" | "created_at" | "updated_at">;

export function useProfiles(ws: string) {
  return useQuery({
    queryKey: profilesKey(ws),
    queryFn: ({ signal }) => data<ResearchProfile[]>(wsPath(ws, "/research-profiles"), { signal }),
  });
}

export function useProfile(ws: string, id: string) {
  return useQuery({
    queryKey: [...profilesKey(ws), id],
    queryFn: ({ signal }) => data<ResearchProfile>(wsPath(ws, `/research-profiles/${id}`), { signal }),
    enabled: id !== "",
  });
}

export function useSaveProfile(ws: string, id?: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (body: ProfileInput) =>
      data<ResearchProfile>(wsPath(ws, id ? `/research-profiles/${id}` : "/research-profiles"), {
        method: id ? "PATCH" : "POST",
        body,
      }),
    onSuccess: () => client.invalidateQueries({ queryKey: profilesKey(ws) }),
  });
}

export function useDeleteProfile(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => request<void>(wsPath(ws, `/research-profiles/${id}`), { method: "DELETE" }),
    onSuccess: () => client.invalidateQueries({ queryKey: profilesKey(ws) }),
  });
}

export function useStartRun(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (profileID: string) =>
      data<ResearchRun>(wsPath(ws, `/research-profiles/${profileID}/runs`), { method: "POST" }),
    onSuccess: () => client.invalidateQueries({ queryKey: runsKey(ws) }),
  });
}

export function useRuns(ws: string, profileID?: string, limit = 20) {
  return useQuery({
    queryKey: [...runsKey(ws), "list", profileID ?? "", limit],
    queryFn: ({ signal }) =>
      data<ResearchRun[]>(wsPath(ws, "/research-runs") + query({ profile: profileID, limit: String(limit) }), { signal }),
    refetchInterval: (q) => (q.state.data?.some((r) => isActive(r)) ? POLL_MS : false),
  });
}

export function useRun(ws: string, id: string) {
  return useQuery({
    queryKey: [...runsKey(ws), id],
    queryFn: ({ signal }) => data<ResearchRun>(wsPath(ws, `/research-runs/${id}`), { signal }),
    refetchInterval: (q) => (isActive(q.state.data) ? POLL_MS : false),
  });
}

export function useCancelRun(ws: string, id: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: () => data<ResearchRun>(wsPath(ws, `/research-runs/${id}/cancel`), { method: "POST" }),
    onSuccess: (run) => client.setQueryData([...runsKey(ws), id], run),
  });
}

const maxEvents = 2000;

// useRunEvents keeps a run's log, fetching only lines after the last one it
// has, and polls while the run is active. level "error" shows only errors.
export function useRunEvents(ws: string, runID: string, errorsOnly: boolean, active: boolean) {
  const key = `${runID}:${errorsOnly}`;
  const [log, setLog] = useState<{ key: string; events: RunEvent[] }>({ key, events: [] });
  const events = log.key === key ? log.events : [];
  const after = events.length ? events[events.length - 1].id : 0;
  // The worker writes its last lines as the run ends; read once more then.
  const [wasActive, setWasActive] = useState(active);
  const client = useQueryClient();
  if (wasActive !== active) {
    setWasActive(active);
    if (!active) void client.invalidateQueries({ queryKey: [...runsKey(ws), runID, "events"] });
  }
  useQuery({
    queryKey: [...runsKey(ws), runID, "events", errorsOnly, after],
    queryFn: async ({ signal }) => {
      const page = await data<RunEvent[]>(
        wsPath(ws, `/research-runs/${runID}/events`) +
          query({ after: String(after), level: errorsOnly ? "error" : undefined, limit: "500" }),
        { signal },
      );
      if (page.length) {
        setLog((prev) => {
          const base = prev.key === key ? prev.events : [];
          return { key, events: [...base, ...page].slice(-maxEvents) };
        });
      }
      return page.length;
    },
    refetchInterval: active ? POLL_MS : false,
  });
  return events;
}

// useSuggestProfile asks the AI for a profile draft from a description. It
// saves nothing; the form shows the draft for review.
export function useSuggestProfile(ws: string) {
  return useMutation({
    mutationFn: (description: string) =>
      data<ProfileInput>(wsPath(ws, "/research-profiles/suggest"), { method: "POST", body: { description } }),
  });
}
