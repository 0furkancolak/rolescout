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
