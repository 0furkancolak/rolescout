export function redirectTarget(state: unknown): string {
  const from = (state as { from?: unknown } | null)?.from;
  // Only follow in-app paths so the login page cannot be used as an open redirect.
  return typeof from === "string" && from.startsWith("/") && !from.startsWith("//") ? from : "/jobs";
}
