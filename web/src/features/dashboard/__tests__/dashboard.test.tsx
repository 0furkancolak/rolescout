import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { makeProfile, makeRun, makeWorkspaceJob, mockApi, PROFILE, renderApp, RUN, signedIn, WS } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

const base = `/workspaces/${WS}`;

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
});
