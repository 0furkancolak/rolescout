import { createContext, useContext } from "react";
import type { Workspace } from "../../lib/api";

export interface WorkspaceState {
  workspace: Workspace;
  workspaces: Workspace[];
  select: (id: string) => void;
}

export const WorkspaceContext = createContext<WorkspaceState | null>(null);

export const WORKSPACE_STORAGE_KEY = "rolescout.workspace";

// useWorkspace returns the active workspace. It must be used below
// WorkspaceProvider, which only renders once a workspace is known.
export function useWorkspace(): WorkspaceState {
  const state = useContext(WorkspaceContext);
  if (!state) throw new Error("useWorkspace must be used inside WorkspaceProvider");
  return state;
}

// wsPath builds a workspace-scoped API path.
export function wsPath(workspaceID: string, path = ""): string {
  return `/workspaces/${workspaceID}${path}`;
}
