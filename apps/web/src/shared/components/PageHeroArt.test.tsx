import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PageHeroArt } from "./PageHeroArt";

describe("PageHeroArt", () => {
  it.each(["water", "map", "risk", "announcement", "report"] as const)("renders decorative tiles for %s without accessible controls", (kind) => {
    const { container } = render(<PageHeroArt kind={kind} />);
    expect(container.querySelector('.page-hero__art')).toHaveAttribute("aria-hidden", "true");
    expect(container.querySelectorAll('.page-hero__tile')).toHaveLength(3);
    expect(container.querySelectorAll('svg')).toHaveLength(3);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });
});
