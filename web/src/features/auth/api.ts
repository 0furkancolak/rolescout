import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { data, isApiError, request, type Locale, type User } from "../../lib/api";

export const meKey = ["auth", "me"] as const;

// useCurrentUser resolves to null when there is no valid session.
export function useCurrentUser() {
  return useQuery({
    queryKey: meKey,
    queryFn: async ({ signal }) => {
      try {
        return await data<User>("/auth/me", { signal });
      } catch (err) {
        if (isApiError(err, "unauthenticated")) return null;
        throw err;
      }
    },
    staleTime: 5 * 60_000,
    retry: false,
  });
}

export interface LoginInput {
  email: string;
  password: string;
}

export interface RegisterInput extends LoginInput {
  display_name: string;
  locale: Locale;
}

export function useLogin() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: LoginInput) => data<User>("/auth/login", { method: "POST", body: input }),
    onSuccess: (user) => {
      client.clear();
      client.setQueryData(meKey, user);
    },
  });
}

export function useRegister() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: RegisterInput) => data<User>("/auth/register", { method: "POST", body: input }),
    onSuccess: (user) => {
      client.clear();
      client.setQueryData(meKey, user);
    },
  });
}

export function useLogout() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: () => request<void>("/auth/logout", { method: "POST" }),
    // Forget cached workspace data even if the request failed.
    onSettled: () => {
      client.clear();
      client.setQueryData(meKey, null);
    },
  });
}
