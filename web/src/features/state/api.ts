import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { data, isApiError, request } from "../../lib/api";
import { wsPath } from "../workspaces/context";

// Small documents the server keeps per user and workspace: where the user
// left off in the job browser, and the details used to fill application forms.
export type StateKey = "jobs_view" | "autofill";

const stateKey = (ws: string, key: StateKey) => ["workspaces", ws, "state", key] as const;

// useUserState returns the stored document, or null when none was saved yet.
export function useUserState<T extends object>(ws: string, key: StateKey, enabled = true) {
  return useQuery({
    queryKey: stateKey(ws, key),
    queryFn: async ({ signal }) => {
      try {
        return await data<T>(wsPath(ws, `/state/${key}`), { signal });
      } catch (err) {
        if (isApiError(err, "not_found")) return null;
        throw err;
      }
    },
    enabled,
    staleTime: Infinity,
  });
}

export function usePutUserState<T extends object>(ws: string, key: StateKey) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (value: T) => request<void>(wsPath(ws, `/state/${key}`), { method: "PUT", body: value }),
    onSuccess: (_r, value) => client.setQueryData(stateKey(ws, key), value),
  });
}
