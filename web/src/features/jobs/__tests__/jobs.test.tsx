import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { makeJob, makeProfile, makeWorkspaceJob, mockApi, PROFILE, renderApp, signedIn, WS } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

const base = `/workspaces/${WS}`;
const facets = {
  posted: { "24h": 1, "7d": 2, "30d": 2 },
  arrangement: { remote: 2, onsite: 1 },
  seniority: { senior: 2 },
  country: { TR: 1 },
  technologies: [{ value: "react", label: "react", count: 2 }],
  companies: [{ value: "c1", label: "Acme", count: 1 }],
};
const matched = (n: number, extra = {}) =>
  makeWorkspaceJob(n, {
    title: `Full Stack Engineer ${n}`,
    match: { score: 90 - n, new: n === 1, reasons: [{ code: "tech", params: { matched: "3", total: "5" } }, { code: "remote" }] },
    ...extra,
  });

function browse(handlers = {}) {
  return mockApi({
    ...signedIn(),
    [`GET ${base}/research-profiles`]: { body: { data: [makeProfile()] } },
    [`GET ${base}/jobs/facets`]: { body: { data: facets } },
    [`GET ${base}/jobs`]: (call: { search: URLSearchParams }) =>
      call.search.get("cursor") === "next"
        ? { body: { data: [matched(3)], next_cursor: null } }
        : { body: { data: [matched(1), matched(2)], next_cursor: "next" } },
    [`GET /jobs/${makeJob(1).id}`]: {
      body: { data: { ...makeJob(1), listings: [{ id: "l1", source_id: "s1", url: "https://boards.greenhouse.io/acme/jobs/1", source_posted_at: null, source_updated_at: null, last_ingested_at: "2026-10-16T09:00:00Z" }] } },
    },
    [`GET /jobs/${makeJob(2).id}`]: { body: { data: { ...makeJob(2), listings: [] } } },
    ...handlers,
  });
}

