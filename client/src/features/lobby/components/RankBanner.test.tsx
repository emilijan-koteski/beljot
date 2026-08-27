import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RankBanner } from "@/features/lobby/components/RankBanner";
import { i18n } from "@/shared/i18n/i18n";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";

// A fixed "now" so the days-remaining figure is deterministic.
const NOW = Date.parse("2026-08-27T12:00:00Z");

const baseSeason: CurrentSeasonResponse = {
  seasonName: "2026 Q3",
  endsAt: "2026-10-01T00:00:00Z",
  sp: 4000,
  rankTier: "gold",
  spIntoTier: 1000,
  spForNextTier: 2500,
  gamesPlayed: 31,
  gamesCompleted: 29,
};

describe("RankBanner", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });

  afterEach(async () => {
    vi.useRealTimers();
    await i18n.changeLanguage("en");
  });

  it("renders nothing while the season query is in flight", () => {
    render(<RankBanner season={undefined} />);
    expect(screen.queryByTestId("rank-banner")).toBeNull();
  });

  it("renders the tier badge", () => {
    render(<RankBanner season={baseSeason} />);
    expect(screen.getByTestId("rank-badge")).toBeTruthy();
  });

  it("marks the banner with the tier token so the colour can be asserted", () => {
    render(<RankBanner season={baseSeason} />);
    expect(screen.getByTestId("rank-banner").getAttribute("data-tier")).toBe("gold");
  });

  it("renders the tier name", () => {
    render(<RankBanner season={baseSeason} />);
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Gold");
  });

  it("renders the current SP total", () => {
    render(<RankBanner season={baseSeason} />);
    // Grouped via toLocaleString (the same call HeroBlock's stat pills use), so
    // the expectation is built the same way rather than hardcoding a separator
    // the CI host's locale may not use.
    expect(screen.getByTestId("rank-sp").textContent).toContain((4000).toLocaleString());
  });

  it("renders the progress bar with the server's own decomposition", () => {
    render(<RankBanner season={baseSeason} />);
    const bar = screen.getByTestId("rank-progress");
    expect(bar.getAttribute("role")).toBe("progressbar");
    expect(bar.getAttribute("aria-valuemin")).toBe("0");
    expect(bar.getAttribute("aria-valuemax")).toBe("100");
    // 1000 of a 2500-wide band.
    expect(bar.getAttribute("aria-valuenow")).toBe("40");
    expect(bar.getAttribute("aria-label")).toBeTruthy();
  });

  it("renders the days remaining in the season", () => {
    render(<RankBanner season={baseSeason} />);
    // 2026-08-27T12:00Z -> 2026-10-01T00:00Z is 34.5 days, rounded up.
    expect(screen.getByTestId("rank-season-days").textContent).toContain("35");
  });

  it("renders the season identifier verbatim, untranslated", async () => {
    await i18n.changeLanguage("mk");
    render(<RankBanner season={baseSeason} />);
    expect(screen.getByTestId("rank-season-days").getAttribute("title")).toBe("2026 Q3");
  });

  it("renders a player at zero SP as Iron rather than unranked", () => {
    render(
      <RankBanner
        season={{ ...baseSeason, sp: 0, rankTier: "iron", spIntoTier: 0, spForNextTier: 500 }}
      />,
    );
    expect(screen.getByTestId("rank-banner").getAttribute("data-tier")).toBe("iron");
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Iron");
    expect(screen.getByTestId("rank-progress").getAttribute("aria-valuenow")).toBe("0");
  });

  it("renders a full bar at Grandmaster, where there is no next tier", () => {
    render(
      <RankBanner
        season={{
          ...baseSeason,
          sp: 20000,
          rankTier: "grandmaster",
          spIntoTier: 2000,
          spForNextTier: 0,
        }}
      />,
    );
    // The terminal case: spForNextTier 0 must read as complete, not empty.
    expect(screen.getByTestId("rank-progress").getAttribute("aria-valuenow")).toBe("100");
    expect(screen.getByTestId("rank-progress-caption").textContent).toBe(
      i18n.t("season.banner.atTop"),
    );
  });

  it("falls back to the SP bucket for an unrecognised tier token", () => {
    // Version skew: a newer server sends a tier this bundle has never heard of.
    render(<RankBanner season={{ ...baseSeason, rankTier: "mythic" }} />);
    expect(screen.getByTestId("rank-banner").getAttribute("data-tier")).toBe("gold");
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Gold");
  });

  it("renders zero days remaining for a window that has already closed", () => {
    render(<RankBanner season={{ ...baseSeason, endsAt: "2026-01-01T00:00:00Z" }} />);
    expect(screen.getByTestId("rank-season-days").textContent).toContain("0");
  });

  it("renders the localized tier name in mk", async () => {
    await i18n.changeLanguage("mk");
    render(<RankBanner season={baseSeason} />);
    expect(screen.getByTestId("rank-tier-name").textContent).toBe(i18n.t("season.tier.gold"));
  });
});
