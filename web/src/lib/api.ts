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
  first_seen_at: string;
  last_seen_at: string;
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

export function query(params: Record<string, string | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value) search.set(key, value);
  }
  const s = search.toString();
  return s ? `?${s}` : "";
}

export function isApiError(err: unknown, code?: string): err is ApiError {
  return err instanceof ApiError && (code === undefined || err.code === code);
}

// Mirrors statusTransitions in api/internal/domain/rules.go so the UI only
// offers changes the server accepts. The server remains the authority.
export const statusTransitions: Record<ApplicationStatus, ApplicationStatus[]> = {
  planned: ["applied", "withdrawn"],
  applied: ["interviewing", "offer", "rejected", "withdrawn"],
  interviewing: ["offer", "rejected", "withdrawn"],
  offer: ["accepted", "rejected", "withdrawn"],
  accepted: ["withdrawn"],
  rejected: ["planned", "applied"],
  withdrawn: ["planned", "applied"],
};

export const applicationStatuses = Object.keys(statusTransitions) as ApplicationStatus[];
export const workArrangements = ["onsite", "hybrid", "remote", "unknown"];
export const employmentTypes = ["full_time", "part_time", "contract", "internship", "temporary", "unknown"];
export const seniorityLevels = ["intern", "junior", "mid", "senior", "lead", "unknown"];
