// The job browser keeps its whole state in the URL so views can be shared
// and survive reloads.

export type Sort = "" | "score" | "recent" | "relevance";
export type Posted = "" | "24h" | "7d" | "30d";

export interface JobSearch {
  q: string;
  // profile is a profile ID, "all" for every job, or "" for the default
  // (the first profile).
  profile: string;
  sort: Sort;
  posted: Posted;
  arrangement: string;
  seniority: string;
  technologies: string[];
  countries: string[];
  // location is free text matched against job locations ("izmir"); region
  // is a named group of countries ("emea"); ats is a job board system.
  location: string;
  region: string;
  ats: string;
  companies: string[];
  closed: boolean;
  job: string;
}

const sorts: Sort[] = ["score", "recent", "relevance"];
const posteds: Posted[] = ["24h", "7d", "30d"];
const arrangements = ["onsite", "hybrid", "remote"];
export const regions = ["europe", "eu", "emea", "mena", "dach", "nordics", "benelux", "north america", "latam", "apac"] as const;
export const atsList = ["lever", "greenhouse", "ashby", "workable", "recruitee", "personio", "bamboohr", "workday", "teamtailor", "breezy", "rippling", "gem", "htmlpage", "jsonld"] as const;
const seniorities = ["intern", "junior", "mid", "senior", "lead"];

const pick = <T extends string>(v: string | null, allowed: readonly T[]): T | "" =>
  v && (allowed as readonly string[]).includes(v) ? (v as T) : "";

export function readJobSearch(p: URLSearchParams): JobSearch {
  const list = (key: string) => p.getAll(key).map((v) => v.trim()).filter(Boolean);
  return {
    q: (p.get("q") ?? "").trim(),
    profile: (p.get("profile") ?? "").trim(),
    sort: pick(p.get("sort"), sorts),
    posted: pick(p.get("posted"), posteds),
    arrangement: pick(p.get("arrangement"), arrangements),
    seniority: pick(p.get("seniority"), seniorities),
    technologies: list("technology"),
    countries: list("country"),
    location: (p.get("location") ?? "").trim().slice(0, 80),
    region: pick(p.get("region"), regions),
    ats: pick(p.get("ats"), atsList),
    companies: list("company"),
    closed: p.get("closed") === "true",
    job: (p.get("job") ?? "").trim(),
  };
}

export function writeJobSearch(s: JobSearch): URLSearchParams {
  const p = new URLSearchParams();
  const set = (k: string, v: string) => v && p.set(k, v);
  set("q", s.q);
  set("profile", s.profile);
  set("sort", s.sort);
  set("posted", s.posted);
  set("arrangement", s.arrangement);
  set("seniority", s.seniority);
  for (const t of s.technologies) p.append("technology", t);
  for (const c of s.countries) p.append("country", c);
  set("location", s.location);
  set("region", s.region);
  set("ats", s.ats);
  for (const c of s.companies) p.append("company", c);
  if (s.closed) p.set("closed", "true");
  set("job", s.job);
  return p;
}

// effectiveSearch resolves defaults: the first profile and, with a profile,
// best-match order; relevance needs a query.
export function effectiveSearch(s: JobSearch, firstProfile?: string): JobSearch {
  const profile = s.profile === "all" ? "" : s.profile || firstProfile || "";
  let sort: Sort = s.sort || (profile ? "score" : "recent");
  if (sort === "score" && !profile) sort = "recent";
  if (sort === "relevance" && !s.q) sort = profile ? "score" : "recent";
  return { ...s, profile, sort };
}
