import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { data, query, request, type Company, type Page } from "../../lib/api";
import { wsPath } from "../workspaces/context";

export interface CompanyFilters {
  q: string;
  ats: string;
  status: string;
}

const companiesKey = (ws: string) => ["workspaces", ws, "companies"] as const;

export function useCompanies(ws: string, f: CompanyFilters) {
  return useInfiniteQuery({
    queryKey: [...companiesKey(ws), f],
    queryFn: ({ pageParam, signal }) =>
      request<Page<Company>>(
        wsPath(ws, "/companies") +
          query({ q: f.q || undefined, ats: f.ats || undefined, status: f.status || undefined, limit: "50", cursor: pageParam ?? undefined }),
        { signal },
      ),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.next_cursor,
  });
}

export function useAddCompany(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (body: { careers_url: string; name?: string }) => data<Company>("/companies", { method: "POST", body }),
    onSuccess: () => client.invalidateQueries({ queryKey: companiesKey(ws) }),
  });
}

export function useBlockCompany(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ id, blocked }: { id: string; blocked: boolean }) =>
      request<void>(wsPath(ws, `/company-blocks/${id}`), { method: blocked ? "PUT" : "DELETE" }),
    onSettled: () => {
      void client.invalidateQueries({ queryKey: companiesKey(ws) });
      void client.invalidateQueries({ queryKey: ["workspaces", ws, "jobs"] });
    },
  });
}

export function useDisableCompany(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ id, disabled }: { id: string; disabled: boolean }) =>
      data<Company>(`/admin/companies/${id}`, { method: "PATCH", body: { disabled } }),
    onSettled: () => client.invalidateQueries({ queryKey: companiesKey(ws) }),
  });
}
