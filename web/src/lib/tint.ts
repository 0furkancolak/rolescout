// Deterministic soft colours for company tiles: the same name always maps to
// the same pair. Every pair is a token fill with ink text; mixes sit between
// fills whose contrast with ink is already tested.

export interface Tint {
  bg: string;
  fg: string;
}

const mix = (a: string, b: string) => `color-mix(in srgb, var(--color-${a}) 55%, var(--color-${b}))`;

const tints: Tint[] = [
  "var(--color-sage)",
  "var(--color-amber)",
  "var(--color-lavender)",
  mix("sage", "amber"),
  mix("amber", "lavender"),
  mix("lavender", "sage"),
  "var(--color-danger-soft)",
  "var(--color-line)",
].map((bg) => ({ bg, fg: "var(--color-ink)" }));

export function tintFor(name: string): Tint {
  let h = 0x811c9dc5;
  for (const ch of name.trim().toLowerCase()) {
    h ^= ch.codePointAt(0) ?? 0;
    h = Math.imul(h, 0x01000193) >>> 0;
  }
  return tints[h % tints.length];
}
