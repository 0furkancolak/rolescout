import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { makeProfile, makeRun, makeWorkspaceJob, mockApi, PROFILE, renderApp, RUN, signedIn, WS } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

const base = `/workspaces/${WS}`;

const activity = {
  days: ["2026-10-15", "2026-10-16", "2026-10-17"],
  new_matches: [1, 2, 5],
  jobs_collected: [100, 0, 30],
  applications_created: [0, 1, 0],
  upcoming_interviews: 2,
};

describe("DashboardPage", () => {
  it("invites to run the first research", async () => {
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [makeProfile()] } },
      [`GET ${base}/research-runs`]: { body: { data: [] } },
      [`GET ${base}/applications`]: { body: { data: [] } },
      [`GET ${base}/jobs`]: { body: { data: [], next_cursor: null } },
      [`POST ${base}/research-profiles/${PROFILE}/runs`]: { status: 202, body: { data: makeRun({ state: "queued" }) } },
      [`GET ${base}/research-runs/${RUN}`]: { body: { data: makeRun({ state: "queued" }) } },
      [`GET ${base}/research-profiles/${PROFILE}`]: { body: { data: makeProfile() } },
      [`GET ${base}/research-runs/${RUN}/events`]: { body: { data: [] } },
    });
    const { router } = renderApp("/");
    expect(await screen.findByText("No research yet")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Run research now" }));
    await vi.waitFor(() => expect(router.state.location.pathname).toBe(`/research/runs/${RUN}`));
    expect(api.calls.some((c) => c.method === "POST")).toBe(true);
  });

  it("shows the last run, new matches and application counts", async () => {
    mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [makeProfile()] } },
      [`GET ${base}/research-runs`]: { body: { data: [makeRun({ state: "succeeded", finished_at: "2026-10-17T10:20:00Z", summary: { matches: 612, new_matches: 40 } })] } },
      [`GET ${base}/applications`]: { body: { data: [
        { id: "a1", job_id: "j1", status: "applied", created_by: null, created_at: "2026-10-10T10:00:00Z", updated_at: "2026-10-10T10:00:00Z" },
        { id: "a2", job_id: "j2", status: "applied", created_by: null, created_at: "2026-10-10T10:00:00Z", updated_at: "2026-10-10T10:00:00Z" },
        { id: "a3", job_id: "j3", status: "interviewing", created_by: null, created_at: "2026-10-10T10:00:00Z", updated_at: "2026-10-10T10:00:00Z" },
      ] } },
      [`GET ${base}/activity`]: { body: { data: activity } },
      [`GET ${base}/jobs`]: (call) => {
        expect(call.search.get("profile")).toBe(PROFILE);
        expect(call.search.get("sort")).toBe("score");
        return { body: { data: [makeWorkspaceJob(1, { title: "Senior Full Stack Engineer", match: { score: 92, new: true, reasons: [] } })], next_cursor: null } };
      },
    });
    renderApp("/");
    expect(await screen.findByRole("link", { name: "Senior Full Stack Engineer" })).toBeInTheDocument();
    expect(screen.getByText("92")).toBeInTheDocument();
    expect(await screen.findByText("Finished")).toBeInTheDocument();
    expect(screen.getByText("Applied").closest("li")).toHaveTextContent("2");
  });

  it("shows stat cards with numbers and accessible series", async () => {
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [makeProfile()] } },
      [`GET ${base}/research-runs`]: { body: { data: [makeRun({ state: "succeeded", finished_at: "2026-10-17T10:20:00Z", summary: { matches: 612, new_matches: 40, jobs_open: 30805 } })] } },
      [`GET ${base}/applications`]: { body: { data: [
        { id: "a1", job_id: "j1", status: "applied", created_by: null, created_at: "2026-10-10T10:00:00Z", updated_at: "2026-10-10T10:00:00Z" },
        { id: "a2", job_id: "j2", status: "interviewing", created_by: null, created_at: "2026-10-10T10:00:00Z", updated_at: "2026-10-10T10:00:00Z" },
        { id: "a3", job_id: "j3", status: "rejected", created_by: null, created_at: "2026-10-10T10:00:00Z", updated_at: "2026-10-10T10:00:00Z" },
      ] } },
      [`GET ${base}/activity`]: { body: { data: activity } },
      [`GET ${base}/jobs`]: { body: { data: [], next_cursor: null } },
    });
    renderApp("/");
    expect(await screen.findByRole("img", { name: "New matches today: 1, 2, 5" })).toBeInTheDocument();
    const matches = screen.getByRole("link", { name: /New matches today/ });
    expect(matches).toHaveTextContent("5");
    expect(matches).toHaveAttribute("href", expect.stringContaining("/jobs"));
    expect(screen.getByText("Open jobs tracked").closest("div[class*='rounded-card']")).toHaveTextContent("30,805");
    // applied + interviewing count as in progress; rejected does not.
    expect(screen.getByText("Applications in progress").closest("a")).toHaveTextContent("2");
    expect(screen.getByText("Interviews upcoming").closest("a, div[class*='rounded-card']")).toHaveTextContent("2");
    expect(api.calls.find((c) => c.path === `${base}/activity`)?.search.get("days")).toBe("14");
  });

  it("still renders the cards when the activity series is unavailable", async () => {
    mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [makeProfile()] } },
      [`GET ${base}/research-runs`]: { body: { data: [makeRun({ state: "succeeded", finished_at: "2026-10-17T10:20:00Z", summary: { jobs_open: 12 } })] } },
      [`GET ${base}/applications`]: { body: { data: [] } },
      [`GET ${base}/activity`]: { status: 500, body: { error: { code: "internal", message: "boom" } } },
      [`GET ${base}/jobs`]: { body: { data: [], next_cursor: null } },
    });
    renderApp("/");
    expect(await screen.findByText("Open jobs tracked")).toBeInTheDocument();
    expect(screen.getByText("New matches today").closest("a")).toHaveTextContent("–");
    expect(screen.queryByRole("img", { name: /New matches today/ })).toBeNull();
  });
});
