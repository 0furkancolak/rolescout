// Application form autofill. Career pages live on other sites, so RoleScout
// cannot reach into them; instead it hands the user a bookmarklet that fills
// the common fields of the page they are on from the details saved in
// Settings. It never submits a form, never touches files, passwords or fields
// that already have a value.

export const autofillFields = [
  "first_name",
  "last_name",
  "email",
  "phone",
  "city",
  "country",
  "linkedin",
  "github",
  "website",
  "current_company",
  "current_title",
] as const;

export type AutofillField = (typeof autofillFields)[number];
export type AutofillProfile = Partial<Record<AutofillField, string>>;

// fillForms is serialised into the bookmarklet, so it must not refer to
// anything outside its own body.
export function fillForms(profile: AutofillProfile, doc: Document = document): number {
  const p = profile;
  const full = [p.first_name, p.last_name].filter(Boolean).join(" ");
  const rules: Array<[string, RegExp, string | undefined]> = [
    ["email", /e-?mail|e-?posta/, p.email],
    ["phone", /phone|mobile|telefon|gsm|\btel\b|cell/, p.phone],
    ["first", /first.?name|given.?name|fname|forename|^ad[ıi]?$|\bad[ıi]n[ıi]z\b/, p.first_name],
    ["last", /last.?name|family.?name|surname|lname|soyad/, p.last_name],
    ["linkedin", /linkedin/, p.linkedin],
    ["github", /github/, p.github],
    ["website", /website|web.?site|portfolio|personal.?(site|url|page)|homepage/, p.website],
    ["company", /current.?(company|employer)|present.?(company|employer)|\bemployer\b|mevcut.?(şirket|sirket)/, p.current_company],
    ["title", /current.?(title|role|position)|job.?title|present.?(title|position)|unvan/, p.current_title],
    ["country", /country|ülke|ulke/, p.country],
    ["city", /\bcity\b|location|şehir|sehir|based in|where.*(live|based)/, p.city],
    ["name", /full.?name|your.?name|^name$|candidate.?name|ad.?soyad|isim/, full],
  ];
  const fold = (s: string) => s.toLowerCase().replace(/\s+/g, " ").trim();
  const labelOf = (el: Element): string => {
    const id = el.getAttribute("id");
    const own = id ? doc.querySelector(`label[for="${CSS.escape(id)}"]`) : null;
    const wrap = el.closest("label");
    const by = (el.getAttribute("aria-labelledby") || "")
      .split(" ")
      .map((i) => doc.getElementById(i)?.textContent || "")
      .join(" ");
    return [own?.textContent, wrap?.textContent, by].filter(Boolean).join(" ");
  };
  let filled = 0;
  const fields = doc.querySelectorAll<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>("input, textarea, select");
  fields.forEach((el) => {
    const type = (el.getAttribute("type") || "text").toLowerCase();
    if (["hidden", "file", "password", "checkbox", "radio", "submit", "button", "image", "reset"].includes(type)) return;
    if (el.disabled || (el as HTMLInputElement).readOnly || el.value) return;
    const auto = fold(el.getAttribute("autocomplete") || "");
    const hint = fold(
      [auto, el.getAttribute("name"), el.getAttribute("id"), el.getAttribute("placeholder"), el.getAttribute("aria-label"), labelOf(el)]
        .filter(Boolean)
        .join(" "),
    );
    let value: string | undefined;
    if (type === "email" || auto === "email") value = p.email;
    else if (type === "tel" || auto === "tel") value = p.phone;
    else if (auto === "given-name") value = p.first_name;
    else if (auto === "family-name") value = p.last_name;
    else if (auto === "name") value = full;
    else {
      const rule = rules.find(([, re]) => re.test(hint));
      value = rule?.[2];
    }
    if (!value) return;
    if (el.tagName === "SELECT") {
      const select = el as HTMLSelectElement;
      const option = [...select.options].find((o) => fold(o.text) === fold(value!) || fold(o.value) === fold(value!));
      if (!option) return;
      select.value = option.value;
    } else {
      const proto = el.tagName === "TEXTAREA" ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
      // React-style forms track values through the native setter.
      Object.getOwnPropertyDescriptor(proto, "value")?.set?.call(el, value);
    }
    el.dispatchEvent(new Event("input", { bubbles: true }));
    el.dispatchEvent(new Event("change", { bubbles: true }));
    filled++;
  });
  const note = doc.createElement("div");
  note.textContent = `RoleScout: ${filled}`;
  note.setAttribute("style", "position:fixed;z-index:2147483647;right:16px;bottom:16px;padding:10px 14px;border-radius:10px;background:#17171a;color:#fffdf9;font:14px system-ui");
  doc.body.appendChild(note);
  setTimeout(() => note.remove(), 3500);
  return filled;
}

// bookmarklet returns a javascript: URL that fills the page with the profile.
export function bookmarklet(profile: AutofillProfile): string {
  const clean = Object.fromEntries(Object.entries(profile).filter(([, v]) => typeof v === "string" && v.trim()));
  return `javascript:${encodeURIComponent(`(${fillForms.toString()})(${JSON.stringify(clean)});void 0`)}`;
}
