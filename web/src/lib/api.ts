// Thin client for the RoleScout REST API (docs/api/openapi.yaml). Requests go
// to the same origin; the dev server and nginx proxy /api to the Go API.

export type ApplicationStatus =
  | "planned"
  | "applied"
  | "interviewing"
  | "offer"
  | "accepted"
  | "rejected"
  | "withdrawn";

export type EventType =
  | "created"
  | "status_changed"
  | "attempt_added"
  | "interview"
  | "follow_up"
  | "deadline";

export type Locale = "tr" | "en";

export interface User {
  id: string;
  email: string;
  display_name: string;
  locale: Locale;
  is_admin?: boolean;
  created_at: string;
}

export interface Workspace {
  id: string;
  name: string;
  role: "owner" | "member";
  created_at: string;
}

export interface Member {
  user_id: string;
  email: string;
  display_name: string;
  role: "owner" | "member";
  joined_at: string;
}

export interface Job {
  id: string;
  title: string;
  company_name: string;
  location: string | null;
  work_arrangement: string;
  employment_type: string;
  seniority: string;
  technologies: string[];
  description: string;
  company_id?: string | null;
  country_code?: string | null;
  closed_at?: string | null;
  first_seen_at: string;
  last_seen_at: string;
}

export interface MatchReason {
  code: "title" | "keyword" | "role" | "tech" | "remote" | "country" | "location" | "hybrid" | "seniority" | "fresh";
  params?: Record<string, string>;
}

export interface Match {
  score: number;
  new: boolean;
  reasons: MatchReason[];
  // A short reason written by the local model, when there is one.
  explanation?: string | null;
}

// JobAI is what the local model added to a job; null or absent when it has
// not read the job.
export interface JobAI {
  model: string;
  summary: string | null;
  remote_policy: string | null;
  salary: { min: number | null; max: number | null; currency: string; period: "year" | "month" | "day" | "hour" } | null;
  // Effective values the model filled because the rules did not know them.
  fields: Array<"seniority" | "work_arrangement" | "country">;
  extra_technologies: string[];
}

export interface LlmConfig {
  enabled: boolean;
  base_url: string;
  model: string;
  has_api_key: boolean;
  api_key_hint?: string;
  background: boolean;
  background_auto: boolean;
  local: boolean;
  stored: Record<string, boolean>;
}

export interface LlmConfigUpdate {
  enabled?: boolean;
  base_url?: string;
  model?: string;
  api_key?: string;
  background?: boolean;
  reset_background?: boolean;
}

export interface Analysis {
  explanation: string;
  ai: JobAI | null;
}

export interface LlmStatus {
  enabled: boolean;
  base_host?: string;
  model?: string;
  fallback_model?: string;
  reachable: boolean;
  model_present: boolean;
  degraded: boolean;
  local: boolean;
  queue: number;
  enriched: number;
  failed: number;
  avg_latency_ms: number;
  last_error?: string;
  last_checked_at: string | null;
  probe_ok: boolean;
  probe_ms: number;
}

// WorkspaceJob is a job as the current workspace sees it.
export interface WorkspaceJob extends Job {
  saved: boolean;
  match: Match | null;
  ai?: JobAI | null;
}

export interface FacetCount {
  value: string;
  label: string;
  count: number;
}

export interface Facets {
  posted: Record<string, number>;
  arrangement: Record<string, number>;
  seniority: Record<string, number>;
  country: Record<string, number>;
  technologies: FacetCount[];
  companies: FacetCount[];
}

export type RemotePreference = "any" | "remote_only" | "remote_or_hybrid" | "onsite_ok";

export interface ResearchProfile {
  id: string;
  name: string;
  title_keywords: string[];
  keywords: string[];
  technologies: string[];
  countries: string[];
  locations: string[];
  excluded_keywords: string[];
  remote: RemotePreference;
  seniority: string[];
  schedule_hours: number;
  last_run_at: string | null;
  created_at: string;
  updated_at: string;
}

export type RunState = "queued" | "running" | "succeeded" | "failed" | "cancelled";
export type Stage = "discovery" | "detection" | "collection" | "scoring" | "result";
export const stages: Stage[] = ["discovery", "detection", "collection", "scoring", "result"];

export interface StageProgress {
  state: "pending" | "running" | "done" | "skipped" | "failed";
  total: number;
  done: number;
  failed: number;
  detail: Record<string, number>;
  note: string;
  started_at: string | null;
  finished_at: string | null;
}

export interface ResearchRun {
  id: string;
  profile_id: string;
  state: RunState;
  stage: Stage;
  stages: Record<Stage, StageProgress>;
  trigger: "manual" | "schedule";
  error: string | null;
  cancel_requested: boolean;
  summary: Record<string, number>;
  queued_at: string;
  started_at: string | null;
  finished_at: string | null;
}

export interface RunEvent {
  id: number;
  at: string;
  level: "info" | "warn" | "error";
  stage: Stage;
  company: string;
  message: string;
}

