import { describe, expect, it } from "vitest";
import { contrastRatio } from "../contrast";

describe("contrastRatio", () => {
  it("is 21 for black on white and 1 for identical colours", () => {
    expect(contrastRatio("#000000", "#ffffff")).toBeCloseTo(21, 5);
    expect(contrastRatio("#777777", "#777777")).toBeCloseTo(1, 5);
  });

  it("accepts short hex and is symmetric", () => {
    expect(contrastRatio("#fff", "#000")).toBeCloseTo(21, 5);
    expect(contrastRatio("#123456", "#fedcba")).toBeCloseTo(contrastRatio("#fedcba", "#123456"), 10);
  });

  it("rejects colours below a threshold (the guard is real)", () => {
    expect(contrastRatio("#999999", "#ffffff")).toBeLessThan(4.5);
  });

  it("throws on invalid colours", () => {
    expect(() => contrastRatio("red", "#fff")).toThrow();
  });
});
