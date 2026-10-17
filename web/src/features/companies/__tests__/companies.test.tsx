import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { mockApi, renderApp, signedIn, user, WS } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

const base = `/workspaces/${WS}`;
const company = (overrides = {}) => ({
  id: "66666666-6666-4666-8666-666666666666", name: "Acme", careers_url: "https://jobs.lever.co/acme", ats: "lever",
  status: "active", origin: "seed", open_jobs: 12, last_checked_at: "2026-10-17T09:00:00Z", last_error: null, blocked: false, ...overrides,
});

describe("CompaniesPage", () => {
  it("adds a company and validates the URL", async () => {
    let added = false;
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/companies`]: () => ({ body: { data: added ? [company({ name: "Beta", status: "pending", ats: "unknown", open_jobs: 0 })] : [], next_cursor: null } }),
      "POST /companies": (call) => {
        added = true;
        return { status: 202, body: { data: company({ name: "Beta", status: "pending", careers_url: (call.body as { careers_url: string }).careers_url }) } };
      },
    });
    renderApp("/companies");
    await userEvent.click(await screen.findByRole("button", { name: "Add company" }));
    const dialog = screen.getByRole("dialog", { name: "Add a company by its career page" });
    await userEvent.type(within(dialog).getByLabelText("Career page URL"), "beta.example");
    await userEvent.click(within(dialog).getByRole("button", { name: "Add" }));
    expect(await within(dialog).findByText("Enter a full http(s) address.")).toBeInTheDocument();
    await userEvent.clear(within(dialog).getByLabelText("Career page URL"));
    await userEvent.type(within(dialog).getByLabelText("Career page URL"), "https://beta.example/careers");
    await userEvent.click(within(dialog).getByRole("button", { name: "Add" }));
    expect(await screen.findByText("Beta was added. The next research run reads it.")).toBeInTheDocument();
    const row = (await screen.findByText("Beta")).closest("tr")!;
    expect(within(row).getByText("Waiting")).toBeInTheDocument();
    expect(api.calls.find((c) => c.method === "POST")?.body).toEqual({ careers_url: "https://beta.example/careers" });
  });

  it("hides a company in this workspace", async () => {
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/companies`]: { body: { data: [company()], next_cursor: null } },
      [`PUT ${base}/company-blocks/${company().id}`]: { status: 204 },
    });
    renderApp("/companies");
    const row = (await screen.findByText("Acme")).closest("tr")!;
    expect(within(row).getByText("12")).toBeInTheDocument();
    expect(within(row).queryByRole("button", { name: "Disable for everyone" })).not.toBeInTheDocument();
    await userEvent.click(within(row).getByRole("button", { name: "Hide" }));
    await vi.waitFor(() => expect(api.calls.some((c) => c.method === "PUT")).toBe(true));
  });

  it("lets the administrator disable a company for everyone", async () => {
    const api = mockApi({
      ...signedIn(),
      "GET /auth/me": { body: { data: { ...user, is_admin: true } } },
      [`GET ${base}/companies`]: { body: { data: [company()], next_cursor: null } },
      [`PATCH /admin/companies/${company().id}`]: { body: { data: company({ status: "disabled" }) } },
    });
    renderApp("/companies");
    const row = (await screen.findByText("Acme")).closest("tr")!;
    await userEvent.click(within(row).getByRole("button", { name: "Disable for everyone" }));
    await vi.waitFor(() => expect(api.calls.find((c) => c.method === "PATCH")?.body).toEqual({ disabled: true }));
  });

  it("filters by search, job board and status", async () => {
    const api = mockApi({ ...signedIn(), [`GET ${base}/companies`]: { body: { data: [], next_cursor: null } } });
    renderApp("/companies");
    await userEvent.type(await screen.findByLabelText("Search companies"), "acme{Enter}");
    await userEvent.selectOptions(screen.getByLabelText("Status"), "unreachable");
    await vi.waitFor(() => {
      const last = api.calls.filter((c) => c.path === `${base}/companies`).at(-1)!;
      expect(last.search.get("q")).toBe("acme");
      expect(last.search.get("status")).toBe("unreachable");
    });
  });
});
