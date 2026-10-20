import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { makeJob, mockApi, renderApp, signedIn, WS, type Reply } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

const base = `/workspaces/${WS}`;
const app = (id: string, status: string, n: number) => ({
  id, job_id: makeJob(n).id, status, created_by: null, created_at: "2026-10-10T10:00:00Z", updated_at: "2026-10-10T10:00:00Z",
});

function board(patch: (body: unknown) => Reply = (body) => ({ body: { data: body } })) {
  return mockApi({
    ...signedIn(),
    [`GET ${base}/applications`]: { body: { data: [app("a1", "applied", 1), app("a2", "planned", 2)] } },
    [`GET /jobs/${makeJob(1).id}`]: { body: { data: { ...makeJob(1), listings: [] } } },
    [`GET /jobs/${makeJob(2).id}`]: { body: { data: { ...makeJob(2), listings: [] } } },
    [`PATCH ${base}/applications/a1`]: (call) => patch(call.body),
  });
}

describe("ApplicationsBoard", () => {
  it("moves a card a step forward and back with its buttons", async () => {
    const api = board();
    renderApp("/applications?view=board");
    const card = await screen.findByRole("article", { name: "Backend Engineer 1" });
    await userEvent.click(within(card).getByRole("button", { name: "Move to Interviewing" }));
    await vi.waitFor(() => expect(api.calls.find((c) => c.method === "PATCH")?.body).toEqual({ status: "interviewing" }));
  });

  it("has no step back for the first column", async () => {
    board();
    renderApp("/applications?view=board");
    const card = await screen.findByRole("article", { name: "Backend Engineer 2" });
    expect(within(card).getByRole("button", { name: /Move back/ })).toBeDisabled();
  });

  it("moves a card with the menu and reports a refused change", async () => {
    const api = board(() => ({ status: 409, body: { error: { code: "invalid_transition", message: "no" } } }));
    renderApp("/applications?view=board");
    const card = await screen.findByRole("article", { name: "Backend Engineer 1" });
    await userEvent.selectOptions(within(card).getByRole("combobox", { name: "Change status" }), "offer");
    await vi.waitFor(() => expect(api.calls.find((c) => c.method === "PATCH")?.body).toEqual({ status: "offer" }));
    expect(await within(card).findByText("This status change is not allowed from the current status.")).toBeInTheDocument();
  });

  it("switches between list and board", async () => {
    board();
    const { router } = renderApp("/applications");
    await userEvent.click(await screen.findByRole("radio", { name: "Board" }));
    expect(router.state.location.search).toBe("?view=board");
    expect(await screen.findByRole("region", { name: "Applied" })).toBeInTheDocument();
  });
});
