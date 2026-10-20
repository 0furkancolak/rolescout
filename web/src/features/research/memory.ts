import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { data, request, type WorkspaceContext } from "../../lib/api";
import { wsPath } from "../workspaces/context";

const contextKey = (ws: string) => ["workspaces", ws, "context"] as const;

// useMemory reads what the AI knows about the person: a note and a CV.
export function useMemory(ws: string) {
  return useQuery({
    queryKey: contextKey(ws),
    queryFn: ({ signal }) => data<WorkspaceContext>(wsPath(ws, "/context"), { signal }),
  });
}

// A new note or CV changes how jobs are compared and explained, so job lists
// are read again.
function useRefresh(ws: string) {
  const client = useQueryClient();
  return (ctx?: WorkspaceContext) => {
    if (ctx) client.setQueryData(contextKey(ws), ctx);
    void client.invalidateQueries({ queryKey: ["workspaces", ws] });
  };
}

export function useSaveAbout(ws: string) {
  const refresh = useRefresh(ws);
  return useMutation({
    mutationFn: (about: string) => data<WorkspaceContext>(wsPath(ws, "/context"), { method: "PUT", body: { about } }),
    onSuccess: refresh,
  });
}

export function useUploadCV(ws: string) {
  const refresh = useRefresh(ws);
  return useMutation({
    mutationFn: (file: File) => {
      const form = new FormData();
      form.append("file", file);
      return data<WorkspaceContext>(wsPath(ws, "/context/cv"), { method: "POST", body: form });
    },
    onSuccess: refresh,
  });
}

export function useDeleteCV(ws: string) {
  const refresh = useRefresh(ws);
  return useMutation({
    mutationFn: () => request<void>(wsPath(ws, "/context/cv"), { method: "DELETE" }),
    onSuccess: () => refresh(),
  });
}
