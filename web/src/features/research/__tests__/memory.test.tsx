import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { makeProfile, mockApi, renderApp, signedIn, WS } from "../../../test/utils";

afterEach(() => vi.unstubAllGlobals());

const base = `/workspaces/${WS}`;

const withCV = {
  about: "Backend jobs in Istanbul",
  ai_read: true,
  cv: {
    filename: "ayse-cv.pdf",
    uploaded_at: "2026-10-20T10:00:00Z",
    chars: 2400,
    skills: ["go", "postgresql", "docker"],
    titles: ["Backend Developer"],
    seniority: "senior",
    years: 6,
    summary: "Backend developer with six years of Go.",
  },
};

describe("MemoryCard", () => {
  it("uploads a CV and shows what was read from it", async () => {
    let stored: unknown = { about: "", ai_read: false, cv: null };
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [makeProfile()] } },
      [`GET ${base}/research-runs`]: { body: { data: [] } },
      [`GET ${base}/context`]: () => ({ body: { data: stored } }),
      [`POST ${base}/context/cv`]: () => {
        stored = withCV;
        return { body: { data: withCV } };
      },
    });
    renderApp("/about");

    const card = (await screen.findByRole("heading", { name: "What the AI knows about you" })).closest("section, div")!.parentElement!;
    expect(await within(card).findByText("PDF, Word (.docx) or text, up to 5 MB")).toBeInTheDocument();

    const file = new File(["%PDF-1.4 fake"], "ayse-cv.pdf", { type: "application/pdf" });
    await userEvent.upload(screen.getByLabelText("Choose file", { selector: "input" }), file);

    expect(await screen.findByText("ayse-cv.pdf")).toBeInTheDocument();
    expect(screen.getByText("Backend developer with six years of Go.")).toBeInTheDocument();
    expect(screen.getByText("postgresql")).toBeInTheDocument();
    expect(screen.getByText("Senior · 6 years")).toBeInTheDocument();
    expect(api.calls.some((c) => c.method === "POST" && c.path === `${base}/context/cv` && (c.body as { file: string }).file === "ayse-cv.pdf")).toBe(true);
  });

  it("saves the note and deletes the CV", async () => {
    let stored: unknown = withCV;
    const api = mockApi({
      ...signedIn(),
      [`GET ${base}/research-profiles`]: { body: { data: [makeProfile()] } },
      [`GET ${base}/research-runs`]: { body: { data: [] } },
      [`GET ${base}/context`]: () => ({ body: { data: stored } }),
      [`PUT ${base}/context`]: (call) => {
        stored = { ...(stored as object), about: (call.body as { about: string }).about };
        return { body: { data: stored } };
      },
      [`DELETE ${base}/context/cv`]: () => {
        stored = { about: (stored as { about: string }).about, ai_read: false, cv: null };
        return { status: 204 };
      },
    });
    vi.spyOn(window, "confirm").mockReturnValue(true);
    renderApp("/about");

    const note = await screen.findByLabelText("The work you look for");
    expect(note).toHaveValue("Backend jobs in Istanbul");
    expect(screen.getByRole("button", { name: "Save note" })).toBeDisabled();
    await userEvent.type(note, " or remote");
    await userEvent.click(screen.getByRole("button", { name: "Save note" }));
    expect(await screen.findByText("Note saved")).toBeInTheDocument();
    expect(api.calls.find((c) => c.method === "PUT")?.body).toEqual({ about: "Backend jobs in Istanbul or remote" });

    await userEvent.click(screen.getByRole("button", { name: "Delete CV" }));
    expect(await screen.findByText("Drop your CV here or choose a file")).toBeInTheDocument();
  });
});