describe("JobsPage", () => {
  it("shows the first profile's matches and loads more", async () => {
    const api = browse();
    renderApp("/jobs");
    expect(await screen.findByRole("button", { name: /Full Stack Engineer 1/ })).toBeInTheDocument();
    const first = api.calls.find((c) => c.path === `${base}/jobs`)!;
    expect(first.search.get("profile")).toBe(PROFILE);
    expect(first.search.get("sort")).toBe("score");
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(await screen.findByRole("button", { name: /Full Stack Engineer 3/ })).toBeInTheDocument();
  });

  it("rates all jobs against the first profile so every row shows a score", async () => {
    const api = browse();
    renderApp("/jobs?profile=all");
    expect(await screen.findByRole("button", { name: /Full Stack Engineer 1/ })).toBeInTheDocument();
    const first = api.calls.find((c) => c.path === `${base}/jobs`)!;
    expect(first.search.get("profile")).toBeNull();
    expect(first.search.get("score_profile")).toBe(PROFILE);
    expect(screen.getAllByRole("img", { name: /match|eşleşme|score|skor/i }).length).toBeGreaterThan(0);
  });

  it("filters with facets and keeps them in the URL", async () => {
    const api = browse();
    const { router } = renderApp("/jobs");
    await userEvent.click(await screen.findByRole("button", { name: "Filters" }));
    await userEvent.click(await screen.findByRole("checkbox", { name: /react/ }));
    await userEvent.keyboard("{Escape}");
    await vi.waitFor(() => expect(router.state.location.search).toContain("technology=react"));
    await userEvent.click(screen.getByRole("button", { name: /^Remote/ }));
    await vi.waitFor(() => expect(router.state.location.search).toContain("arrangement=remote"));
    const last = api.calls.filter((c) => c.path === `${base}/jobs`).at(-1)!;
    expect(last.search.getAll("technology")).toEqual(["react"]);
    expect(last.search.get("arrangement")).toBe("remote");
    await userEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    await vi.waitFor(() => expect(router.state.location.search).not.toContain("technology"));
  });

  it("filters with chips that toggle and read the same URL params", async () => {
    const api = browse();
    const { router } = renderApp("/jobs");
    const week = await screen.findByRole("button", { name: /Last 7 days/ });
    expect(week).toHaveAttribute("aria-pressed", "false");
    await userEvent.click(week);
    await vi.waitFor(() => expect(router.state.location.search).toContain("posted=7d"));
    expect(screen.getByRole("button", { name: /Last 7 days/ })).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(screen.getByRole("button", { name: /^Senior/ }));
    await vi.waitFor(() => expect(router.state.location.search).toContain("seniority=senior"));
    const last = api.calls.filter((c) => c.path === `${base}/jobs`).at(-1)!;
    expect(last.search.get("posted")).toBe("7d");
    expect(last.search.get("seniority")).toBe("senior");
    // Pressing a pressed chip clears it.
    await userEvent.click(screen.getByRole("button", { name: /Last 7 days/ }));
    await vi.waitFor(() => expect(router.state.location.search).not.toContain("posted"));
  });

  it("sorts with a segmented control that works from the keyboard", async () => {
    const api = browse();
    const { router } = renderApp("/jobs");
    const best = await screen.findByRole("radio", { name: "Best match" });
    expect(best).toBeChecked();
    best.focus();
    await userEvent.keyboard("{ArrowRight}");
    await vi.waitFor(() => expect(router.state.location.search).toContain("sort=recent"));
    expect(screen.getByRole("radio", { name: "Newest" })).toBeChecked();
    expect(api.calls.filter((c) => c.path === `${base}/jobs`).at(-1)!.search.get("sort")).toBe("recent");
  });

  it("shows a score ring only for rows that have a match", async () => {
    browse({
      [`GET ${base}/jobs`]: { body: { data: [matched(1), makeWorkspaceJob(5, { title: "Plain Engineer 5", match: null })], next_cursor: null } },
    });
    renderApp("/jobs");
    const withMatch = await screen.findByRole("button", { name: /Full Stack Engineer 1/ });
    expect(within(withMatch).getByRole("img", { name: "Match score 89 of 100" })).toBeInTheDocument();
    const plain = screen.getByRole("button", { name: /Plain Engineer 5/ });
    expect(within(plain).queryByRole("img")).toBeNull();
  });

  it("marks the selected row and opens the filters in a drawer that Escape closes", async () => {
    browse();
    renderApp(`/jobs?job=${makeJob(1).id}`);
    const row = await screen.findByRole("button", { name: /Full Stack Engineer 1/ });
    expect(row).toHaveAttribute("aria-current", "true");
    expect(screen.queryByRole("dialog")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Filters" }));
    expect(screen.getByRole("dialog", { name: "Filters" })).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("opens the country filter with its search box focused and narrows the list", async () => {
    const api = browse();
    api.set(`GET ${base}/jobs/facets`, { body: { data: { ...facets, country: { TR: 12, DE: 7, US: 30, FR: 4 } } } });
    const { router } = renderApp("/jobs");
    await userEvent.click(await screen.findByRole("button", { name: /^Country/ }));
    const box = await screen.findByRole("searchbox", { name: "Search Country" });
    await vi.waitFor(() => expect(box).toHaveFocus());
    await userEvent.type(box, "germ");
    expect(screen.queryByRole("checkbox", { name: /United States/ })).toBeNull();
    await userEvent.click(screen.getByRole("checkbox", { name: /Germany/ }));
    await vi.waitFor(() => expect(router.state.location.search).toContain("country=DE"));
  });

  it("loads the next page by itself when the end of the list scrolls into view", async () => {
    const observers: Array<() => void> = [];
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        constructor(cb: (e: Array<{ isIntersecting: boolean }>) => void) {
          observers.push(() => cb([{ isIntersecting: true }]));
        }
        observe() {}
        disconnect() {}
      },
    );
    browse();
    renderApp("/jobs");
    await screen.findByRole("button", { name: /Full Stack Engineer 1/ });
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
    await vi.waitFor(() => expect(observers.length).toBeGreaterThan(0));
    observers.at(-1)!();
    expect(await screen.findByRole("button", { name: /Full Stack Engineer 3/ })).toBeInTheDocument();
  });

  it("restores where the user left off and saves later changes", async () => {
    const api = browse({
      [`GET ${base}/state/jobs_view`]: { body: { data: { query: `technology=react&job=${makeJob(1).id}` } } },
      [`PUT ${base}/state/jobs_view`]: { status: 204 },
    });
    const { router } = renderApp("/jobs");
    await vi.waitFor(() => expect(router.state.location.search).toContain("technology=react"));
    expect(router.state.location.search).toContain(`job=${makeJob(1).id}`);
    await screen.findByRole("region", { name: "Full Stack Engineer 1" });
    await userEvent.click(await screen.findByRole("button", { name: /Full Stack Engineer 2/ }));
    await vi.waitFor(() => expect(api.calls.some((c) => c.method === "PUT" && String((c.body as { query: string }).query).includes(makeJob(2).id))).toBe(true), { timeout: 3000 });
  });

  it("shows the selected job with reasons, saves it and links to the career page safely", async () => {
    const api = browse({ [`PUT ${base}/saved-jobs/${makeJob(1).id}`]: { status: 204 } });
    const { router } = renderApp("/jobs");
    await userEvent.click(await screen.findByRole("button", { name: /Full Stack Engineer 1/ }));
    const pane = await screen.findByRole("region", { name: "Full Stack Engineer 1" });
    expect(router.state.location.search).toContain(`job=${makeJob(1).id}`);
    expect(within(pane).getByText("3 of your 5 technologies")).toBeInTheDocument();
    expect(within(pane).getByRole("img", { name: "Match score 89 of 100" })).toBeInTheDocument();
    const link = await within(pane).findByRole("link", { name: /Career page/ });
    expect(link).toHaveAttribute("href", "https://boards.greenhouse.io/acme/jobs/1");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
    await userEvent.click(within(pane).getByRole("button", { name: "Save job" }));
    await vi.waitFor(() => expect(api.calls.some((c) => c.method === "PUT")).toBe(true));
  });

  it("moves through jobs and saves with the keyboard", async () => {
    const api = browse({ [`PUT ${base}/saved-jobs/${makeJob(2).id}`]: { status: 204 } });
    const { router } = renderApp(`/jobs?job=${makeJob(1).id}`);
    await screen.findByRole("region", { name: "Full Stack Engineer 1" });
    await userEvent.keyboard("j");
    await vi.waitFor(() => expect(router.state.location.search).toContain(`job=${makeJob(2).id}`));
    await userEvent.keyboard("s");
    await vi.waitFor(() => expect(api.calls.some((c) => c.method === "PUT" && c.path.endsWith(makeJob(2).id))).toBe(true));
    await userEvent.keyboard("k");
    await vi.waitFor(() => expect(router.state.location.search).toContain(`job=${makeJob(1).id}`));
  });

  it("records an application from the job and takes it back", async () => {
    const app = { id: "a1", job_id: makeJob(1).id, status: "planned", created_by: null, created_at: "2026-10-17T10:00:00Z", updated_at: "2026-10-17T10:00:00Z" };
    let list: unknown[] = [];
    const api = browse({
      [`GET ${base}/applications`]: () => ({ body: { data: list } }),
      [`POST ${base}/applications`]: () => {
        list = [app];
        return { status: 201, body: { data: app } };
      },
      [`PATCH ${base}/applications/a1`]: () => {
        list = [{ ...app, status: "applied" }];
        return { body: { data: { ...app, status: "applied" } } };
      },
      [`DELETE ${base}/applications/a1`]: () => {
        list = [];
        return { status: 204 };
      },
    });
    const { router } = renderApp(`/jobs?job=${makeJob(1).id}`);
    const pane = await screen.findByRole("region", { name: "Full Stack Engineer 1" });
    await userEvent.click(within(pane).getByRole("button", { name: "I applied" }));
    // The user stays on the job; the pane now shows the status and a way back.
    expect(await within(pane).findByRole("link", { name: "Applied" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/jobs");
    expect(api.calls.find((c) => c.method === "PATCH")?.body).toEqual({ status: "applied" });
    await userEvent.click(within(pane).getByRole("button", { name: "Undo" }));
    expect(await within(pane).findByRole("button", { name: "I applied" })).toBeInTheDocument();
    expect(api.calls.some((c) => c.method === "DELETE")).toBe(true);
  });

  it("marks closed jobs and explains an empty catalog", async () => {
    browse({
      [`GET ${base}/jobs`]: { body: { data: [matched(1, { closed_at: "2026-10-15T00:00:00Z" })], next_cursor: null } },
    });
    renderApp("/jobs");
    expect(await screen.findByText("No longer listed")).toBeInTheDocument();

    vi.unstubAllGlobals();
  });

  it("shows an empty state with a research hint", async () => {
    browse({ [`GET ${base}/jobs`]: { body: { data: [], next_cursor: null } } });
    renderApp("/jobs");
    expect(await screen.findByText("No jobs match these filters.")).toBeInTheDocument();
    expect(screen.getByText("Run a research to collect jobs from career pages.")).toBeInTheDocument();
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
