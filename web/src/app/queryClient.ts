import { QueryClient } from "@tanstack/react-query";
import { isApiError } from "../lib/api";

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        // Client errors will not succeed on retry; network and 5xx errors might.
        retry: (count, err) => count < 2 && !(isApiError(err) && err.status >= 400 && err.status < 500),
      },
    },
  });
}
