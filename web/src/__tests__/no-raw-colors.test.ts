/// <reference types="node" />
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// Screens use the semantic design tokens (bg-surface, text-ink, ...), never a
// raw Tailwind palette colour or a hex literal, so both themes stay in step.

const roots = ["src/features", "src/components"];
const allowed: string[] = [];

const shaded = "slate|gray|zinc|neutral|stone|blue|red|green|emerald|amber|violet|sky|indigo|purple|pink|rose|orange|yellow|lime|teal|cyan|fuchsia";
const utility = "bg|text|border|ring|from|to|via|outline|divide|fill|stroke|accent|decoration|placeholder|caret|shadow";
const rawUtility = new RegExp(`(?<![\\w-])(?:[a-z0-9-]+:)*(?:${utility})-(?:white|black|(?:${shaded})-\\d{2,3})(?:/\\d+)?(?![\\w-])`, "g");
const hexLiteral = /#[0-9a-fA-F]{3,8}\b/g;

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return name === "__tests__" ? [] : sources(path);
    return /\.tsx?$/.test(name) ? [path] : [];
  });
}

describe("design tokens only", () => {
  const files = roots.flatMap(sources).filter((f) => !allowed.includes(f));

  it("scans the screens", () => {
    expect(files.length).toBeGreaterThan(20);
  });

  it("uses no raw palette utilities or hex colours", () => {
    const offenders: string[] = [];
    for (const f of files) {
      const text = readFileSync(f, "utf8");
      const hits = [...(text.match(rawUtility) ?? []), ...(text.match(hexLiteral) ?? [])];
      if (hits.length) offenders.push(`${f}: ${[...new Set(hits)].slice(0, 6).join(" ")}`);
    }
    expect(offenders, `files still using raw colours:\n${offenders.join("\n")}`).toEqual([]);
  });
});
