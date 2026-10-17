import { describe, expect, it } from "vitest";
import { readJobSearch, writeJobSearch } from "../searchParams";

describe("job search URL state", () => {
  it("round-trips every field", () => {
    const params = new URLSearchParams(
      "q=react&profile=p1&sort=score&posted=7d&arrangement=remote&seniority=senior&technology=react&technology=go&country=TR&company=c1&closed=true&job=j1",
    );
    const s = readJobSearch(params);
    expect(s).toEqual({
      q: "react", profile: "p1", sort: "score", posted: "7d", arrangement: "remote", seniority: "senior",
      technologies: ["react", "go"], countries: ["TR"], companies: ["c1"], closed: true, job: "j1",
    });
    expect(writeJobSearch(s).toString()).toBe(params.toString());
  });

  it("drops empty values and unknown enum values", () => {
    const s = readJobSearch(new URLSearchParams("q=&sort=nonsense&posted=1y&closed=yes"));
    expect(s).toEqual({ q: "", profile: "", sort: "", posted: "", arrangement: "", seniority: "", technologies: [], countries: [], companies: [], closed: false, job: "" });
    expect(writeJobSearch(s).toString()).toBe("");
  });
});
