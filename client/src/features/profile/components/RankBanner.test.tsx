import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RankBanner } from "@/features/profile/components/RankBanner";
import { i18n } from "@/shared/i18n/i18n";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";

// A fixed "now" so the days-remaining figure is deterministic.
const NOW = Date.parse("2026-08-27T12:00:00Z");

const baseSeason: CurrentSeasonResponse = {
  seasonName: "2026 Q3",
  endsAt: "2026-10-01T00:00:00Z",
  sp: 680,
  rankTier: "gold",
  // 680 SP is Gold 2 (667-733): 13 SP into its 67-SP step toward Gold 3.
  rankDivision: 2,
  spIntoDivision: 13,
  spForNextDivision: 67,
  gamesPlayed: 31,
  gamesCompleted: 29,
};

// NO PROVIDER. The banner needed a QueryClient while it owned the season
// boundary effect; that moved to useSeasonWindowWatch (mounted on the header's
// rank chip) when the banner moved to the profile, and the banner is once again
// a pure function of its one prop. A provider here would hide a regression that
// re-added a query to it.
function renderBanner(season: CurrentSeasonResponse | undefined) {
  return render(<RankBanner season={season} />);
}

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
    renderBanner(undefined);
    expect(screen.queryByTestId("rank-banner")).toBeNull();
  });

  it("renders the tier badge", () => {
    renderBanner(baseSeason);
    expect(screen.getByTestId("rank-badge")).toBeTruthy();
  });

  // Story 13.2 lifted this badge into the shared TierBadge, which the leaderboard
  // rows render at `sm`. Nothing then pinned the BANNER's geometry, so a retune
  // of the shared `md` scale would silently resize the AC3 banner badge — the one
  // element the AC names by size — with every test still green.
  it("renders its badge at the md scale, with the tier token attached", () => {
    renderBanner(baseSeason);
    const badge = screen.getByTestId("rank-badge");
    expect(badge.className).toContain("size-11");
    expect(badge.getAttribute("data-tier")).toBe("gold");
    // Decorative: the tier NAME is rendered as text beside it.
    expect(badge.getAttribute("aria-hidden")).toBe("true");
    expect(badge.querySelector("svg")?.getAttribute("class")).toContain("size-5");
  });

  it("marks the banner with the tier token so the colour can be asserted", () => {
    renderBanner(baseSeason);
    expect(screen.getByTestId("rank-banner").getAttribute("data-tier")).toBe("gold");
  });

  it("renders the rank as tier plus division", () => {
    renderBanner(baseSeason);
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Gold 2");
  });

  it("renders the current SP total", () => {
    renderBanner(baseSeason);
    // Grouped via toLocaleString (the same call HeroBlock's stat pills use), so
    // the expectation is built the same way rather than hardcoding a separator
    // the CI host's locale may not use.
    expect(screen.getByTestId("rank-sp").textContent).toContain((680).toLocaleString());
  });

  it("fills the bar toward the next rank step, from the server's own decomposition", () => {
    renderBanner(baseSeason);
    const bar = screen.getByTestId("rank-progress");
    expect(bar.getAttribute("role")).toBe("progressbar");
    expect(bar.getAttribute("aria-valuemin")).toBe("0");
    expect(bar.getAttribute("aria-valuemax")).toBe("100");
    // 13 of Gold 2's 67-SP step, NOT 80 of the 200-wide Gold band.
    expect(bar.getAttribute("aria-valuenow")).toBe("19");
    expect(bar.getAttribute("aria-label")).toBe("Gold 2, 13 of 67 SP to the next rank");
    expect(screen.getByTestId("rank-progress-caption").textContent).toBe("13 / 67 SP");
  });

  it("renders the days remaining in the season", () => {
    renderBanner(baseSeason);
    // 2026-08-27T12:00Z -> 2026-10-01T00:00Z is 34.5 days, rounded up.
    expect(screen.getByTestId("rank-season-days").textContent).toContain("35");
  });

  it("renders the season identifier verbatim, untranslated", async () => {
    await i18n.changeLanguage("mk");
    renderBanner(baseSeason);
    expect(screen.getByTestId("rank-season-days").getAttribute("title")).toBe("2026 Q3");
  });

  it("renders a player at zero SP as Iron 1 rather than unranked", () => {
    renderBanner({
      ...baseSeason,
      sp: 0,
      rankTier: "iron",
      rankDivision: 1,
      spIntoDivision: 0,
      spForNextDivision: 50,
    });
    expect(screen.getByTestId("rank-banner").getAttribute("data-tier")).toBe("iron");
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Iron 1");
    expect(screen.getByTestId("rank-progress").getAttribute("aria-valuenow")).toBe("0");
  });

  it("renders Master without a division, filling toward Grandmaster", () => {
    renderBanner({
      ...baseSeason,
      sp: 1250,
      rankTier: "master",
      rankDivision: null,
      spIntoDivision: 50,
      spForNextDivision: 200,
    });
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Master");
    expect(screen.getByTestId("rank-progress").getAttribute("aria-valuenow")).toBe("25");
  });

  it("renders a full bar at Grandmaster, where there is no next rank", () => {
    renderBanner({
      ...baseSeason,
      sp: 1500,
      rankTier: "grandmaster",
      rankDivision: null,
      spIntoDivision: 100,
      spForNextDivision: 0,
    });
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Grandmaster");
    // The terminal case: a 0 step must read as complete, not empty.
    expect(screen.getByTestId("rank-progress").getAttribute("aria-valuenow")).toBe("100");
    expect(screen.getByTestId("rank-progress-caption").textContent).toBe(
      i18n.t("season.banner.atTop"),
    );
  });

  it("drops a division the tier cannot have rather than rendering it", () => {
    // Version skew the other way: a division on a single tier.
    renderBanner({ ...baseSeason, sp: 1250, rankTier: "master", rankDivision: 2 });
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Master");
  });

  it("falls back to the SP bucket for an unrecognised tier token", () => {
    // Version skew: a newer server sends a tier this bundle has never heard of.
    // 700 SP is Gold.
    renderBanner({ ...baseSeason, rankTier: "mythic", sp: 700 });
    expect(screen.getByTestId("rank-banner").getAttribute("data-tier")).toBe("gold");
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Gold 2");
  });

  it("renders zero days remaining for a window that has already closed", () => {
    renderBanner({ ...baseSeason, endsAt: "2026-01-01T00:00:00Z" });
    expect(screen.getByTestId("rank-season-days").textContent).toContain("0");
  });

  it("renders the localized rank in mk", async () => {
    await i18n.changeLanguage("mk");
    renderBanner(baseSeason);
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Злато 2");
  });
});
