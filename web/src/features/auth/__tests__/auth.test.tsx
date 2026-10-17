import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { mockApi, renderApp, user, workspace } from "../../../test/utils";
import { redirectTarget } from "../redirect";

afterEach(() => vi.unstubAllGlobals());

const signedOut = { "GET /auth/me": { status: 401, body: { error: { code: "unauthenticated", message: "" } } } };

describe("LoginPage", () => {
  it("validates required fields without calling the API", async () => {
    const api = mockApi(signedOut);
    renderApp("/login");
    await userEvent.click(await screen.findByRole("button", { name: "Sign in" }));
    expect(await screen.findAllByText("This field is required.")).toHaveLength(2);
    expect(screen.getByLabelText("Email")).toHaveAttribute("aria-invalid", "true");
    expect(api.calls.some((c) => c.path === "/auth/login")).toBe(false);
  });

  it("shows a generic message for wrong credentials", async () => {
    mockApi({
      ...signedOut,
      "POST /auth/login": { status: 401, body: { error: { code: "invalid_credentials", message: "" } } },
    });
    renderApp("/login");
    await userEvent.type(await screen.findByLabelText("Email"), "ada@example.com");
    await userEvent.type(screen.getByLabelText("Password"), "wrong password!");
    await userEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("The email or password is incorrect.");
  });

  it("only redirects to in-app paths", () => {
    expect(redirectTarget({ from: "/saved" })).toBe("/saved");
    expect(redirectTarget({ from: "//evil.example" })).toBe("/");
    expect(redirectTarget({ from: "https://evil.example" })).toBe("/");
    expect(redirectTarget(null)).toBe("/");
  });
});

describe("RegisterPage", () => {
  it("checks the password length like the server", async () => {
    const api = mockApi(signedOut);
    renderApp("/register");
    await userEvent.type(await screen.findByLabelText("Name"), "Ada");
    await userEvent.type(screen.getByLabelText("Email"), "ada@example.com");
    await userEvent.type(screen.getByLabelText("Password"), "short");
    await userEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect(await screen.findByText("The password must be at least 12 characters.")).toBeInTheDocument();
    expect(api.calls.some((c) => c.path === "/auth/register")).toBe(false);
  });

  it("explains when registration is closed", async () => {
    mockApi({
      ...signedOut,
      "POST /auth/register": { status: 403, body: { error: { code: "registration_closed", message: "" } } },
    });
    renderApp("/register");
    await userEvent.type(await screen.findByLabelText("Name"), "Ada");
    await userEvent.type(screen.getByLabelText("Email"), "ada@example.com");
    await userEvent.type(screen.getByLabelText("Password"), "a long enough password");
    await userEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Registration is disabled on this server.");
  });

  it("registers and opens the dashboard", async () => {
    const ws = `/workspaces/${workspace.id}`;
    const api = mockApi({
      ...signedOut,
      "POST /auth/register": { status: 201, body: { data: user } },
      "GET /workspaces": { body: { data: [workspace] } },
      [`GET ${ws}/research-profiles`]: { body: { data: [] } },
      [`GET ${ws}/research-runs`]: { body: { data: [] } },
      [`GET ${ws}/applications`]: { body: { data: [] } },
    });
    renderApp("/register");
    await userEvent.type(await screen.findByLabelText("Name"), " Ada ");
    await userEvent.type(screen.getByLabelText("Email"), "ada@example.com");
    await userEvent.type(screen.getByLabelText("Password"), "a long enough password");
    await userEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect(await screen.findByRole("heading", { name: "Dashboard" })).toBeInTheDocument();
    expect(api.calls.find((c) => c.path === "/auth/register")?.body).toEqual({
      display_name: "Ada",
      email: "ada@example.com",
      password: "a long enough password",
      locale: "en",
    });
  });
});
