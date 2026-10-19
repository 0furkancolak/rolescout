import { describe, expect, it } from "vitest";
import { bookmarklet, fillForms } from "../autofill";

const profile = {
  first_name: "Ada",
  last_name: "Lovelace",
  email: "ada@example.com",
  phone: "+90 555 000 00 00",
  linkedin: "https://linkedin.com/in/ada",
  city: "İstanbul",
  country: "Türkiye",
};

function page(html: string) {
  document.body.innerHTML = html;
  return fillForms(profile, document);
}

describe("fillForms", () => {
  it("fills fields by name, placeholder, label and autocomplete", () => {
    const n = page(`
      <form>
        <input name="first_name">
        <input id="ln" placeholder="Last name">
        <label for="mail">Your email address</label><input id="mail">
        <input autocomplete="tel">
        <label>LinkedIn profile <input></label>
        <input name="city">
        <select name="country"><option value="">-</option><option value="tr">Türkiye</option></select>
      </form>`);
    expect(n).toBe(7);
    const values = [...document.querySelectorAll("input, select")].map((e) => (e as HTMLInputElement).value);
    expect(values).toEqual(["Ada", "Lovelace", "ada@example.com", "+90 555 000 00 00", "https://linkedin.com/in/ada", "İstanbul", "tr"]);
  });

  it("leaves filled, hidden, file and password fields alone and fills a full name", () => {
    const n = page(`
      <input name="email" value="keep@example.com">
      <input type="hidden" name="email2">
      <input type="file" name="resume">
      <input type="password" name="email3">
      <input name="full_name">`);
    expect(n).toBe(1);
    expect((document.querySelector('[name="email"]') as HTMLInputElement).value).toBe("keep@example.com");
    expect((document.querySelector('[name="full_name"]') as HTMLInputElement).value).toBe("Ada Lovelace");
  });

  it("tells React-style forms about the change", () => {
    document.body.innerHTML = `<input name="email">`;
    const input = document.querySelector("input")!;
    let seen = 0;
    input.addEventListener("input", () => seen++);
    fillForms(profile, document);
    expect(seen).toBe(1);
  });

  it("builds a self-contained bookmarklet", () => {
    const url = bookmarklet({ email: "ada@example.com", phone: " " });
    expect(url.startsWith("javascript:")).toBe(true);
    const code = decodeURIComponent(url.slice("javascript:".length));
    expect(code).toContain("ada@example.com");
    expect(code.endsWith('({"email":"ada@example.com"});void 0')).toBe(true);
    document.body.innerHTML = `<input name="email">`;
    new Function(code)();
    expect(document.querySelector("input")!.value).toBe("ada@example.com");
  });
});
