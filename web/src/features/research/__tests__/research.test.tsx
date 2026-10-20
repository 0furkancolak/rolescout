import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { makeProfile, makeRun, mockApi, PROFILE, renderApp, RUN, signedIn, WS } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

const base = `/workspaces/${WS}`;

describe("RunPage", () => {
  it("shows stage progress and appends log lines as they arrive", async () => {
    let polls = 0;
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/research-runs/${RUN}`]: () => {
        polls++;
        return { body: { data: makeRun() } };
      },
      [`GET ${base}/research-profiles/${PROFILE}`]: { body: { data: makeProfile() } },
      [`GET ${base}/research-runs/${RUN}/events`]: (call) =>
        call.search.get("after") === "0"
          ? { body: { data: [{ id: 1, at: "2026-10-17T10:00:02Z", level: "info", stage: "collection", company: "Stripe", message: "greenhouse · 412 jobs" }] } }
          : { body: { data: call.search.get("after") === "1"
              ? [{ id: 2, at: "2026-10-17T10:00:03Z", level: "error", stage: "collection", company: "Acme", message: "greenhouse · board not found" }]
              : [] } },
    });
    renderApp(`/research/runs/${RUN}`);

    const collection = await screen.findByRole("region", { name: "Job collection" });
    expect(within(collection).getByText("64 / 100")).toBeInTheDocument();
    expect(within(collection).getByText("3 failed")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "Career page detection" })).getByText(/greenhouse/)).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "Matching and scoring" })).getByText("Waiting")).toBeInTheDocument();

    const log = screen.getByRole("log");
    expect(await within(log).findByText(/412 jobs/)).toBeInTheDocument();
    expect(await within(log).findByText(/board not found/)).toBeInTheDocument();
    expect(polls).toBeGreaterThan(1);
    expect(api.calls.some((c) => c.path.endsWith("/events") && c.search.get("after") === "1")).toBe(true);
  });

  it("cancels a running run", async () => {
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/research-runs/${RUN}`]: { body: { data: makeRun() } },
      [`GET ${base}/research-profiles/${PROFILE}`]: { body: { data: makeProfile() } },
      [`GET ${base}/research-runs/${RUN}/events`]: { body: { data: [] } },
      [`POST ${base}/research-runs/${RUN}/cancel`]: { status: 202, body: { data: makeRun({ cancel_requested: true }) } },
    });
    renderApp(`/research/runs/${RUN}`);
    await userEvent.click(await screen.findByRole("button", { name: "Cancel run" }));
    await vi.waitFor(() => expect(api.calls.some((c) => c.method === "POST" && c.path.endsWith("/cancel"))).toBe(true));
  });

  it("stops polling and shows the summary when the run finished", async () => {
    let polls = 0;
    const finished = makeRun({
      state: "succeeded", stage: "result", finished_at: "2026-10-17T10:20:00Z",
      summary: { companies_known: 2431, jobs_collected: 31540, matches: 612, new_matches: 40 },
    });
    mockApi({
      ...signedIn(),
      [`GET ${base}/research-runs/${RUN}`]: () => {
        polls++;
        return { body: { data: finished } };
      },
      [`GET ${base}/research-profiles/${PROFILE}`]: { body: { data: makeProfile() } },
      [`GET ${base}/research-runs/${RUN}/events`]: { body: { data: [] } },
    });
    renderApp(`/research/runs/${RUN}`);
    expect(await screen.findByText("612")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "See matches" })).toHaveAttribute("href", `/jobs?profile=${PROFILE}&sort=score`);
    const before = polls;
    await new Promise((r) => setTimeout(r, 200));
    expect(polls).toBe(before);
    expect(screen.queryByRole("button", { name: "Cancel run" })).not.toBeInTheDocument();
  });
});

describe("RunPage log", () => {
  it("reads the last log lines once the run finishes", async () => {
    let runPolls = 0;
    mockApi({
      ...signedIn(),
      [`GET ${base}/research-runs/${RUN}`]: () => {
        runPolls++;
        return { body: { data: runPolls < 3 ? makeRun() : makeRun({ state: "succeeded", finished_at: "2026-10-17T10:20:00Z" }) } };
      },
      [`GET ${base}/research-profiles/${PROFILE}`]: { body: { data: makeProfile() } },
      [`GET ${base}/research-runs/${RUN}/events`]: (call) => {
        if (call.search.get("after") === "0") {
          return { body: { data: [{ id: 1, at: "2026-10-17T10:00:02Z", level: "info", stage: "collection", company: "", message: "first line" }] } };
        }
        // The final summary line is only written when the run ends.
        return { body: { data: runPolls >= 3 ? [{ id: 2, at: "2026-10-17T10:20:00Z", level: "info", stage: "scoring", company: "", message: "612 of 31540 open jobs match" }] : [] } };
      },
    });
    renderApp(`/research/runs/${RUN}`);
    const log = await screen.findByRole("log");
    expect(await within(log).findByText("first line")).toBeInTheDocument();
    expect(await within(log).findByText(/612 of 31540/)).toBeInTheDocument();
  });
});

