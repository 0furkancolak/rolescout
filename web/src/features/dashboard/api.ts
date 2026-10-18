import { useQuery } from "@tanstack/react-query";
import { data } from "../../lib/api";
import { wsPath } from "../workspaces/context";

export interface Activity {
  days: string[];
  new_matches: number[];
  jobs_collected: number[];
  applications_created: number[];
  upcoming_interviews: number;
}

export function useActivity(workspaceID: string, days = 14) {
  return useQuery({
    queryKey: ["workspaces", workspaceID, "activity", days],
    queryFn: ({ signal }) => data<Activity>(wsPath(workspaceID, `/activity?days=${days}`), { signal }),
    staleTime: 60_000,
  });
}
