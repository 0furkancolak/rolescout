import { tintFor } from "../lib/tint";

const sizes = { 32: "size-8 text-sm", 40: "size-10 text-base", 56: "size-14 text-xl" } as const;

// A rounded square with the company's initial. Decorative: the name is always
// rendered next to it.
export function CompanyTile({ name, size = 40 }: { name: string; size?: keyof typeof sizes }) {
  const initial = [...name.trim()].find((c) => /[\p{L}\p{N}]/u.test(c));
  const { bg, fg } = tintFor(name);
  return (
    <span
      aria-hidden="true"
      style={{ background: bg, color: fg }}
      className={`inline-flex shrink-0 select-none items-center justify-center rounded-control font-semibold ${sizes[size]}`}
    >
      {initial ? initial.toLocaleUpperCase() : "•"}
    </span>
  );
}
