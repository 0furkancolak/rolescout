import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { mockApi, renderApp, signedIn, user, WS } from "../../test/utils";

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
  document.documentElement.classList.remove("dark");
});

const dashboardData = {
  [`GET /workspaces/${WS}/research-profiles`]: { body: { data: [] } },
  [`GET /workspaces/${WS}/research-runs`]: { body: { data: [] } },
  [`GET /workspaces/${WS}/applications`]: { body: { data: [] } },
  [`GET /workspaces/${WS}/jobs`]: { body: { data: [], next_cursor: null } },
};

describe("App shell", () => {
  it("shows the settings link only to the administrator", async () => {
    mockApi({ ...signedIn(), ...dashboardData });
    const { unmount } = renderApp("/");
    const nav = await screen.findByRole("navigation", { name: "Main navigation" });
    for (const name of ["Dashboard", "Research", "Jobs", "Saved", "Applications", "Companies"]) {
      expect(within(nav).getByRole("link", { name })).toBeInTheDocument();
    }
    expect(within(nav).queryByRole("link", { name: "Settings" })).not.toBeInTheDocument();
    unmount();

    mockApi({ ...signedIn(), "GET /auth/me": { body: { data: { ...user, is_admin: true } } }, ...dashboardData });
    renderApp("/");
    const adminNav = await screen.findByRole("navigation", { name: "Main navigation" });
    expect(within(adminNav).getByRole("link", { name: "Settings" })).toBeInTheDocument();
  });

  it("switches between light, dark and system themes and remembers the choice", async () => {
    mockApi({ ...signedIn(), ...dashboardData });
    renderApp("/");
    await userEvent.click(await screen.findByRole("button", { name: /Account menu/ }));
    const toggle = await screen.findByRole("button", { name: /Theme/ });
    await userEvent.click(toggle); // system -> light
    expect(document.documentElement.classList.contains("dark")).toBe(false);
    await userEvent.click(toggle); // light -> dark
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(localStorage.getItem("rolescout.theme")).toBe("dark");
    await userEvent.click(toggle); // dark -> system
    expect(localStorage.getItem("rolescout.theme")).toBe("system");
  });

  it("opens and closes the mobile menu", async () => {
    mockApi({ ...signedIn(), ...dashboardData });
    renderApp("/");
    const button = await screen.findByRole("button", { name: "Open menu" });
    expect(button).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(button);
    expect(button).toHaveAttribute("aria-expanded", "true");
    await userEvent.keyboard("{Escape}");
    expect(button).toHaveAttribute("aria-expanded", "false");
  });

  it("marks the current page in the navigation", async () => {
    mockApi({ ...signedIn(), ...dashboardData });
    renderApp("/");
    const nav = await screen.findByRole("navigation", { name: "Main navigation" });
    expect(within(nav).getByRole("link", { name: "Dashboard" })).toHaveAttribute("aria-current", "page");
    expect(within(nav).getByRole("link", { name: "Jobs" })).not.toHaveAttribute("aria-current");
  });

  it("returns focus to the menu button when the mobile menu closes", async () => {
    mockApi({ ...signedIn(), ...dashboardData });
    renderApp("/");
    const button = await screen.findByRole("button", { name: "Open menu" });
    await userEvent.click(button);
    await userEvent.keyboard("{Escape}");
    expect(button).toHaveFocus();
  });

  it("makes the skip link the first focusable element", async () => {
    mockApi({ ...signedIn(), ...dashboardData });
    renderApp("/");
    await screen.findByRole("navigation", { name: "Main navigation" });
    await userEvent.tab();
    expect(screen.getByRole("link", { name: "Skip to content" })).toHaveFocus();
  });

  it("keeps language, theme and sign out in an account menu that closes with Escape", async () => {
    mockApi({ ...signedIn(), ...dashboardData });
    renderApp("/");
    const trigger = await screen.findByRole("button", { name: /Account menu/ });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("combobox", { name: "Language" })).toBeNull();
    await userEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("combobox", { name: "Language" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Theme/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sign out" })).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
  });

  it("redirects old job detail links to the job browser", async () => {
    mockApi({ ...signedIn(), ...dashboardData,
      [`GET /workspaces/${WS}/jobs/facets`]: { body: { data: { posted: {}, arrangement: {}, seniority: {}, country: {}, technologies: [], companies: [] } } },
    });
    const { router } = renderApp("/jobs/33333333-3333-4333-8333-000000000001");
    await vi.waitFor(() => expect(router.state.location.pathname).toBe("/jobs"));
    expect(router.state.location.search).toBe("?job=33333333-3333-4333-8333-000000000001");
  });
});
