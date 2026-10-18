export function seriesLabel(label: string, values: number[]) {
  return values.length ? `${label}: ${values.join(", ")}` : label;
}
