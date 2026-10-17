import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { makeJob, mockApi, renderApp, signedIn, user, workspace } from "../test/utils";

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

const unauthenticated = { status: 401, body: { error: { code: "unauthenticated", message: "Sign in" } } };

describe("App", () => {
  it("sends signed-out visitors to the login page and back after signing in", async () => {
    const api = mockApi({
      "GET /auth/me": unauthenticated,
      "POST /auth/login": { body: { data: user } },
      "GET /workspaces": { body: { data: [workspace] } },
      [`GET /workspaces/${workspace.id}/applications`]: { body: { data: [] } },
    });
    const { router } = renderApp("/applications?status=offer");

    expect(await screen.findByRole("heading", { name: "Sign in to RoleScout" })).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Email"), "ada@example.com");
    await userEvent.type(screen.getByLabelText("Password"), "correct horse battery");
    await userEvent.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByRole("heading", { name: "Applications" })).toBeInTheDocument();
    expect(router.state.location.pathname + router.state.location.search).toBe("/applications?status=offer");
    expect(api.calls.find((c) => c.path === "/auth/login")?.body).toEqual({
      email: "ada@example.com",
      password: "correct horse battery",
    });
  });

  it("renders the app in Turkish and switches language", async () => {
    mockApi({ ...signedIn(), "GET /jobs": { body: { data: [makeJob(1)], next_cursor: null } } });
    renderApp("/jobs", { locale: "tr" });

    expect(await screen.findByRole("heading", { name: "İlan ara" })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Ana menü" })).toBeInTheDocument();
    expect(document.documentElement.lang).toBe("tr");

    await userEvent.selectOptions(screen.getAllByRole("combobox", { name: "Dil" })[0], "en");
    expect(await screen.findByRole("heading", { name: "Find jobs" })).toBeInTheDocument();
    expect(document.documentElement.lang).toBe("en");
  });

  it("signs out and clears the session", async () => {
    const api = mockApi({
      ...signedIn(),
      "GET /jobs": { body: { data: [], next_cursor: null } },
      "POST /auth/logout": { status: 204 },
    });
    const { router } = renderApp("/jobs");
    await userEvent.click(await screen.findByRole("button", { name: "Sign out" }));
    expect(await screen.findByRole("heading", { name: "Sign in to RoleScout" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/login");
    expect(api.calls.some((c) => c.method === "POST" && c.path === "/auth/logout")).toBe(true);
  });

  it("shows a not found page for unknown routes", async () => {
    mockApi(signedIn());
    renderApp("/nope");
    expect(await screen.findByRole("heading", { name: "Not found" })).toBeInTheDocument();
  });

  it("offers a skip link and labelled navigation", async () => {
    mockApi({ ...signedIn(), "GET /jobs": { body: { data: [], next_cursor: null } } });
    renderApp("/jobs");
    expect(await screen.findByRole("link", { name: "Skip to content" })).toHaveAttribute("href", "#main");
    const nav = screen.getByRole("navigation", { name: "Main navigation" });
    expect(nav).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Jobs" })).toHaveAttribute("aria-current", "page");
  });
});
