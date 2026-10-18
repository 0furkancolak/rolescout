/// <reference types="node" />
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { contrastRatio } from "../contrast";
import { tokens, type TokenName } from "../tokens";

const themes = ["light", "dark"] as const;

describe.each(themes)("%s tokens", (theme) => {
  const t = tokens[theme];
  const text: Array<[TokenName, TokenName]> = [
    ["ink", "canvas"],
    ["ink", "surface"],
    ["ink", "raised"],
    ["ink-muted", "surface"],
    ["ink-muted", "canvas"],
    ["sage-deep", "surface"],
    ["amber-deep", "surface"],
    ["lavender-deep", "surface"],
    ["danger", "surface"],
    ["ink", "sage"],
    ["ink", "amber"],
    ["ink", "lavender"],
    ["ink-inverse", "ink"],
    ["well-ink", "well"],
    ["well-muted", "well"],
    ["well-sage", "well"],
    ["well-amber", "well"],
    ["well-danger", "well"],
  ];
  it.each(text)("%s on %s has at least 4.5:1", (fg, bg) => {
    expect(contrastRatio(t[fg], t[bg])).toBeGreaterThanOrEqual(4.5);
  });

  const boundaries: Array<[TokenName, TokenName]> = [
    ["line-strong", "canvas"],
    ["line-strong", "surface"],
    ["line-strong", "raised"],
    ["focus", "surface"],
    ["focus", "canvas"],
  ];
  it.each(boundaries)("%s on %s has at least 3:1", (fg, bg) => {
    expect(contrastRatio(t[fg], t[bg])).toBeGreaterThanOrEqual(3);
  });
});

describe.each(themes)("%s decorative line", (theme) => {
  it("is visible but quiet against the canvas", () => {
    const r = contrastRatio(tokens[theme].line, tokens[theme].canvas);
    expect(r).toBeGreaterThan(1.1);
    expect(r).toBeLessThan(3);
  });
});

describe("CSS parity", () => {
  const css = readFileSync("src/index.css", "utf8");

  function block(theme: string): string {
    const start = css.indexOf(`/* tokens:${theme} */`);
    const end = css.indexOf("/* tokens:end */", start);
    expect(start, `tokens:${theme} marker`).toBeGreaterThanOrEqual(0);
    return css.slice(start, end);
  }

  it.each(themes)("the %s block repeats the TypeScript constants", (theme) => {
    const body = block(theme);
    for (const [name, value] of Object.entries(tokens[theme])) {
      expect(body.toLowerCase(), `--color-${name}`).toContain(`--color-${name}: ${value.toLowerCase()};`);
    }
  });
});
