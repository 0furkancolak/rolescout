import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { tintFor } from "../../lib/tint";
import { CompanyTile } from "../CompanyTile";
import { MiniBars } from "../MiniBars";
import { Pill } from "../Pill";
import { ScoreRing } from "../ScoreRing";
import { Segmented } from "../Segmented";
import { Sparkline } from "../Sparkline";
import { StatCard } from "../StatCard";

describe("tintFor", () => {
  it("is stable and differs between names", () => {
    expect(tintFor("Stripe")).toEqual(tintFor("Stripe"));
    const tints = new Set(["Stripe", "Linear", "Vercel", "Getir", "Trendyol", "Notion", "Figma", "Ramp"].map((n) => tintFor(n).bg));
    expect(tints.size).toBeGreaterThan(3);
  });
});

describe("CompanyTile", () => {
  it.each([
    ["Ömer Labs", "Ö"],
    ["42 Labs", "4"],
    ["stripe", "S"],
  ])("shows the initial of %s", (name, initial) => {
    const { container } = render(<CompanyTile name={name} />);
    expect(container.textContent).toBe(initial);
    expect(container.firstElementChild).toHaveAttribute("aria-hidden", "true");
  });

  it("falls back to a neutral glyph for an empty name", () => {
    const { container } = render(<CompanyTile name="  " />);
    expect(container.textContent).toBe("•");
  });
});

describe("ScoreRing", () => {
  it("shows the number and a translated accessible name", () => {
    render(<ScoreRing value={82} />);
    expect(screen.getByRole("img", { name: "Match score 82 of 100" })).toBeInTheDocument();
    expect(screen.getByText("82")).toBeInTheDocument();
  });

  it("clamps out-of-range values", () => {
    const { rerender } = render(<ScoreRing value={-5} />);
    expect(screen.getByText("0")).toBeInTheDocument();
    rerender(<ScoreRing value={120} />);
    expect(screen.getByText("100")).toBeInTheDocument();
  });
});

describe("Sparkline and MiniBars", () => {
  it.each([[[]], [[3]], [[1, 2, 3]]])("render %j with an accessible name", (values) => {
    const { unmount } = render(<Sparkline values={values} label="New matches" />);
    expect(screen.getByRole("img", { name: /New matches/ })).toBeInTheDocument();
    unmount();
    render(<MiniBars values={values} label="By stage" />);
    expect(screen.getByRole("img", { name: /By stage/ })).toBeInTheDocument();
  });

  it("lists the values in the label", () => {
    render(<Sparkline values={[1, 2, 3]} label="Matches" />);
    expect(screen.getByRole("img", { name: "Matches: 1, 2, 3" })).toBeInTheDocument();
  });
});

describe("Pill", () => {
  it("is plain text when not interactive", () => {
    render(<Pill>Remote</Pill>);
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("toggles aria-pressed as a button", async () => {
    const onClick = vi.fn();
    const { rerender } = render(<Pill selected={false} onClick={onClick}>Remote</Pill>);
    const btn = screen.getByRole("button", { name: "Remote" });
    expect(btn).toHaveAttribute("aria-pressed", "false");
    await userEvent.click(btn);
    expect(onClick).toHaveBeenCalledTimes(1);
    rerender(<Pill selected onClick={onClick}>Remote</Pill>);
    expect(screen.getByRole("button", { name: "Remote" })).toHaveAttribute("aria-pressed", "true");
  });
});

describe("Segmented", () => {
  function Harness({ onChange }: { onChange: (v: string) => void }) {
    return (
      <>
        <Segmented
          label="Sort"
          value="score"
          onChange={onChange}
          options={[
            { value: "score", label: "Best match" },
            { value: "recent", label: "Newest" },
            { value: "relevance", label: "Relevance" },
          ]}
        />
        <button>after</button>
      </>
    );
  }

  it("is a radiogroup with the current value checked", () => {
    render(<Harness onChange={() => {}} />);
    expect(screen.getByRole("radiogroup", { name: "Sort" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Best match" })).toBeChecked();
  });

  it("moves with the arrow keys and Tab leaves the group", async () => {
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    screen.getByRole("radio", { name: "Best match" }).focus();
    await userEvent.keyboard("{ArrowRight}");
    expect(onChange).toHaveBeenLastCalledWith("recent");
    await userEvent.keyboard("{ArrowLeft}");
    expect(onChange).toHaveBeenLastCalledWith("relevance");
    await userEvent.tab();
    expect(screen.getByRole("button", { name: "after" })).toHaveFocus();
  });
});

describe("StatCard", () => {
  it("shows the label and value and links when href is set", () => {
    render(
      <MemoryRouter>
        <StatCard label="New matches" value="12" href="/jobs" series={[1, 2, 3]} />
      </MemoryRouter>,
    );
    expect(screen.getByRole("link", { name: /New matches/ })).toHaveAttribute("href", "/jobs");
    expect(screen.getByText("12")).toHaveClass("tabular-nums");
    expect(screen.getByRole("img", { name: "New matches: 1, 2, 3" })).toBeInTheDocument();
  });

  it("is not a link without href", () => {
    render(<StatCard label="Open jobs" value="30,805" />);
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.getByText("Open jobs")).toBeInTheDocument();
  });
});
