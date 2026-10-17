import { render } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { vi } from "vitest";
import { Providers } from "../app/Providers";
import { createQueryClient } from "../app/queryClient";
import { routes } from "../app/routes";
import type { Job, Locale, User, Workspace } from "../lib/api";

export interface Call {
  method: string;
  path: string;
  search: URLSearchParams;
  body: unknown;
}

export interface Reply {
  status?: number;
  body?: unknown;
}

type Handler = Reply | ((call: Call) => Reply);

// mockApi replaces fetch with an in-memory API. Handlers are keyed by
// "METHOD /path" (without /api/v1 and the query string). Unmatched requests
// fail the test with a 500 so missing stubs are obvious.
export function mockApi(handlers: Record<string, Handler>) {
  const calls: Call[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), "http://localhost");
    const call: Call = {
      method: init?.method ?? "GET",
      path: url.pathname.replace(/^\/api\/v1/, ""),
      search: url.searchParams,
      body: init?.body ? JSON.parse(String(init.body)) : undefined,
    };
    calls.push(call);
    const handler = handlers[`${call.method} ${call.path}`];
    if (!handler) {
      return new Response(JSON.stringify({ error: { code: "internal", message: `unmocked ${call.method} ${call.path}` } }), {
        status: 500,
      });
    }
    const reply = typeof handler === "function" ? handler(call) : handler;
    const status = reply.status ?? 200;
    return new Response(status === 204 ? null : JSON.stringify(reply.body ?? {}), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  });
  vi.stubGlobal("fetch", fetchMock);
  return { calls, set: (key: string, h: Handler) => (handlers[key] = h) };
}

export function renderApp(path: string, { locale = "en" as Locale } = {}) {
  const client = createQueryClient();
  client.setDefaultOptions({ queries: { ...client.getDefaultOptions().queries, retry: false } });
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const result = render(
    <Providers client={client} locale={locale}>
      <RouterProvider router={router} />
    </Providers>,
  );
  return { ...result, router, client };
}

export const WS = "11111111-1111-4111-8111-111111111111";

export const user: User = {
  id: "22222222-2222-4222-8222-222222222222",
  email: "ada@example.com",
  display_name: "Ada",
  locale: "en",
  created_at: "2026-10-01T10:00:00Z",
};

export const workspace: Workspace = { id: WS, name: "Personal", role: "owner", created_at: "2026-10-01T10:00:00Z" };

export function makeJob(n: number, overrides: Partial<Job> = {}): Job {
  return {
    id: `33333333-3333-4333-8333-${String(n).padStart(12, "0")}`,
    title: `Backend Engineer ${n}`,
    company_name: `Company ${n}`,
    location: "Istanbul",
    work_arrangement: "remote",
    employment_type: "full_time",
    seniority: "senior",
    technologies: ["go", "postgresql"],
    description: "Build things.",
    first_seen_at: "2026-10-10T09:00:00Z",
    last_seen_at: "2026-10-16T09:00:00Z",
    ...overrides,
  };
}

// signedIn returns handlers for an authenticated user with one workspace.
export function signedIn(): Record<string, Handler> {
  return {
    "GET /auth/me": { body: { data: user } },
    "GET /workspaces": { body: { data: [workspace] } },
  };
}
