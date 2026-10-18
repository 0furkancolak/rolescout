import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { mockApi, renderApp, signedIn, user } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

const admin = { "GET /auth/me": { body: { data: { ...user, is_admin: true } } } };
const settingsBase = {
  ...signedIn(),
  ...admin,
  "GET /admin/adapters": { body: { data: [] } },
  "GET /admin/settings": { body: { data: [] } },
};

const status = (over = {}) => ({
  enabled: true, base_host: "host.docker.internal:11434", model: "qwen3:4b-instruct-2507-q4_K_M", fallback_model: "qwen3:1.7b-q4_K_M",
  reachable: true, model_present: true, degraded: false, local: true, queue: 120, enriched: 30, failed: 2, avg_latency_ms: 3400,
  last_error: "", last_checked_at: "2026-10-18T12:00:00Z", probe_ok: false, probe_ms: 0, ...over,
});

async function card() {
  const heading = await screen.findByRole("heading", { name: "Local AI" });
  return heading.closest("section") as HTMLElement;
}

describe("LocalAiCard", () => {
  it("explains how to turn it on when it is off", async () => {
    mockApi({ ...settingsBase, "GET /admin/llm": { body: { data: { enabled: false } } } });
    renderApp("/settings");
    const c = within(await card());
    expect(await c.findByText("Local AI is off")).toBeInTheDocument();
    expect(c.getByText(/LLM_ENABLED=true/)).toBeInTheDocument();
    expect(c.getByRole("link", { name: /local-ai/i })).toHaveAttribute("href", expect.stringContaining("local-ai.md"));
    expect(c.queryByRole("button", { name: "Test connection" })).toBeNull();
  });

  it("tells what to start when the server is unreachable", async () => {
    mockApi({ ...settingsBase, "GET /admin/llm": { body: { data: status({ reachable: false, model_present: false }) } } });
    renderApp("/settings");
    const c = within(await card());
    expect(await c.findByText("Can't reach the model server")).toBeInTheDocument();
    expect(c.getAllByText(/host\.docker\.internal:11434/).length).toBeGreaterThan(0);
    expect(c.getByText(/ollama serve/)).toBeInTheDocument();
  });

  it("shows the pull command when the model is missing", async () => {
    mockApi({ ...settingsBase, "GET /admin/llm": { body: { data: status({ model_present: false }) } } });
    renderApp("/settings");
    const c = within(await card());
    expect(await c.findByText("The model is not on the server")).toBeInTheDocument();
    expect(c.getByText("ollama pull qwen3:4b-instruct-2507-q4_K_M")).toBeInTheDocument();
  });

  it("says when the smaller fallback model is doing the work", async () => {
    mockApi({ ...settingsBase, "GET /admin/llm": { body: { data: status({ degraded: true }) } } });
    renderApp("/settings");
    const c = within(await card());
    expect(await c.findByText("Using the smaller fallback model")).toBeInTheDocument();
    expect(c.getByText(/qwen3:1\.7b-q4_K_M/)).toBeInTheDocument();
  });

  it("shows the counters while it works", async () => {
    mockApi({ ...settingsBase, "GET /admin/llm": { body: { data: status({ last_error: "timeout" }) } } });
    renderApp("/settings");
    const c = within(await card());
    expect(await c.findByText("Working")).toBeInTheDocument();
    const counters = c.getByRole("list", { name: "Enrichment progress" });
    expect(within(counters).getByText("120").closest("li")).toHaveTextContent("Waiting");
    expect(within(counters).getByText("30").closest("li")).toHaveTextContent("Read");
    expect(within(counters).getByText("2").closest("li")).toHaveTextContent("Failed");
    expect(within(counters).getByText("3.4 s").closest("li")).toHaveTextContent("per job");
    expect(c.getByText(/timeout/)).toBeInTheDocument();
  });

  it("tests the connection and shows the answer time", async () => {
    const api = mockApi({
      ...settingsBase,
      "GET /admin/llm": { body: { data: status() } },
      "POST /admin/llm/check": { body: { data: status({ probe_ok: true, probe_ms: 900 }) } },
    });
    renderApp("/settings");
    const c = within(await card());
    await userEvent.click(await c.findByRole("button", { name: "Test connection" }));
    expect(await c.findByText("The model answered in 900 ms")).toBeInTheDocument();
    expect(api.calls.some((x) => x.method === "POST" && x.path === "/admin/llm/check")).toBe(true);
  });

  it("reports a failed test", async () => {
    mockApi({
      ...settingsBase,
      "GET /admin/llm": { body: { data: status() } },
      "POST /admin/llm/check": { body: { data: status({ probe_ok: false }) } },
    });
    renderApp("/settings");
    const c = within(await card());
    await userEvent.click(await c.findByRole("button", { name: "Test connection" }));
    expect(await c.findByText("The model did not answer")).toBeInTheDocument();
  });

  it("explains a rate-limited check", async () => {
    mockApi({
      ...settingsBase,
      "GET /admin/llm": { body: { data: status() } },
      "POST /admin/llm/check": { status: 429, body: { error: { code: "rate_limited", message: "Wait a few seconds before checking again." } } },
    });
    renderApp("/settings");
    const c = within(await card());
    await userEvent.click(await c.findByRole("button", { name: "Test connection" }));
    expect(await c.findByRole("alert")).toHaveTextContent(/few seconds/i);
  });

  it("warns when job text would leave the computer", async () => {
    mockApi({ ...settingsBase, "GET /admin/llm": { body: { data: status({ local: false, base_host: "gpu.example.com:443" }) } } });
    renderApp("/settings");
    const c = within(await card());
    expect(await c.findByRole("alert")).toHaveTextContent("Job text is sent to gpu.example.com:443");
  });
});
