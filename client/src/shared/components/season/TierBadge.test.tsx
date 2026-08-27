import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { SEASON_TIER_COLOR, SEASON_TIERS } from "@/shared/lib/seasonTier";

describe("TierBadge", () => {
  it("marks itself with the tier token so colour can be asserted", () => {
    render(<TierBadge tier="gold" />);
    expect(screen.getByTestId("tier-badge").getAttribute("data-tier")).toBe("gold");
  });

  it("is hidden from assistive tech, since the tier name is always rendered beside it", () => {
    render(<TierBadge tier="gold" />);
    expect(screen.getByTestId("tier-badge").getAttribute("aria-hidden")).toBe("true");
  });

  it("paints from the rank ramp variables rather than a hardcoded colour", () => {
    // The ramp is a runtime CSS value, so it must arrive through inline style —
    // a Tailwind class could not carry it, and a literal hex would not re-root
    // on the .game-table felt scope.
    render(<TierBadge tier="diamond" />);
    const style = screen.getByTestId("tier-badge").getAttribute("style") ?? "";
    expect(style).toContain(SEASON_TIER_COLOR.diamond);
    expect(style).toContain("box-shadow");
  });

  it("renders every tier in the ladder without falling off the colour map", () => {
    for (const tier of SEASON_TIERS) {
      const { unmount } = render(<TierBadge tier={tier} data-testid={`badge-${tier}`} />);
      const style = screen.getByTestId(`badge-${tier}`).getAttribute("style") ?? "";
      expect(style, `${tier} must resolve to a ramp colour`).toContain(SEASON_TIER_COLOR[tier]);
      unmount();
    }
  });

  it("scales down for a list row without changing the treatment", () => {
    const { rerender } = render(<TierBadge tier="silver" size="md" />);
    expect(screen.getByTestId("tier-badge").className).toContain("size-11");

    rerender(<TierBadge tier="silver" size="sm" />);
    expect(screen.getByTestId("tier-badge").className).toContain("size-7");
    expect(screen.getByTestId("tier-badge").getAttribute("data-tier")).toBe("silver");
  });

  it("takes a caller-supplied test id so each surface can name its own badge", () => {
    render(<TierBadge tier="iron" data-testid="rank-badge" />);
    expect(screen.getByTestId("rank-badge")).toBeTruthy();
  });
});
