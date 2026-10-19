import { describe, expect, it } from "vitest";
import { countryName, resolveCountry } from "../countries";

describe("resolveCountry", () => {
  it("accepts codes, English names and Turkish names", () => {
    expect(resolveCountry("tr")).toBe("TR");
    expect(resolveCountry("Türkiye")).toBe("TR");
    expect(resolveCountry("Turkey")).toBe("TR");
    expect(resolveCountry("Almanya")).toBe("DE");
    expect(resolveCountry("germany")).toBe("DE");
    expect(resolveCountry("Hollanda")).toBe("NL");
    expect(resolveCountry("United Kingdom")).toBe("GB");
  });
  it("rejects what is not a country", () => {
    expect(resolveCountry("Narnia")).toBe("");
    expect(resolveCountry("")).toBe("");
  });
});

describe("countryName", () => {
  it("names a country in the locale", () => {
    expect(countryName("DE", "tr")).toBe("Almanya");
    expect(countryName("DE", "en")).toBe("Germany");
  });
});
