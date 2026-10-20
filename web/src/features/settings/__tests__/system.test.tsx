import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { mockApi, renderApp, signedIn, user } from "../../../test/utils";
import { formatBytes } from "../SystemCard";

afterEach(() => vi.unstubAllGlobals());

const MB = 1 << 20;

describe("formatBytes", () => {
  it("uses MB below a gigabyte and GB above", () => {
    expect(formatBytes(512 * MB)).toBe("512 MB");
    expect(formatBytes(12.5 * MB)).toBe("12.5 MB");
    expect(formatBytes(1536 * MB)).toBe("1.5 GB");
    expect(formatBytes(2048 * MB)).toBe("2 GB");
  });
});

describe("SystemCard", () => {
  it("shows used and allowed memory per service", async () => {
    mockApi({
      ...signedIn(),
      "GET /auth/me": { body: { data: { ...user, is_admin: true } } },
      "GET /admin/adapters": { body: { data: [] } },
      "GET /admin/system": {
        body: {
          data: {
            host_total_bytes: 16384 * MB,
            services: [
              { name: "api", used_bytes: 60 * MB, limit_bytes: 512 * MB, heap_bytes: 20 * MB, estimated: false, stale: false },
              { name: "worker", used_bytes: 400 * MB, limit_bytes: 1024 * MB, estimated: false, stale: true },
              { name: "postgres", used_bytes: null, limit_bytes: 1024 * MB, estimated: true, stale: false, note: "container" },
              { name: "web", used_bytes: null, limit_bytes: 128 * MB, estimated: true, stale: false },
            ],
            postgres: { shared_buffers_bytes: 128 * MB, work_mem_bytes: 4 * MB, max_connections: 40, connections: 7, database_bytes: 300 * MB },
          },
        },
      },
    });
    renderApp("/settings?tab=system");
    const card = within((await screen.findByRole("heading", { name: "Memory" })).closest("section") as HTMLElement);
    expect(await card.findByText("460 MB")).toBeInTheDocument(); // api + worker in use
    expect(card.getByText("2.6 GB")).toBeInTheDocument(); // 512 + 1024 + 1024 + 128 MB
    expect(card.getByText("16 GB")).toBeInTheDocument();
    expect(card.getByText("Not reporting. Is it running?")).toBeInTheDocument();
    expect(card.getByText(/7 of 40 connections/)).toBeInTheDocument();
    expect(card.getByRole("progressbar", { name: "API" })).toHaveAttribute("aria-valuenow", "12");
    await userEvent.click(screen.getByRole("tab", { name: "AI" }));
    expect(screen.queryByRole("heading", { name: "Memory" })).toBeNull();
  });
});
