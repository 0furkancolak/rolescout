import type { ResearchProfile } from "../../lib/api";
import type { ProfileInput } from "./api";

export function emptyProfile(): ProfileInput {
  return {
    name: "", title_keywords: [], keywords: [], technologies: [], countries: [], locations: [],
    excluded_keywords: [], remote: "any", seniority: [], schedule_hours: 12, schedule_enabled: true,
  };
}

export function toInput(p: ResearchProfile): ProfileInput {
  const { id: _id, last_run_at: _l, created_at: _c, updated_at: _u, ...rest } = p;
  return rest;
}
