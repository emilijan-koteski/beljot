import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render } from "@testing-library/react";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useSeasonWindowWatch } from "@/shared/hooks/useSeasonWindowWatch";
import { i18n } from "@/shared/i18n/i18n";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";

// The transition toast rides sonner, mocked so a test can count firings.
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() },
}));

// A fixed "now" so every boundary comparison is deterministic.
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

/**
 * The hook renders NOTHING, which is the point: these effects used to live
 * inside RankBanner, where every assertion about them had to go through that
 * component's markup. Exercised through a bare probe, the behaviour is pinned
 * to the hook itself and survives the next surface that hosts it.
 */
function Probe({ season }: { season: CurrentSeasonResponse | undefined }) {
  useSeasonWindowWatch(season);
  return null;
}

function renderWatch(
  season: CurrentSeasonResponse | undefined,
  qc: QueryClient = new QueryClient(),
) {
  return render(
    <QueryClientProvider client={qc}>
      <Probe season={season} />
    </QueryClientProvider>,
  );
}

describe("useSeasonWindowWatch", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    vi.mocked(toast.success).mockClear();
  });

  afterEach(async () => {
    vi.useRealTimers();
    await i18n.changeLanguage("en");
  });

  it("does nothing at all while the season query is in flight", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");
    renderWatch(undefined, qc);
    expect(spy).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("invalidates season.current, every leaderboard key and the seasons list when the window has ended", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    // endsAt an hour before the fixed NOW: the boundary has already passed.
    renderWatch({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);

    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "current"] });
    // The PREFIX, so the page's every size and every season selector —
    // including the infinite entries — all go together.
    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "leaderboard"] });
    // The picker's feed: it gains a window at exactly this moment, and it is
    // deliberately unpolled, so nothing else would ever refresh it.
    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "list"] });
    expect(spy).toHaveBeenCalledTimes(3);
  });

  it("does not invalidate while the window is still open", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    renderWatch(baseSeason, qc); // ends 2026-10-01, after NOW

    expect(spy).not.toHaveBeenCalled();
  });

  it("invalidates once per boundary, not on every 30s tick", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    renderWatch({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);
    expect(spy).toHaveBeenCalledTimes(3);

    // Three more shared ticks over the SAME dead window: no further calls — a
    // repeat would refetch-loop the app forever while the server (correctly)
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
    renderWatch({ ...baseSeason, endsAt: "2026-08-27T12:00:15Z" }, qc);
    expect(spy).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(30_000);
    });
    expect(spy).toHaveBeenCalledTimes(3);
  });

  // THE SECOND ROLLOVER IN ONE MOUNTED SESSION. The guard keys on the endsAt
  // value precisely so the refetched new window re-arms it; a fire-once boolean
  // would pass every other test in this file and then silently revive the
  // dead-season bug for anyone whose tab outlives two quarters.
  it("re-arms for the next quarter after the refetched season arrives", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    const view = renderWatch({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);
    expect(spy).toHaveBeenCalledTimes(3);

    // What the invalidation's refetch delivers: the NEW window, still open.
    view.rerender(
      <QueryClientProvider client={qc}>
        <Probe season={{ ...baseSeason, seasonName: "2026 Q4", endsAt: "2026-08-27T12:00:15Z" }} />
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

    renderWatch({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);
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
    const view = renderWatch(baseSeason, qc);
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
        <Probe season={next} />
      </QueryClientProvider>,
    );
    expect(toast.success).toHaveBeenCalledTimes(1);
    expect(vi.mocked(toast.success).mock.calls[0]![0]).toBe(
      i18n.t("season.banner.newSeason", { season: "2026 Q4" }),
    );

    // Re-rendering the SAME new season must not toast again.
    view.rerender(
      <QueryClientProvider client={qc}>
        <Probe season={{ ...next, sp: 120 }} />
      </QueryClientProvider>,
    );
    expect(toast.success).toHaveBeenCalledTimes(1);
  });
});
