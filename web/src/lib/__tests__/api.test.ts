import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, data, query, request, statusTransitions } from "../api";
import { messages } from "../messages";

afterEach(() => vi.unstubAllGlobals());

function stubFetch(impl: () => Promise<Response>) {
  const fn = vi.fn(impl);
  vi.stubGlobal("fetch", fn);
  return fn;
}

describe("request", () => {
  it("unwraps data and sends JSON with same-origin credentials", async () => {
    const fetchMock = stubFetch(async () => new Response(JSON.stringify({ data: { id: "1" } }), { status: 201 }));
    await expect(data("/tags", { method: "POST", body: { name: "go" } })).resolves.toEqual({ id: "1" });
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/v1/tags");
    expect(init.credentials).toBe("same-origin");
    expect(init.body).toBe('{"name":"go"}');
    expect((init.headers as Record<string, string>)["Content-Type"]).toBe("application/json");
  });

  it("returns undefined for 204", async () => {
    stubFetch(async () => new Response(null, { status: 204 }));
    await expect(request("/saved-jobs/x", { method: "PUT" })).resolves.toBeUndefined();
  });

  it("turns error bodies into ApiError with code and fields", async () => {
    stubFetch(
      async () =>
        new Response(
          JSON.stringify({ error: { code: "invalid_input", message: "bad", fields: [{ field: "q", message: "too long" }] } }),
          { status: 422 },
        ),
    );
    const err = await request("/jobs").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 422, code: "invalid_input", fields: [{ field: "q", message: "too long" }] });
  });

  it("reports unreachable servers as network errors", async () => {
    stubFetch(async () => {
      throw new TypeError("Failed to fetch");
    });
    await expect(request("/jobs")).rejects.toMatchObject({ code: "network", status: 0 });
  });

  it("handles non-JSON error responses", async () => {
    stubFetch(async () => new Response("<html>bad gateway</html>", { status: 502, statusText: "Bad Gateway" }));
    await expect(request("/jobs")).rejects.toMatchObject({ code: "internal", status: 502 });
  });
});

describe("query", () => {
  it("drops empty values and encodes the rest", () => {
    expect(query({ q: "c++ & go", seniority: "", cursor: undefined })).toBe("?q=c%2B%2B+%26+go");
    expect(query({})).toBe("");
  });
});

describe("statusTransitions", () => {
  it("never offers a transition to the same status", () => {
    for (const [from, tos] of Object.entries(statusTransitions)) expect(tos).not.toContain(from);
  });
});

describe("messages", () => {
  it("has a non-empty translation for every key in both languages", () => {
    for (const locale of ["tr", "en"] as const) {
      for (const [key, value] of Object.entries(messages[locale])) {
        expect(value.trim(), `${locale} ${key}`).not.toBe("");
      }
    }
    expect(Object.keys(messages.tr).sort()).toEqual(Object.keys(messages.en).sort());
  });
});