export type CompanyStatus = "pending" | "active" | "empty" | "unreachable" | "blocked_by_robots" | "disabled";

export interface Company {
  id: string;
  name: string;
  careers_url: string;
  ats: string;
  status: CompanyStatus;
  origin: string;
  open_jobs: number;
  last_checked_at: string | null;
  last_error: string | null;
  blocked: boolean;
}

export interface AdapterReview {
  adapter: string;
  enabled: boolean;
  reviewed_at: string | null;
  note: string | null;
}

export interface Setting {
  name: string;
  value: string;
}

export interface Listing {
  id: string;
  source_id: string;
  url: string;
  source_posted_at: string | null;
  source_updated_at: string | null;
  last_ingested_at: string;
}

export interface JobDetail extends Job {
  listings: Listing[];
  ai?: JobAI | null;
}

export interface SavedJob {
  saved_at: string;
  saved_by: string | null;
  job: Job;
}

export interface Application {
  id: string;
  job_id: string;
  status: ApplicationStatus;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface Attempt {
  id: string;
  attempt_number: number;
  applied_at: string;
  channel: string;
  created_by: string | null;
  created_at: string;
}

export interface ApplicationEvent {
  id: string;
  type: EventType;
  attempt_id: string | null;
  from_status: ApplicationStatus | null;
  to_status: ApplicationStatus | null;
  occurred_at: string;
  details: Record<string, unknown>;
  created_by: string | null;
  created_at: string;
}

export interface Tag {
  id: string;
  name: string;
  created_at: string;
}

export interface Note {
  id: string;
  author_id: string | null;
  body: string;
  created_at: string;
  updated_at: string;
}

export interface Page<T> {
  data: T[];
  next_cursor: string | null;
}

export interface FieldError {
  field: string;
  message: string;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: FieldError[];

  constructor(status: number, code: string, message: string, fields: FieldError[] = []) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

export const API_BASE = "/api/v1";

interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  signal?: AbortSignal;
}

// request returns the parsed JSON body, or undefined for 204 responses. Any
// non-2xx response becomes an ApiError carrying the API's error code.
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (options.body !== undefined) headers["Content-Type"] = "application/json";

  let response: Response;
  try {
    response = await fetch(API_BASE + path, {
      method: options.method ?? "GET",
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      credentials: "same-origin",
      signal: options.signal,
    });
  } catch (err) {
    if (err instanceof DOMException && err.name === "AbortError") throw err;
    throw new ApiError(0, "network", "The server could not be reached.");
  }

  if (response.status === 204) return undefined as T;

  const body: unknown = await response.json().catch(() => undefined);
  if (!response.ok) {
    const error = (body as { error?: { code?: string; message?: string; fields?: FieldError[] } } | undefined)
      ?.error;
    throw new ApiError(
      response.status,
      error?.code ?? "internal",
      error?.message ?? response.statusText,
      error?.fields ?? [],
    );
  }
  return body as T;
}

// data unwraps the {"data": ...} envelope used by single resources and lists.
export async function data<T>(path: string, options?: RequestOptions): Promise<T> {
  const body = await request<{ data: T }>(path, options);
  return body.data;
}

export function query(params: Record<string, string | string[] | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (Array.isArray(value)) {
      for (const v of value) if (v) search.append(key, v);
    } else if (value) {
      search.set(key, value);
    }
  }
  const s = search.toString();
  return s ? `?${s}` : "";
}

export function isApiError(err: unknown, code?: string): err is ApiError {
  return err instanceof ApiError && (code === undefined || err.code === code);
}

// Mirrors statusTransitions in api/internal/domain/rules.go: the tracker is
// the user's own record, so any status may be corrected to any other.
const allStatuses: ApplicationStatus[] = ["planned", "applied", "interviewing", "offer", "accepted", "rejected", "withdrawn"];
export const statusTransitions = Object.fromEntries(
  allStatuses.map((from) => [from, allStatuses.filter((to) => to !== from)]),
) as Record<ApplicationStatus, ApplicationStatus[]>;

export const applicationStatuses = Object.keys(statusTransitions) as ApplicationStatus[];
export const workArrangements = ["onsite", "hybrid", "remote", "unknown"];
export const employmentTypes = ["full_time", "part_time", "contract", "internship", "temporary", "unknown"];
export const seniorityLevels = ["intern", "junior", "mid", "senior", "lead", "unknown"];

export interface SystemService {
  name: "api" | "worker" | "postgres" | "web";
  used_bytes: number | null;
  limit_bytes: number | null;
  heap_bytes?: number;
  estimated: boolean;
  stale: boolean;
  updated_at?: string;
  note?: "ceiling" | "container";
}

export interface SystemReport {
  host_total_bytes: number;
  services: SystemService[];
  postgres: { shared_buffers_bytes: number; work_mem_bytes: number; max_connections: number; connections: number; database_bytes: number };
}
