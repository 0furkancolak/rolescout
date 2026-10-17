import { useQuery } from "@tanstack/react-query";
import { useMemo, useState, type ReactNode } from "react";
import { ErrorState, Spinner } from "../../components/ui";
import { data, type Workspace } from "../../lib/api";
import { WORKSPACE_STORAGE_KEY, WorkspaceContext } from "./context";

function storedWorkspace(): string | null {
  try {
    return localStorage.getItem(WORKSPACE_STORAGE_KEY);
  } catch {
    return null;
  }
}

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const workspaces = useQuery({
    queryKey: ["workspaces"],
    queryFn: ({ signal }) => data<Workspace[]>("/workspaces", { signal }),
  });
  const [selected, setSelected] = useState<string | null>(storedWorkspace);

  const value = useMemo(() => {
    const list = workspaces.data ?? [];
    const workspace = list.find((w) => w.id === selected) ?? list[0];
    if (!workspace) return null;
    return {
      workspace,
      workspaces: list,
      select: (id: string) => {
        setSelected(id);
        try {
          localStorage.setItem(WORKSPACE_STORAGE_KEY, id);
        } catch {
          // Selection still applies for this visit.
        }
      },
    };
  }, [workspaces.data, selected]);

  if (workspaces.isPending) return <Spinner />;
  if (workspaces.isError || !value)
    return (
      <main className="mx-auto max-w-md p-6">
        <ErrorState error={workspaces.error} onRetry={() => void workspaces.refetch()} />
      </main>
    );
  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>;
}
