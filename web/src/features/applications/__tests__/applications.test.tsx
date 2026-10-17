import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Application, ApplicationEvent, Note } from "../../../lib/api";
import { makeJob, mockApi, renderApp, signedIn, WS } from "../../../test/utils";
import { todayInput, toISODate } from "../format";

afterEach(() => vi.unstubAllGlobals());

const job = makeJob(1);
const app: Application = {
  id: "44444444-4444-4444-8444-444444444444",
  job_id: job.id,
  status: "applied",
  created_by: null,
  created_at: "2026-10-10T10:00:00Z",
  updated_at: "2026-10-11T10:00:00Z",
};
const base = `/workspaces/${WS}/applications/${app.id}`;

function event(id: string, partial: Partial<ApplicationEvent>): ApplicationEvent {
  return {
    id,
    type: "created",
    attempt_id: null,
    from_status: null,
    to_status: null,
    occurred_at: "2026-10-10T10:00:00Z",
    details: {},
    created_by: null,
    created_at: "2026-10-10T10:00:00Z",
    ...partial,
  };
}

function detailHandlers(state: { app: Application; events: ApplicationEvent[]; notes: Note[] }) {
  return {
    ...signedIn(),
    [`GET /jobs/${job.id}`]: { body: { data: { ...job, listings: [] } } },
    [`GET ${base}`]: () => ({ body: { data: state.app } }),
    [`GET ${base}/events`]: () => ({ body: { data: state.events } }),
    [`GET ${base}/attempts`]: { body: { data: [] } },
    [`GET ${base}/notes`]: () => ({ body: { data: state.notes } }),
    [`GET ${base}/tags`]: { body: { data: [] } },
    [`GET /workspaces/${WS}/tags`]: { body: { data: [] } },
  };
}

describe("ApplicationsPage", () => {
  it("filters by status", async () => {
    const api = mockApi({
      ...signedIn(),
      [`GET /workspaces/${WS}/applications`]: (call) => ({
        body: { data: call.search.get("status") === "offer" ? [] : [app] },
      }),
      [`GET /jobs/${job.id}`]: { body: { data: { ...job, listings: [] } } },
    });
    const { router } = renderApp("/applications");
    expect(await screen.findByText("Backend Engineer 1")).toBeInTheDocument();
    expect(screen.getByText("Applied", { selector: "span" })).toBeInTheDocument();

    const filters = screen.getByRole("navigation", { name: "Filter by status" });
    await userEvent.click(within(filters).getByRole("button", { name: "Offer" }));
    expect(await screen.findByText("No applications with this status.")).toBeInTheDocument();
    expect(router.state.location.search).toBe("?status=offer");
    expect(api.calls.at(-1)?.search.get("status")).toBe("offer");
  });

  it("shows an empty state", async () => {
    mockApi({ ...signedIn(), [`GET /workspaces/${WS}/applications`]: { body: { data: [] } } });
    renderApp("/applications");
    expect(await screen.findByText("No applications yet.")).toBeInTheDocument();
  });
});

