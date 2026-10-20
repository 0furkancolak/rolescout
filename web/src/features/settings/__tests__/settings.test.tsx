import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { mockApi, renderApp, signedIn, user } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

const admin = { "GET /auth/me": { body: { data: { ...user, is_admin: true } } } };

describe("SettingsPage", () => {
  it("requires a review note to enable an adapter type", async () => {
    const api = mockApi({
      ...signedIn(),
      ...admin,
      "GET /admin/adapters": { body: { data: [
        { adapter: "greenhouse", enabled: true, reviewed_at: "2026-10-18T00:00:00Z", note: "public API" },
        { adapter: "workday", enabled: false, reviewed_at: null, note: null },
      ] } },
      "GET /admin/settings": { body: { data: [{ name: "CRAWL_CONCURRENCY", value: "16" }] } },
      "PATCH /admin/adapters/workday": (call) => ({ body: { data: { adapter: "workday", enabled: true, reviewed_at: "2026-10-18T00:00:00Z", note: (call.body as { note: string }).note } } }),
    });
    renderApp("/settings");
    const row = await screen.findByRole("group", { name: "workday" });
    expect(within(row).getByText(/no documented public API/)).toBeInTheDocument();
    await userEvent.click(within(row).getByRole("button", { name: "Enable" }));
    expect(await within(row).findByText("Write a review note before enabling.")).toBeInTheDocument();
    expect(api.calls.some((c) => c.method === "PATCH")).toBe(false);
    await userEvent.type(within(row).getByLabelText("Review note"), "Checked the employers' terms");
    await userEvent.click(within(row).getByRole("button", { name: "Enable" }));
    await vi.waitFor(() =>
      expect(api.calls.find((c) => c.method === "PATCH")?.body).toEqual({ enabled: true, note: "Checked the employers' terms" }),
    );
    await userEvent.click(screen.getByRole("tab", { name: "Crawl" }));
    expect(await screen.findByText("CRAWL_CONCURRENCY")).toBeInTheDocument();
  });

  it("is not available to other users", async () => {
    mockApi({ ...signedIn() });
    renderApp("/settings");
    expect(await screen.findByRole("heading", { name: "Not found" })).toBeInTheDocument();
  });
});
