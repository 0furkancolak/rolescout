import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { mockApi, renderApp, signedIn, user } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

const admin = { ...user, is_admin: true };
const base = {
  ...signedIn(),
  "GET /auth/me": { body: { data: admin } },
  "GET /admin/adapters": { body: { data: [] } },
  "GET /admin/settings": { body: { data: [] } },
  "GET /admin/llm": { body: { data: { enabled: false } } },
  "GET /admin/llm/config": { body: { data: { enabled: false, base_url: "", model: "", has_api_key: false, background: true, background_auto: true, local: true, stored: {} } } },
};

async function card() {
  const heading = await screen.findByRole("heading", { name: "People" });
  return within(heading.closest("section") as HTMLElement);
}

describe("UsersCard", () => {
  it("lists accounts and adds a person without signing them in", async () => {
    const created = { ...user, id: "u2", display_name: "Grace", email: "grace@example.com", is_admin: false };
    const api = mockApi({
      ...base,
      "GET /admin/users": { body: { data: [admin] } },
      "POST /admin/users": { status: 201, body: { data: created } },
    });
    renderApp("/settings?tab=people");
    const c = await card();
    expect(await c.findByText(admin.email)).toBeInTheDocument();
    await userEvent.type(c.getByLabelText("Name"), "Grace");
    await userEvent.type(c.getByLabelText("Email"), "grace@example.com");
    await userEvent.type(c.getByLabelText("Password"), "a long enough password");
    await userEvent.click(c.getByRole("button", { name: "Add person" }));
    expect(await c.findByText("Added.")).toBeInTheDocument();
    expect(api.calls.find((x) => x.method === "POST" && x.path === "/admin/users")?.body).toMatchObject({
      display_name: "Grace", email: "grace@example.com", password: "a long enough password",
    });
  });

  it("says when the email is taken", async () => {
    mockApi({
      ...base,
      "GET /admin/users": { body: { data: [admin] } },
      "POST /admin/users": { status: 409, body: { error: { code: "conflict", message: "exists" } } },
    });
    renderApp("/settings?tab=people");
    const c = await card();
    await userEvent.type(await c.findByLabelText("Name"), "Ada");
    await userEvent.type(c.getByLabelText("Email"), "ada@example.com");
    await userEvent.type(c.getByLabelText("Password"), "a long enough password");
    await userEvent.click(c.getByRole("button", { name: "Add person" }));
    expect(await c.findByRole("alert")).toHaveTextContent("already exists");
  });
});

describe("Settings tabs", () => {
  it("shows one section at a time and keeps the tab in the address", async () => {
    mockApi({ ...base, "GET /admin/users": { body: { data: [admin] } } });
    const { router } = renderApp("/settings");
    expect(await screen.findByRole("tab", { name: "Job boards", selected: true })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "People" })).toBeNull();
    await userEvent.click(screen.getByRole("tab", { name: "People" }));
    expect(await screen.findByRole("heading", { name: "People" })).toBeInTheDocument();
    expect(router.state.location.search).toBe("?tab=people");
    await userEvent.keyboard("{ArrowRight}");
    expect(screen.getByRole("tab", { name: "AI", selected: true })).toBeInTheDocument();
  });
});
