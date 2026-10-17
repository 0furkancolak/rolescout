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
    // The mobile header and the sidebar both have a toggle; they share state.
    const [toggle] = await screen.findAllByRole("button", { name: /Theme/ });
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

  it("redirects old job detail links to the job browser", async () => {
    mockApi({ ...signedIn(), ...dashboardData,
      [`GET /workspaces/${WS}/jobs/facets`]: { body: { data: { posted: {}, arrangement: {}, seniority: {}, country: {}, technologies: [], companies: [] } } },
    });
    const { router } = renderApp("/jobs/33333333-3333-4333-8333-000000000001");
    await vi.waitFor(() => expect(router.state.location.pathname).toBe("/jobs"));
    expect(router.state.location.search).toBe("?job=33333333-3333-4333-8333-000000000001");
  });
});
