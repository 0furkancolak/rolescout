import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { ErrorState, Spinner } from "../../components/ui";
import { useCurrentUser } from "./api";

export function RequireAuth({ children }: { children: ReactNode }) {
  const me = useCurrentUser();
  const location = useLocation();
  if (me.isPending) return <Spinner />;
  if (me.isError)
    return (
      <main className="mx-auto max-w-md p-6">
        <ErrorState error={me.error} onRetry={() => void me.refetch()} />
      </main>
    );
  if (!me.data) return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />;
  return children;
}