describe("Research profiles", () => {
  it("validates and saves a new profile", async () => {
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [makeProfile()] } },
      [`GET ${base}/research-runs`]: { body: { data: [] } },
      [`POST ${base}/research-profiles`]: { status: 201, body: { data: makeProfile({ id: "77777777-7777-4777-8777-777777777777", name: "Backend" }) } },
    });
    renderApp("/research/profiles/new");
    await userEvent.click(await screen.findByRole("button", { name: "Save" }));
    expect(await screen.findByText("Give the profile a name.")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Name"), "Backend");
    await userEvent.type(screen.getByLabelText("Technologies"), "Go{Enter}PostgreSQL{Enter}");
    await userEvent.type(screen.getByLabelText("Countries"), "tr{Enter}");
    await userEvent.click(screen.getByLabelText("Remote only"));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(api.calls.some((c) => c.method === "POST" && c.path === `${base}/research-profiles`)).toBe(true));
    const body = api.calls.find((c) => c.method === "POST")?.body as Record<string, unknown>;
    expect(body).toMatchObject({ name: "Backend", technologies: ["go", "postgresql"], countries: ["TR"], remote: "remote_only" });
  });

  it("drafts a profile with AI and saves it only after review", async () => {
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [] } },
      [`GET ${base}/research-runs`]: { body: { data: [] } },
      [`POST ${base}/research-profiles/suggest`]: {
        body: { data: { name: "Go backend, Berlin", title_keywords: ["backend engineer"], keywords: [], technologies: ["go", "postgresql"], countries: ["DE"], locations: ["Berlin"], excluded_keywords: ["php"], remote: "remote_only", seniority: ["senior"], schedule_hours: 12 } },
      },
      [`POST ${base}/research-profiles`]: { status: 201, body: { data: makeProfile() } },
    });
    renderApp("/research/profiles/new");
    const draft = await screen.findByRole("button", { name: "Draft with AI" });
    expect(draft).toBeDisabled();
    await userEvent.type(screen.getByLabelText("Describe it, let AI draft the profile"), "Senior Go backend in Berlin, remote, no PHP");
    await userEvent.click(draft);
    expect(await screen.findByText(/Draft ready below/)).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveValue("Go backend, Berlin");
    expect(screen.getByLabelText("Remote only")).toBeChecked();
    expect(api.calls.some((c) => c.method === "POST" && c.path === `${base}/research-profiles`)).toBe(false);
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(api.calls.some((c) => c.method === "POST" && c.path === `${base}/research-profiles`)).toBe(true));
    expect(api.calls.find((c) => c.path === `${base}/research-profiles` && c.method === "POST")?.body).toMatchObject({ technologies: ["go", "postgresql"], countries: ["DE"] });
  });

  it("explains when AI is off", async () => {
    mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [] } },
      [`GET ${base}/research-runs`]: { body: { data: [] } },
      [`POST ${base}/research-profiles/suggest`]: { status: 412, body: { error: { code: "ai_disabled", message: "off" } } },
    });
    renderApp("/research/profiles/new");
    await userEvent.type(await screen.findByLabelText("Describe it, let AI draft the profile"), "Senior Go backend in Berlin");
    await userEvent.click(screen.getByRole("button", { name: "Draft with AI" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("AI is switched off");
  });

  it("limits list fields to 30 items", async () => {
    mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [] } },
      [`GET ${base}/research-runs`]: { body: { data: [] } },
    });
    renderApp("/research/profiles/new");
    const input = await screen.findByLabelText("Other keywords");
    for (let i = 0; i < 31; i++) await userEvent.type(input, `k${i}{Enter}`);
    expect(screen.getByText("At most 30 items.")).toBeInTheDocument();
  });

  it("lists profiles and starts a run", async () => {
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [makeProfile()] } },
      [`GET ${base}/research-runs`]: { body: { data: [makeRun({ state: "succeeded", finished_at: "2026-10-17T10:20:00Z" })] } },
      [`POST ${base}/research-profiles/${PROFILE}/runs`]: { status: 202, body: { data: makeRun({ state: "queued" }) } },
      [`GET ${base}/research-runs/${RUN}`]: { body: { data: makeRun({ state: "queued" }) } },
      [`GET ${base}/research-profiles/${PROFILE}`]: { body: { data: makeProfile() } },
      [`GET ${base}/research-runs/${RUN}/events`]: { body: { data: [] } },
    });
    const { router } = renderApp("/research");
    expect(await screen.findByRole("heading", { name: "Full Stack Developer" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Run now" }));
    await vi.waitFor(() => expect(router.state.location.pathname).toBe(`/research/runs/${RUN}`));
    expect(api.calls.some((c) => c.method === "POST" && c.path.endsWith("/runs"))).toBe(true);
  });
});
