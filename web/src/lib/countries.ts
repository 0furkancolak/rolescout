// Country names for searching and for profile countries: "Türkiye", "Turkey",
// "Almanya" and "DE" all resolve to an ISO 3166-1 alpha-2 code.

const fold = (s: string) =>
  s
    .replace(/İ/g, "i")
    .toLowerCase()
    .replace(/ı/g, "i")
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .trim();

// Codes the locale data names but that are not countries.
const notCountries = new Set(["EU", "UN", "EZ", "UK", "QO", "XA", "XB", "ZZ", "AC", "CP", "DG", "EA", "IC", "TA"]);

let names: Map<string, string> | undefined;

function index(): Map<string, string> {
  if (names) return names;
  names = new Map();
  const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";
  for (const locale of ["en", "tr"]) {
    let display: Intl.DisplayNames;
    try {
      display = new Intl.DisplayNames([locale], { type: "region" });
    } catch {
      continue;
    }
    for (const a of letters) {
      for (const b of letters) {
        const code = a + b;
        const name = display.of(code);
        if (name && name !== code && !notCountries.has(code)) names.set(fold(name), code);
      }
    }
  }
  // Names people type that the locale data spells differently.
  for (const [alias, code] of [["turkey", "TR"], ["turkiye", "TR"], ["uk", "GB"], ["england", "GB"], ["usa", "US"], ["uae", "AE"]] as const) {
    names.set(alias, code);
  }
  return names;
}

// resolveCountry returns the ISO code for a code or a country name in English
// or Turkish, or "" when it is neither.
export function resolveCountry(input: string): string {
  const s = input.trim();
  if (/^[a-z]{2}$/i.test(s)) return s.toUpperCase();
  return index().get(fold(s)) ?? "";
}

// countryName is the name of a country code in the given locale.
export function countryName(code: string, locale: string): string {
  try {
    return new Intl.DisplayNames([locale || "en"], { type: "region" }).of(code) ?? code;
  } catch {
    return code;
  }
}
