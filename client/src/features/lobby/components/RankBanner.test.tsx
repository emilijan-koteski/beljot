import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen } from "@testing-library/react";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RankBanner } from "@/features/lobby/components/RankBanner";
import { i18n } from "@/shared/i18n/i18n";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";

// The transition toast rides sonner, mocked so a test can count firings.
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() },
}));

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

// The banner consumes a QueryClient since Story 13.3 (the boundary effect
// invalidates the season queries), so every render needs a provider.
function renderBanner(
  season: CurrentSeasonResponse | undefined,
  qc: QueryClient = new QueryClient(),
) {
  return render(
    <QueryClientProvider client={qc}>
      <RankBanner season={season} />
    </QueryClientProvider>,
  );
}

describe("RankBanner", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    vi.mocked(toast.success).mockClear();
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

  it("renders the tier name", () => {
    renderBanner(baseSeason);
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Gold");
  });

  it("renders the current SP total", () => {
    renderBanner(baseSeason);
    // Grouped via toLocaleString (the same call HeroBlock's stat pills use), so
    // the expectation is built the same way rather than hardcoding a separator
    // the CI host's locale may not use.
    expect(screen.getByTestId("rank-sp").textContent).toContain((4000).toLocaleString());
  });

  it("renders the progress bar with the server's own decomposition", () => {
    renderBanner(baseSeason);
    const bar = screen.getByTestId("rank-progress");
    expect(bar.getAttribute("role")).toBe("progressbar");
    expect(bar.getAttribute("aria-valuemin")).toBe("0");
    expect(bar.getAttribute("aria-valuemax")).toBe("100");
    // 1000 of a 2500-wide band.
    expect(bar.getAttribute("aria-valuenow")).toBe("40");
    expect(bar.getAttribute("aria-label")).toBeTruthy();
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

  it("renders a player at zero SP as Iron rather than unranked", () => {
    renderBanner({ ...baseSeason, sp: 0, rankTier: "iron", spIntoTier: 0, spForNextTier: 500 });
    expect(screen.getByTestId("rank-banner").getAttribute("data-tier")).toBe("iron");
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Iron");
    expect(screen.getByTestId("rank-progress").getAttribute("aria-valuenow")).toBe("0");
  });

  it("renders a full bar at Grandmaster, where there is no next tier", () => {
    renderBanner({
      ...baseSeason,
      sp: 20000,
      rankTier: "grandmaster",
      spIntoTier: 2000,
      spForNextTier: 0,
    });
    // The terminal case: spForNextTier 0 must read as complete, not empty.
    expect(screen.getByTestId("rank-progress").getAttribute("aria-valuenow")).toBe("100");
    expect(screen.getByTestId("rank-progress-caption").textContent).toBe(
      i18n.t("season.banner.atTop"),
    );
  });

  it("falls back to the SP bucket for an unrecognised tier token", () => {
    // Version skew: a newer server sends a tier this bundle has never heard of.
    renderBanner({ ...baseSeason, rankTier: "mythic" });
    expect(screen.getByTestId("rank-banner").getAttribute("data-tier")).toBe("gold");
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Gold");
  });

  it("renders zero days remaining for a window that has already closed", () => {
    renderBanner({ ...baseSeason, endsAt: "2026-01-01T00:00:00Z" });
    expect(screen.getByTestId("rank-season-days").textContent).toContain("0");
  });

  it("renders the localized tier name in mk", async () => {
    await i18n.changeLanguage("mk");
    renderBanner(baseSeason);
    expect(screen.getByTestId("rank-tier-name").textContent).toBe(i18n.t("season.tier.gold"));
  });

  // --- Story 13.3: the boundary effect + transition toast ---

  it("invalidates season.current, every leaderboard key and the seasons list when the window has ended", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    // endsAt an hour before the fixed NOW: the boundary has already passed.
    renderBanner({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);

    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "current"] });
    // The PREFIX, so the widget's top ten, the page's every size and every
    // season selector — including the infinite entries — all go together.
    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "leaderboard"] });
    // The picker's feed: it gains a window at exactly this moment, and it is
    // deliberately unpolled, so nothing else would ever refresh it.
    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "list"] });
    expect(spy).toHaveBeenCalledTimes(3);
  });

  it("does not invalidate while the window is still open", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    renderBanner(baseSeason, qc); // ends 2026-10-01, after NOW

    expect(spy).not.toHaveBeenCalled();
  });

  it("invalidates once per boundary, not on every 30s tick", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    renderBanner({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);
    expect(spy).toHaveBeenCalledTimes(3);

    // Three more shared ticks over the SAME dead window: no further calls — a
    // repeat would refetch-loop the lobby forever while the server (correctly)
    // keeps answering with whatever window covers now.
    act(() => {
      vi.advanceTimersByTime(90_000);
    });
    expect(spy).toHaveBeenCalledTimes(3);
  });

  it("fires the invalidation when the countdown crosses zero on a later tick", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    // Ends 15s after NOW: alive on mount, dead by the first 30s tick.
    renderBanner({ ...baseSeason, endsAt: "2026-08-27T12:00:15Z" }, qc);
    expect(spy).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(30_000);
    });
    expect(spy).toHaveBeenCalledTimes(3);
  });

  // THE SECOND ROLLOVER IN ONE MOUNTED SESSION. The guard keys on the endsAt
  // value precisely so the refetched new window re-arms it; a fire-once boolean
  // would pass every other test in this file and then silently revive the
  // dead-season bug for anyone whose lobby outlives two quarters.
  it("re-arms for the next quarter after the refetched season arrives", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    const view = renderBanner({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);
    expect(spy).toHaveBeenCalledTimes(3);

    // What the invalidation's refetch delivers: the NEW window, still open.
    view.rerender(
      <QueryClientProvider client={qc}>
        <RankBanner
          season={{ ...baseSeason, seasonName: "2026 Q4", endsAt: "2026-08-27T12:00:15Z" }}
        />
      </QueryClientProvider>,
    );
    expect(spy).toHaveBeenCalledTimes(3); // still alive, nothing to do

    // ...and that one ends too.
    act(() => {
      vi.advanceTimersByTime(30_000);
    });
    expect(spy).toHaveBeenCalledTimes(6);
  });

  // CLOCK SKEW. A client running ahead of the server fires early, the refetch
  // returns the SAME still-active window, and a permanently consumed guard
  // would leave nothing to fire at the real boundary — season.current is
  // deliberately unpolled. The re-arm turns that dead end into a slow retry.
  it("retries the invalidation when the refetch returns the same window", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    renderBanner({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);
    expect(spy).toHaveBeenCalledTimes(3);

    // Inside the re-arm delay: still quiet, so a skewed clock cannot refetch-loop.
    act(() => {
      vi.advanceTimersByTime(4 * 60_000);
    });
    expect(spy).toHaveBeenCalledTimes(3);

    // Past it: one more attempt at the window the server still calls current.
    act(() => {
      vi.advanceTimersByTime(2 * 60_000);
    });
    expect(spy).toHaveBeenCalledTimes(6);
  });

  it("fires one transition toast when the observed season changes, and none on first load", () => {
    const qc = new QueryClient();
    const view = render(
      <QueryClientProvider client={qc}>
        <RankBanner season={baseSeason} />
      </QueryClientProvider>,
    );
    // Loading a season is not a transition.
    expect(toast.success).not.toHaveBeenCalled();

    const next: CurrentSeasonResponse = {
      ...baseSeason,
      seasonName: "2026 Q4",
      endsAt: "2027-01-01T00:00:00Z",
      sp: 0,
      rankTier: "iron",
      spIntoTier: 0,
      spForNextTier: 500,
    };
    view.rerender(
      <QueryClientProvider client={qc}>
        <RankBanner season={next} />
      </QueryClientProvider>,
    );
    expect(toast.success).toHaveBeenCalledTimes(1);
    expect(vi.mocked(toast.success).mock.calls[0]![0]).toBe(
      i18n.t("season.banner.newSeason", { season: "2026 Q4" }),
    );

    // Re-rendering the SAME new season must not toast again.
    view.rerender(
      <QueryClientProvider client={qc}>
        <RankBanner season={{ ...next, sp: 120 }} />
      </QueryClientProvider>,
    );
    expect(toast.success).toHaveBeenCalledTimes(1);
  });
});
