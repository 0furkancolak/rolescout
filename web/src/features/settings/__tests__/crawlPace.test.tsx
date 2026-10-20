import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderApp, signedIn, user } from "../../../test/utils";

const pace = (over = {}) => ({
  concurrency: 128, host_interval_ms: 500, vendor_concurrency: 16, vendor_interval_ms: 100,
  defaults: { concurrency: 128, host_interval_ms: 500, vendor_concurrency: 16, vendor_interval_ms: 100 },
  stored: {}, ...over,
});

const base = {
  ...signedIn(),
  "GET /auth/me": { body: { data: { ...user, is_admin: true } } },
  "GET /admin/settings": { body: { data: [] } },
};

describe("CrawlPaceCard", () => {
  it("shows the pace in effect with its defaults", async () => {
    mockApi({ ...base, "GET /admin/crawl/config": { body: { data: pace() } } });
    renderApp("/settings?tab=crawl");
    expect(await screen.findByLabelText("Companies at once")).toHaveValue(128);
    expect(screen.getByLabelText("Requests per job board")).toHaveValue(16);
    expect(screen.getByText(/Default 128/)).toBeInTheDocument();
  });

  it("saves a higher pace", async () => {
    const api = mockApi({
      ...base,
      "GET /admin/crawl/config": { body: { data: pace() } },
      "PUT /admin/crawl/config": { body: { data: pace({ concurrency: 400, stored: { "crawl.concurrency": true } }) } },
    });
    renderApp("/settings?tab=crawl");
    const input = await screen.findByLabelText("Companies at once");
    await userEvent.clear(input);
    await userEvent.type(input, "400");
    await userEvent.click(screen.getAllByRole("button", { name: "Save" })[0]);
    expect(await screen.findByText(/The worker follows within a few seconds/)).toBeInTheDocument();
    const put = api.calls.find((c) => c.method === "PUT" && c.path === "/admin/crawl/config");
    expect(put?.body).toMatchObject({ concurrency: 400, host_interval_ms: 500, vendor_concurrency: 16, vendor_interval_ms: 100 });
  });

  it("goes back to the defaults", async () => {
    const api = mockApi({
      ...base,
      "GET /admin/crawl/config": { body: { data: pace({ concurrency: 400 }) } },
      "PUT /admin/crawl/config": { body: { data: pace() } },
    });
    renderApp("/settings?tab=crawl");
    await userEvent.click(await screen.findByRole("button", { name: "Back to defaults" }));
    await screen.findByText(/The worker follows within a few seconds/);
    expect(api.calls.find((c) => c.method === "PUT")?.body).toEqual({ concurrency: 0, host_interval_ms: 0, vendor_concurrency: 0, vendor_interval_ms: 0 });
  });
});
