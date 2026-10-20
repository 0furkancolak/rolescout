// secondsBetween is the whole seconds from start to end (or now), never negative.
export function secondsBetween(start: string | null, end: string | null, now = Date.now()): number | null {
  if (!start) return null;
  const to = end ? Date.parse(end) : now;
  return Math.max(0, Math.round((to - Date.parse(start)) / 1000));
}