describe("ApplicationDetailPage", () => {
  it("offers only allowed status changes and records the change", async () => {
    const state = {
      app,
      events: [event("e1", { to_status: "planned" })],
      notes: [] as Note[],
    };
    const api = mockApi({
      ...detailHandlers(state),
      [`PATCH ${base}`]: (call) => {
        const to = (call.body as { status: Application["status"] }).status;
        state.events = [
          ...state.events,
          event("e2", { type: "status_changed", from_status: state.app.status, to_status: to, occurred_at: "2026-10-12T10:00:00Z" }),
        ];
        state.app = { ...state.app, status: to };
        return { body: { data: state.app } };
      },
    });
    renderApp(`/applications/${app.id}`);

    const select = await screen.findByLabelText("Change status");
    const options = within(select).getAllByRole("option").map((o) => o.getAttribute("value"));
    expect(options).toEqual(["", "interviewing", "offer", "rejected", "withdrawn"]);

    await userEvent.selectOptions(select, "interviewing");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Status changed from Applied to Interviewing")).toBeInTheDocument();
    expect(api.calls.find((c) => c.method === "PATCH")?.body).toEqual({ status: "interviewing" });
    expect(screen.getByText("Tracking started")).toBeInTheDocument();
  });

  it("explains rejected transitions", async () => {
    mockApi({
      ...detailHandlers({ app, events: [], notes: [] }),
      [`PATCH ${base}`]: { status: 409, body: { error: { code: "invalid_transition", message: "" } } },
    });
    renderApp(`/applications/${app.id}`);
    await userEvent.selectOptions(await screen.findByLabelText("Change status"), "offer");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("This status change is not allowed from the current status.")).toBeInTheDocument();
  });

  it("adds and edits notes", async () => {
    const state = { app, events: [] as ApplicationEvent[], notes: [] as Note[] };
    const api = mockApi({
      ...detailHandlers(state),
      [`POST ${base}/notes`]: (call) => {
        const note = {
          id: "n1",
          author_id: null,
          body: (call.body as { body: string }).body,
          created_at: "2026-10-12T10:00:00Z",
          updated_at: "2026-10-12T10:00:00Z",
        };
        state.notes = [note];
        return { status: 201, body: { data: note } };
      },
      [`PATCH ${base}/notes/n1`]: (call) => {
        state.notes = [{ ...state.notes[0], body: (call.body as { body: string }).body, updated_at: "2026-10-13T10:00:00Z" }];
        return { body: { data: state.notes[0] } };
      },
    });
    renderApp(`/applications/${app.id}`);

    await userEvent.type(await screen.findByPlaceholderText("Write a note…"), "  Recruiter call went well  ");
    await userEvent.click(screen.getByRole("button", { name: "Add note" }));
    expect(await screen.findByText("Recruiter call went well")).toBeInTheDocument();
    expect(api.calls.find((c) => c.method === "POST")?.body).toEqual({ body: "Recruiter call went well" });

    await userEvent.click(screen.getByRole("button", { name: "Edit" }));
    const editor = screen.getAllByLabelText("Note").at(-1)!;
    await userEvent.clear(editor);
    await userEvent.type(editor, "Second round on Monday");
    await userEvent.click(screen.getAllByRole("button", { name: "Save" }).at(-1)!);
    expect(await screen.findByText("Second round on Monday")).toBeInTheDocument();
    expect(screen.getByText(/edited/)).toBeInTheDocument();
  });

  it("records an attempt for the chosen day", async () => {
    const api = mockApi({
      ...detailHandlers({ app, events: [], notes: [] }),
      [`POST ${base}/attempts`]: { status: 201, body: { data: {} } },
    });
    renderApp(`/applications/${app.id}`);
    await userEvent.type(await screen.findByLabelText("Channel"), "Referral");
    await userEvent.click(screen.getByRole("button", { name: "Record attempt" }));
    await vi.waitFor(() => expect(api.calls.some((c) => c.path === `${base}/attempts` && c.method === "POST")).toBe(true));
    expect(api.calls.find((c) => c.method === "POST")?.body).toEqual({
      applied_at: toISODate(todayInput()),
      channel: "Referral",
    });
  });

  it("creates a tag and attaches it", async () => {
    const tag = { id: "55555555-5555-4555-8555-555555555555", name: "remote-first", created_at: "2026-10-12T10:00:00Z" };
    let attached: (typeof tag)[] = [];
    mockApi({
      ...detailHandlers({ app, events: [], notes: [] }),
      [`GET ${base}/tags`]: () => ({ body: { data: attached } }),
      [`POST /workspaces/${WS}/tags`]: { status: 201, body: { data: tag } },
      [`PUT ${base}/tags/${tag.id}`]: () => {
        attached = [tag];
        return { status: 204 };
      },
    });
    renderApp(`/applications/${app.id}`);
    await userEvent.type(await screen.findByLabelText("Tag"), "remote-first");
    await userEvent.click(screen.getByRole("button", { name: "Add tag" }));
    expect(await screen.findByRole("button", { name: "Remove tag remote-first" })).toBeInTheDocument();
  });
});

describe("format helpers", () => {
  it("stores dates at local noon so they keep their calendar day", () => {
    const iso = toISODate("2026-03-01");
    expect(new Date(iso).getDate()).toBe(1);
    expect(todayInput(new Date(2026, 0, 5))).toBe("2026-01-05");
  });
});
