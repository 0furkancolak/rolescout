import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { createI18n, I18nContext } from "../../../lib/i18n";
import { FacetPanel } from "../FacetPanel";
import { readJobSearch } from "../searchParams";

function setup() {
  const onChange = vi.fn();
  const i18n = createI18n("en", () => {});
  render(
    <I18nContext.Provider value={i18n}>
      <FacetPanel search={readJobSearch(new URLSearchParams())} onChange={onChange} />
    </I18nContext.Provider>,
  );
  return onChange;
}

describe("FacetPanel place filters", () => {
  it("searches a place when the form is submitted", async () => {
    const onChange = setup();
    await userEvent.type(screen.getByRole("searchbox", { name: "City or place" }), "İzmir{Enter}");
    expect(onChange).toHaveBeenCalledWith({ location: "İzmir" });
  });

  it("picks a region and a job board system", async () => {
    const onChange = setup();
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Region" }), "emea");
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Job board system" }), "lever");
    expect(onChange).toHaveBeenCalledWith({ region: "emea" });
    expect(onChange).toHaveBeenCalledWith({ ats: "lever" });
  });
});
