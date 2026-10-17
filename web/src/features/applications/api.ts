import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  data,
  query,
  request,
  type Application,
  type ApplicationEvent,
  type ApplicationStatus,
  type Attempt,
  type Note,
  type Tag,
} from "../../lib/api";
import { wsPath } from "../workspaces/context";

const appsKey = (ws: string) => ["workspaces", ws, "applications"] as const;
const appKey = (ws: string, id: string) => [...appsKey(ws), id] as const;
const tagsKey = (ws: string) => ["workspaces", ws, "tags"] as const;

export function useApplications(ws: string, status?: ApplicationStatus) {
  return useQuery({
    queryKey: [...appsKey(ws), "list", status ?? "all"],
    queryFn: ({ signal }) => data<Application[]>(wsPath(ws, "/applications" + query({ status })), { signal }),
  });
}

export function useApplication(ws: string, id: string) {
  return useQuery({
    queryKey: [...appKey(ws, id), "detail"],
    queryFn: ({ signal }) => data<Application>(wsPath(ws, `/applications/${id}`), { signal }),
  });
}

function useSub<T>(ws: string, id: string, sub: string) {
  return useQuery({
    queryKey: [...appKey(ws, id), sub],
    queryFn: ({ signal }) => data<T>(wsPath(ws, `/applications/${id}/${sub}`), { signal }),
  });
}

export const useAttempts = (ws: string, id: string) => useSub<Attempt[]>(ws, id, "attempts");
export const useEvents = (ws: string, id: string) => useSub<ApplicationEvent[]>(ws, id, "events");
export const useNotes = (ws: string, id: string) => useSub<Note[]>(ws, id, "notes");
export const useApplicationTags = (ws: string, id: string) => useSub<Tag[]>(ws, id, "tags");

export function useTags(ws: string) {
  return useQuery({
    queryKey: tagsKey(ws),
    queryFn: ({ signal }) => data<Tag[]>(wsPath(ws, "/tags"), { signal }),
  });
}

// Every mutation on an application refreshes all of its queries and the list,
// because status, history and updated_at can change together.
function useAppMutation<V, R>(ws: string, fn: (v: V) => Promise<R>) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => client.invalidateQueries({ queryKey: appsKey(ws) }),
  });
}

export function useCreateApplication(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (jobID: string) =>
      data<Application>(wsPath(ws, "/applications"), { method: "POST", body: { job_id: jobID } }),
    onSuccess: () => client.invalidateQueries({ queryKey: appsKey(ws) }),
  });
}

export function useChangeStatus(ws: string, id: string) {
  return useAppMutation(ws, (status: ApplicationStatus) =>
    data<Application>(wsPath(ws, `/applications/${id}`), { method: "PATCH", body: { status } }),
  );
}

export function useAddAttempt(ws: string, id: string) {
  return useAppMutation(ws, (body: { applied_at: string; channel: string }) =>
    data<Attempt>(wsPath(ws, `/applications/${id}/attempts`), { method: "POST", body }),
  );
}

export function useAddEvent(ws: string, id: string) {
  return useAppMutation(ws, (body: { type: "interview" | "follow_up" | "deadline"; occurred_at: string }) =>
    data<ApplicationEvent>(wsPath(ws, `/applications/${id}/events`), { method: "POST", body }),
  );
}

export function useAddNote(ws: string, id: string) {
  return useAppMutation(ws, (body: string) =>
    data<Note>(wsPath(ws, `/applications/${id}/notes`), { method: "POST", body: { body } }),
  );
}

export function useUpdateNote(ws: string, id: string) {
  return useAppMutation(ws, ({ noteID, body }: { noteID: string; body: string }) =>
    data<Note>(wsPath(ws, `/applications/${id}/notes/${noteID}`), { method: "PATCH", body: { body } }),
  );
}

export function useSetTag(ws: string, id: string) {
  return useAppMutation(ws, ({ tagID, on }: { tagID: string; on: boolean }) =>
    request<void>(wsPath(ws, `/applications/${id}/tags/${tagID}`), { method: on ? "PUT" : "DELETE" }),
  );
}

export function useCreateTag(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => data<Tag>(wsPath(ws, "/tags"), { method: "POST", body: { name } }),
    onSuccess: () => client.invalidateQueries({ queryKey: tagsKey(ws) }),
  });
}
