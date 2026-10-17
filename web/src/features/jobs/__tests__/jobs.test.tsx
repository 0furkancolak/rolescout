import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { makeJob, mockApi, renderApp, signedIn, WS } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

describe("JobSearchPage", () => {
  it("lists jobs and loads the next page with the cursor", async () => {
    const api = mockApi({
      ...signedIn(),
      "GET /jobs": (call) =>
        call.search.get("cursor") === "next-1"
          ? { body: { data: [makeJob(3)], next_cursor: null } }
          : { body: { data: [makeJob(1), makeJob(2)], next_cursor: "next-1" } },
    });
    renderApp("/jobs");
    expect(await screen.findByRole("link", { name: "Backend Engineer 1" })).toBeInTheDocument();
    expect(screen.getAllByText("Remote")[0]).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(await screen.findByRole("link", { name: "Backend Engineer 3" })).toBeInTheDocument();
    expect(screen.getAllByRole("listitem").filter((li) => li.closest("main"))).toHaveLength(3);
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
    expect(api.calls.filter((c) => c.path === "/jobs").map((c) => c.search.get("limit"))).toEqual(["20", "20"]);
  });

  it("sends filters as query parameters and keeps them in the URL", async () => {
    const api = mockApi({ ...signedIn(), "GET /jobs": { body: { data: [], next_cursor: null } } });
    const { router } = renderApp("/jobs");
    await userEvent.type(await screen.findByLabelText("Search"), "platform");
    await userEvent.click(screen.getByRole("button", { name: "Search" }));
    await userEvent.selectOptions(screen.getByLabelText("Seniority"), "senior");
    await userEvent.type(screen.getByLabelText("Technologies"), "Go, React");
    await userEvent.tab();

    const last = api.calls.filter((c) => c.path === "/jobs").at(-1)!;
    expect(Object.fromEntries(last.search)).toEqual({
      q: "platform",
      seniority: "senior",
      technology: "Go, React",
      limit: "20",
    });
    expect(router.state.location.search).toContain("seniority=senior");

    await userEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(router.state.location.search).toBe("");
  });

  it("shows an empty state", async () => {
    mockApi({ ...signedIn(), "GET /jobs": { body: { data: [], next_cursor: null } } });
    renderApp("/jobs?q=nothing");
    expect(await screen.findByText("No jobs match these filters.")).toBeInTheDocument();
  });

  it("shows an error with a working retry", async () => {
    let fail = true;
    mockApi({
      ...signedIn(),
      "GET /jobs": () =>
        fail ? { status: 503, body: { error: { code: "internal", message: "" } } } : { body: { data: [makeJob(1)], next_cursor: null } },
    });
    renderApp("/jobs");
    expect(await screen.findByText("Something went wrong.")).toBeInTheDocument();
    fail = false;
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("link", { name: "Backend Engineer 1" })).toBeInTheDocument();
  });
});

describe("JobDetailPage", () => {
  const job = makeJob(7, { description: "Line one\nLine two" });
  const detail = {
    ...job,
    listings: [
      {
        id: "l1",
        source_id: "s1",
        url: "https://jobs.example.com/7",
        source_posted_at: "2026-10-09T00:00:00Z",
        source_updated_at: null,
        last_ingested_at: "2026-10-16T09:00:00Z",
      },
    ],
  };

  it("saves the job and links to the original listing safely", async () => {
    let saved = false;
    const api = mockApi({
      ...signedIn(),
      [`GET /jobs/${job.id}`]: { body: { data: detail } },
      [`GET /workspaces/${WS}/saved-jobs`]: () => ({
        body: { data: saved ? [{ saved_at: "2026-10-17T00:00:00Z", saved_by: null, job }] : [] },
      }),
      [`GET /workspaces/${WS}/applications`]: { body: { data: [] } },
      [`PUT /workspaces/${WS}/saved-jobs/${job.id}`]: () => {
        saved = true;
        return { status: 204 };
      },
    });
    renderApp(`/jobs/${job.id}`);

    const link = await screen.findByRole("link", { name: /Open original listing on jobs.example.com/ });
    expect(link).toHaveAttribute("href", "https://jobs.example.com/7");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");

    await userEvent.click(screen.getByRole("button", { name: "Save job" }));
    expect(await screen.findByRole("button", { name: "Remove from saved" })).toHaveAttribute("aria-pressed", "true");
    expect(api.calls.some((c) => c.method === "PUT")).toBe(true);
  });

  it("starts tracking an application and opens it", async () => {
    const app = {
      id: "44444444-4444-4444-8444-444444444444",
      job_id: job.id,
      status: "planned",
      created_by: null,
      created_at: "2026-10-17T00:00:00Z",
      updated_at: "2026-10-17T00:00:00Z",
    };
    const base = `/workspaces/${WS}/applications/${app.id}`;
    const api = mockApi({
      ...signedIn(),
      [`GET /jobs/${job.id}`]: { body: { data: detail } },
      [`GET /workspaces/${WS}/saved-jobs`]: { body: { data: [] } },
      [`GET /workspaces/${WS}/applications`]: { body: { data: [] } },
      [`POST /workspaces/${WS}/applications`]: { status: 201, body: { data: app } },
      [`GET ${base}`]: { body: { data: app } },
      [`GET ${base}/events`]: { body: { data: [] } },
      [`GET ${base}/attempts`]: { body: { data: [] } },
      [`GET ${base}/notes`]: { body: { data: [] } },
      [`GET ${base}/tags`]: { body: { data: [] } },
      [`GET /workspaces/${WS}/tags`]: { body: { data: [] } },
    });
    const { router } = renderApp(`/jobs/${job.id}`);
    await userEvent.click(await screen.findByRole("button", { name: "Track application" }));
    expect(await screen.findByRole("heading", { name: "History" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(`/applications/${app.id}`);
    expect(api.calls.find((c) => c.method === "POST")?.body).toEqual({ job_id: job.id });
  });

  it("shows not found for jobs that do not exist", async () => {
    mockApi({
      ...signedIn(),
      [`GET /jobs/${job.id}`]: { status: 404, body: { error: { code: "not_found", message: "" } } },
      [`GET /workspaces/${WS}/saved-jobs`]: { body: { data: [] } },
      [`GET /workspaces/${WS}/applications`]: { body: { data: [] } },
    });
    renderApp(`/jobs/${job.id}`);
    expect(await screen.findByRole("heading", { name: "Not found" })).toBeInTheDocument();
  });
});

describe("SavedJobsPage", () => {
  it("lists saved jobs and removes one", async () => {
    let saved = [{ saved_at: "2026-10-17T00:00:00Z", saved_by: null, job: makeJob(1) }];
    mockApi({
      ...signedIn(),
      [`GET /workspaces/${WS}/saved-jobs`]: () => ({ body: { data: saved } }),
      [`DELETE /workspaces/${WS}/saved-jobs/${makeJob(1).id}`]: () => {
        saved = [];
        return { status: 204 };
      },
    });
    renderApp("/saved");
    const card = (await screen.findByRole("link", { name: "Backend Engineer 1" })).closest("article")!;
    await userEvent.click(within(card).getByRole("button", { name: "Remove from saved" }));
    expect(await screen.findByText("You have not saved any jobs yet.")).toBeInTheDocument();
  });
});
